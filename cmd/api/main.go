package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"connectrpc.com/connect"
	connectcors "connectrpc.com/cors"
	"github.com/rs/cors"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	conversationsvc "github.com/holmes89/grey-seal/lib/greyseal/conversation"
	conversationgrpc "github.com/holmes89/grey-seal/lib/greyseal/conversation/grpc"
	rolesvc "github.com/holmes89/grey-seal/lib/greyseal/role"
	rolegrpc "github.com/holmes89/grey-seal/lib/greyseal/role/grpc"
	"github.com/holmes89/grey-seal/lib/repo"
	"github.com/holmes89/grey-seal/lib/repo/ollama"
	"github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services/servicesconnect"
	shrikev1 "github.com/holmes89/shrike/lib/schemas/shrike/v1/services"
	shrikeconnect "github.com/holmes89/shrike/lib/schemas/shrike/v1/services/servicesv1connect"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	store, err := repo.NewDatabase(dbURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer store.Close()

	ollamaLLM := ollama.NewLLM()

	shrikeURL := os.Getenv("SHRIKE_URL")
	if shrikeURL == "" {
		shrikeURL = "http://shrike:9000"
	}
	shrikeClient := shrikeconnect.NewSearchServiceClient(&http.Client{}, shrikeURL)
	searcher := &shrikeSearcher{client: shrikeClient}

	mux := http.NewServeMux()

	// Role service
	roleRepo := &repo.RoleRepo{Conn: store}
	roleSvc := rolesvc.NewRoleService(roleRepo)
	rolePath, roleHandler := servicesconnect.NewRoleServiceHandler(rolegrpc.NewRoleHandler(roleSvc))
	mux.Handle(rolePath, withCORS(roleHandler))

	// Conversation service
	convRepo := repo.NewConversationRepo(store)
	messageRepo := &repo.MessageRepo{Conn: store}
	convSvc := conversationsvc.NewConversationService(
		convRepo,
		messageRepo,
		searcher,
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

// shrikeSearcher adapts the shrike SearchServiceClient to conversation.Searcher.
type shrikeSearcher struct {
	client shrikeconnect.SearchServiceClient
}

func (s *shrikeSearcher) Search(ctx context.Context, query string, limit int32) ([]conversationsvc.SearchResult, error) {
	resp, err := s.client.Search(ctx, connect.NewRequest(&shrikev1.SearchRequest{
		Query: query,
		Limit: limit,
		Mode:  "hybrid",
	}))
	if err != nil {
		return nil, err
	}
	results := make([]conversationsvc.SearchResult, 0, len(resp.Msg.GetResults()))
	for _, r := range resp.Msg.GetResults() {
		results = append(results, conversationsvc.SearchResult{
			EntityUUID: r.GetEntityUuid(),
			Title:      r.GetTitle(),
			Snippet:    r.GetSnippet(),
			Score:      r.GetScore(),
		})
	}
	return results, nil
}
