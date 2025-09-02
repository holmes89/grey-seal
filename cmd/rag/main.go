package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/holmes89/grey-seal/lib/docproc"
	"github.com/holmes89/grey-seal/lib/embedding"
	"github.com/holmes89/grey-seal/lib/handlers/rest"
	"github.com/holmes89/grey-seal/lib/rag"
	"github.com/holmes89/grey-seal/lib/repo/vectordb"
)

func main() {
	vdb, err := vectordb.NewVectorDB("./grey-seal.duckdb")
	if err != nil {
		log.Fatal("Failed to initialize vector database:", err)
	}
	defer vdb.Close()
	embeddings := embedding.NewOllamaEmbeddingServiceFromEnvironment("nomic-embed-text")
	docProcessor := docproc.NewDocumentProcessor(vdb, embeddings)
	ragService := rag.NewRAGService(vdb, embeddings)
	handler := rest.NewRestHandler(ragService, docProcessor)
	router := handler.SetupRoutes()

	log.Println("Starting RAG server on :8080")
	log.Println("Endpoints:")
	log.Println("  POST /ingest - Process documents from directory")
	log.Println("  POST /query - RAG query with context")
	log.Println("  POST /search - Semantic search only")
	log.Println("  GET /health - Health check")

	errs := make(chan error, 2)
	go func() {
		log.Println("Listening server mode on port :3000")
		errs <- router.Run(":8080")
	}()
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT)
		errs <- fmt.Errorf("%s", <-c)
	}()
	log.Println("terminated %w", <-errs)
}
