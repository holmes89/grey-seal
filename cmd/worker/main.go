package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/holmes89/archaea/kafka"
	resourcesvc "github.com/holmes89/grey-seal/lib/greyseal/resource"
	"github.com/holmes89/grey-seal/lib/repo"
	"github.com/holmes89/grey-seal/lib/repo/ollama"
	"github.com/holmes89/grey-seal/lib/repo/qdrant"
	"github.com/holmes89/grey-seal/lib/repo/scraper"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	store, err := repo.NewDatabase(dbURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer store.Close()

	vectorRepo, err := qdrant.NewResourceVectorRepo()
	if err != nil {
		log.Fatalf("failed to connect to qdrant: %v", err)
	}

	resourceService := resourcesvc.NewResourceService(
		&repo.ResourceRepo{Conn: store},
		vectorRepo,
		ollama.NewEmbedder(),
		scraper.NewScraper(),
	)

	brokers := strings.Split(os.Getenv("KAFKA_BROKERS"), ",")
	group := "grey-seal-worker"
	consumer := kafka.NewConsumer(brokers, &group, resourcesvc.ConvertProto)
	defer consumer.Close()

	resourcesvc.NewResourceConsumer(consumer, resourceService)

	fmt.Println("worker listening for resources...")

	errs := make(chan error, 1)
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errs <- fmt.Errorf("signal: %s", <-c)
	}()

	log.Printf("terminated: %s\n", <-errs)
}
