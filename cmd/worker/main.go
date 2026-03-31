package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/holmes89/archaea/kafka"
	"github.com/holmes89/grey-seal/lib/greyseal/resource"
	"github.com/holmes89/grey-seal/lib/repo"
	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	shrikev1 "github.com/holmes89/shrike/lib/schemas/shrike/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	store, err := repo.NewDatabase(dbURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer store.Close()

	kBrokers := os.Getenv("KAFKA_BROKERS")
	if kBrokers == "" {
		log.Fatal("KAFKA_BROKERS environment variable is required")
	}
	brokers := strings.Split(kBrokers, ",")

	kConn := kafka.NewConn(brokers)
	defer kConn.Close()

	// Producer that forwards fetched content to the shrike TextExtractedConsumer.
	textProducer := kafka.NewProducer[*shrikev1.TextExtractedEvent](kConn)
	defer textProducer.Close()

	// Consumer for Resource protos enqueued by the API's kafkaIndexer for
	// SOURCE_WEBSITE / SOURCE_PDF resources that require async content fetching.
	groupID := "greyseal-resource-fetcher"
	resourceConsumer := kafka.NewConsumer(brokers, &groupID, func(data []byte) (*greysealv1.Resource, error) {
		var r greysealv1.Resource
		if err := proto.Unmarshal(data, &r); err != nil {
			return nil, err
		}
		return &r, nil
	})

	resourceRepo := &repo.ResourceRepo{Conn: store}

	fmt.Println("worker started, consuming resources...")

	errs := make(chan error, 1)
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errs <- fmt.Errorf("signal: %s", <-c)
	}()

	go processResources(resourceConsumer, textProducer, resourceRepo)

	log.Printf("terminated: %s\n", <-errs)
}

// processResources reads Resource protos from Kafka, fetches their content,
// publishes TextExtractedEvents to shrike, and marks resources as indexed.
func processResources(
	consumer *kafka.Consumer[*greysealv1.Resource],
	producer *kafka.Producer[*shrikev1.TextExtractedEvent],
	resourceRepo *repo.ResourceRepo,
) {
	for r := range consumer.Read() {
		ctx := context.Background()

		content, err := resource.FetchContent(ctx, r)
		if err != nil {
			log.Printf("worker: failed to fetch content for resource %s: %v", r.Uuid, err)
			continue
		}
		if content == "" {
			log.Printf("worker: empty content for resource %s, skipping", r.Uuid)
			continue
		}

		event := &shrikev1.TextExtractedEvent{
			EntityUuid: r.Uuid,
			App:        "grey-seal",
			EntityType: "Resource",
			FullText:   content,
			Title:      r.Name,
			SourceUrl:  r.Path,
		}
		if err := producer.Publish(ctx, event, r.Uuid, time.Now()); err != nil {
			log.Printf("worker: failed to publish TextExtractedEvent for resource %s: %v", r.Uuid, err)
			continue
		}

		// Mark the resource as indexed now that the content has been forwarded to shrike.
		r.IndexedAt = timestamppb.New(time.Now())
		if err := resourceRepo.Update(ctx, r.Uuid, r); err != nil {
			log.Printf("worker: failed to update indexed_at for resource %s: %v", r.Uuid, err)
		}

		log.Printf("worker: resource %s queued for shrike indexing (%d chars)", r.Uuid, len(content))
	}
}
