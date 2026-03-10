package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	connectcors "connectrpc.com/cors"
	"github.com/rs/cors"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	conversationsvc "github.com/holmes89/grey-seal/lib/greyseal/conversation"
	conversationgrpc "github.com/holmes89/grey-seal/lib/greyseal/conversation/grpc"
	resourcesvc "github.com/holmes89/grey-seal/lib/greyseal/resource"
	resourcegrpc "github.com/holmes89/grey-seal/lib/greyseal/resource/grpc"
	rolesvc "github.com/holmes89/grey-seal/lib/greyseal/role"
	rolegrpc "github.com/holmes89/grey-seal/lib/greyseal/role/grpc"
	"github.com/holmes89/grey-seal/lib/repo"
	"github.com/holmes89/grey-seal/lib/repo/ollama"
	"github.com/holmes89/grey-seal/lib/repo/qdrant"
	"github.com/holmes89/grey-seal/lib/repo/scraper"
	"github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services/servicesconnect"
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

	ollamaEmbedder := ollama.NewEmbedder()
	ollamaLLM := ollama.NewLLM()
	webScraper := scraper.NewScraper()

	mux := http.NewServeMux()

	// Role service
	roleRepo := &repo.RoleRepo{Conn: store}
	roleSvc := rolesvc.NewRoleService(roleRepo)
	rolePath, roleHandler := servicesconnect.NewRoleServiceHandler(rolegrpc.NewRoleHandler(roleSvc))
	mux.Handle(rolePath, withCORS(roleHandler))

	// Resource service
	resourceSvc := resourcesvc.NewResourceService(
		&repo.ResourceRepo{Conn: store},
		vectorRepo,
		ollamaEmbedder,
		webScraper,
	)
	resourcePath, resourceHandler := servicesconnect.NewResourceServiceHandler(resourcegrpc.NewResourceHandler(resourceSvc))
	mux.Handle(resourcePath, withCORS(resourceHandler))

	// Conversation service
	convRepo := repo.NewConversationRepo(store)
	messageRepo := &repo.MessageRepo{Conn: store}
	qdrantQuerier := &qdrantVectorQuerier{repo: vectorRepo}
	convSvc := conversationsvc.NewConversationService(
		convRepo,
		messageRepo,
		qdrantQuerier,
		ollamaEmbedder,
		roleRepo,
		ollamaLLM,
	)
	convPath, convHandler := servicesconnect.NewConversationServiceHandler(conversationgrpc.NewConversationHandler(convSvc))
	mux.Handle(convPath, withCORS(convHandler))

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})

	errs := make(chan error, 2)
	go func() {
		log.Println("listening on :9000")
		errs <- http.ListenAndServe(":9000", h2c.NewHandler(mux, &http2.Server{}))
	}()
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errs <- fmt.Errorf("signal: %s", <-c)
	}()

	log.Printf("terminated: %s\n", <-errs)
}

func withCORS(h http.Handler) http.Handler {
	return cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: connectcors.AllowedMethods(),
		AllowedHeaders: connectcors.AllowedHeaders(),
		ExposedHeaders: connectcors.ExposedHeaders(),
	}).Handler(h)
}

// qdrantVectorQuerier adapts *qdrant.ResourceVectorRepo to conversation.VectorQuerier.
type qdrantVectorQuerier struct {
	repo *qdrant.ResourceVectorRepo
}

func (q *qdrantVectorQuerier) Query(ctx context.Context, queryVector []float32, limit uint64, resourceUUIDs []string) ([]conversationsvc.QueryResult, error) {
	results, err := q.repo.Query(ctx, queryVector, limit, resourceUUIDs)
	if err != nil {
		return nil, err
	}
	out := make([]conversationsvc.QueryResult, 0, len(results))
	for _, r := range results {
		out = append(out, conversationsvc.QueryResult{
			ResourceUUID: r.ResourceUUID,
			Content:      r.Content,
			Score:        r.Score,
		})
	}
	return out, nil
}
