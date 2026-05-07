package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/holmes89/archaea/kafka"
	"github.com/holmes89/archaea/worker"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	"github.com/holmes89/grey-seal/lib/greyseal/resource"
	"github.com/holmes89/grey-seal/lib/repo"
	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	shrikev1 "github.com/holmes89/shrike/lib/schemas/shrike/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger, _ := zap.NewProduction()
	defer logger.Sync() //nolint:errcheck

	shutdown, err := initOTel(ctx, "grey-seal-worker", logger)
	if err != nil {
		logger.Warn("failed to initialize OTel", zap.Error(err))
	} else {
		defer shutdown(ctx)
	}

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
	defer func() { _ = textProducer.Close() }()

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

	go processResources(resourceConsumer, textProducer, resourceRepo)

	log.Println("worker started, consuming resources...")
	worker.Run(ctx, cancel, resourceConsumer)
}

// processResources reads Resource protos from Kafka, fetches their content,
// publishes TextExtractedEvents to shrike, and marks resources as indexed.
func processResources(
	consumer *kafka.Consumer[*greysealv1.Resource],
	producer *kafka.Producer[*shrikev1.TextExtractedEvent],
	resourceRepo *repo.ResourceRepo,
) {
	meter := otel.Meter("grey-seal/resource-consumer")
	msgDuration, _ := meter.Float64Histogram("kafka.consumer.message.duration",
		metric.WithDescription("Duration of Kafka message processing in seconds"),
		metric.WithUnit("s"),
	)
	tracer := otel.Tracer("grey-seal/resource-consumer")

	for r := range consumer.Read() {
		start := time.Now()
		ctx, span := tracer.Start(context.Background(), "resource.consume")
		result := "ok"

		content, err := resource.FetchContent(ctx, r)
		if err != nil {
			log.Printf("worker: failed to fetch content for resource %s: %v", r.Uuid, err)
			result = "error"
			msgDuration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attribute.String("result", result)))
			span.End()
			continue
		}
		if content == "" {
			log.Printf("worker: empty content for resource %s, skipping", r.Uuid)
			msgDuration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attribute.String("result", result)))
			span.End()
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
			result = "error"
			msgDuration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attribute.String("result", result)))
			span.End()
			continue
		}

		r.IndexedAt = timestamppb.New(time.Now())
		if err := resourceRepo.Update(ctx, r.Uuid, r); err != nil {
			log.Printf("worker: failed to update indexed_at for resource %s: %v", r.Uuid, err)
		}

		log.Printf("worker: resource %s queued for shrike indexing (%d chars)", r.Uuid, len(content))
		msgDuration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attribute.String("result", result)))
		span.End()
	}
}
