package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/holmes89/grey-seal/lib/repo/vector/scraper"

	connectcors "connectrpc.com/cors"
	"github.com/rs/cors"
	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/holmes89/archaea/kafka"
	"github.com/holmes89/grey-seal/lib/greyseal/question"
	questiongrpc "github.com/holmes89/grey-seal/lib/greyseal/question/grpc"
	"github.com/holmes89/grey-seal/lib/greyseal/resource"
	resourcegrpc "github.com/holmes89/grey-seal/lib/greyseal/resource/grpc"
	"github.com/holmes89/grey-seal/lib/repo/vector"

	"github.com/holmes89/grey-seal/lib/repo"
	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services/servicesv1connect"
)

// Add request logging middleware
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Request: %s %s", r.Method, r.URL.Path)
		log.Printf("Headers: %v", r.Header)
		next.ServeHTTP(w, r)
	})
}

func main() {

	mux := http.NewServeMux()
	conn := os.Getenv("DATABASE_URL")
	store, err := repo.NewDatabase(conn)
	if err != nil {
		fmt.Println("failed to connect to db:", err)
		panic(err)
	}
	defer store.Close()
	fmt.Println("created store...")

	kconn := os.Getenv("KAFKA_BROKERS")
	fmt.Println("KAFKA_BROKERS:", kconn)
	kclient := kafka.NewConn([]string{kconn})
	defer kclient.Close()

	fmt.Println("creating embedding service...")
	ollamaLLMEmbedder, err := ollama.New(ollama.WithModel("all-minilm"), ollama.WithServerURL(
		"http://host.docker.internal:11434",
	))
	if err != nil {
		log.Fatal(err)
	}
	ollamaEmbeder, err := embeddings.NewEmbedder(ollamaLLMEmbedder)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("creating LLM service...")
	ollamaLLM, err := ollama.New(ollama.WithModel("deepseek-r1"), ollama.WithServerURL(
		"http://host.docker.internal:11434",
	))
	if err != nil {
		log.Fatal(err)
	}

	var resourceVectorDB question.Querier = vector.NewResourceVectorRepo(
		&repo.ResourceRepo{Conn: store},
		scraper.NewScraper(),
		store,
		ollamaEmbeder)
	if err != nil {
		log.Fatal(err)
	}

	regQuestion(mux, store, resourceVectorDB, ollamaLLM, kclient)
	regResource(mux, store, kclient)

	// Add a simple health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Server is running")
	})

	// Add a debug endpoint to list all registered routes
	mux.HandleFunc("/debug/routes", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Registered routes:\n")
		fmt.Fprintf(w, "GET /health\n")
		fmt.Fprintf(w, "GET /debug/routes\n")
		fmt.Fprintf(w, "POST /greyseal.v1.services.QuestionService/ListQuestions\n")
		fmt.Fprintf(w, "POST /greyseal.v1.services.QuestionService/GetQuestion\n")
		fmt.Fprintf(w, "POST /greyseal.v1.services.QuestionService/CreateQuestion\n")
		fmt.Fprintf(w, "POST /greyseal.v1.services.ResourceService/ListResources\n")
		fmt.Fprintf(w, "POST /greyseal.v1.services.ResourceService/GetResource\n")
		fmt.Fprintf(w, "POST /greyseal.v1.services.ResourceService/CreateResource\n")
	})

	errs := make(chan error, 2)
	go func() {
		fmt.Println("listening on :9000...")
		fmt.Println("Available endpoints:")
		fmt.Println("  GET  http://localhost:9000/health")
		fmt.Println("  GET  http://localhost:9000/debug/routes")
		fmt.Println("  POST http://localhost:9000/greyseal.v1.services.QuestionService/ListQuestions")
		fmt.Println("  POST http://localhost:9000/greyseal.v1.services.QuestionService/GetQuestion")
		fmt.Println("  POST http://localhost:9000/greyseal.v1.services.QuestionService/CreateQuestion")
		fmt.Println("  POST http://localhost:9000/greyseal.v1.services.ResourceService/ListResources")
		fmt.Println("  POST http://localhost:9000/greyseal.v1.services.ResourceService/GetResource")
		fmt.Println("  POST http://localhost:9000/greyseal.v1.services.ResourceService/CreateResource")

		// Wrap the entire mux with logging
		handler := loggingMiddleware(mux)

		errs <- http.ListenAndServe(
			":9000",
			// Use h2c so we can serve HTTP/2 without TLS.
			h2c.NewHandler(handler, &http2.Server{}),
		)
	}()
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT)
		errs <- fmt.Errorf("%s", <-c)
	}()

	log.Printf("terminated %s\n", <-errs)
}

// withCORS adds CORS support to a Connect HTTP handler.
func withCORS(h http.Handler) http.Handler {
	middleware := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: connectcors.AllowedMethods(),
		AllowedHeaders: connectcors.AllowedHeaders(),
		ExposedHeaders: connectcors.ExposedHeaders(),
	})
	return middleware.Handler(h)
}

func regQuestion(mux *http.ServeMux, store *repo.Conn, resourceVectorDB question.Querier, ollamaLLM llms.Model, kclient *kafka.Conn) {

	questionServer := questiongrpc.NewQuestionService(
		question.NewQuestionService(
			&repo.QuestionRepo{Conn: store}, resourceVectorDB, ollamaLLM,
		),
		kafka.NewProducer[*greysealv1.Question](kclient),
	)
	path, handler := servicesv1connect.NewQuestionServiceHandler(questionServer)
	fmt.Println("registering question service route:", path)
	mux.Handle(path, withCORS(handler))
}

func regResource(mux *http.ServeMux, conn *repo.Conn, kclient *kafka.Conn) {

	resourceServer := resourcegrpc.NewResourceService(
		resource.NewResourceService(
			&repo.ResourceRepo{Conn: conn},
		),
		kafka.NewProducer[*greysealv1.Resource](kclient),
	)
	path, handler := servicesv1connect.NewResourceServiceHandler(resourceServer)
	fmt.Println("registering resource service route:", path)
	mux.Handle(path, withCORS(handler))
}
