package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/holmes89/grey-seal/lib/repo"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	store, err := repo.NewDatabase(dbURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer store.Close()

	fmt.Println("worker started...")

	errs := make(chan error, 1)
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errs <- fmt.Errorf("signal: %s", <-c)
	}()

	log.Printf("terminated: %s\n", <-errs)
}
