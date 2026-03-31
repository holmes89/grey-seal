package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"connectrpc.com/connect"
	connectcors "connectrpc.com/cors"
	"github.com/rs/cors"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/zap"
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
	ctx := context.Background()
	logger, _ := zap.NewProduction()
	defer logger.Sync() //nolint:errcheck

	shutdown, err := initOTel(ctx, "grey-seal", logger)
	if err != nil {
		logger.Warn("failed to initialize OTel", zap.Error(err))
	} else {
		defer shutdown(ctx)
	}

	dbURL := os.Getenv("DATABASE_URL")
	store, err := repo.NewDatabase(dbURL)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
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
	roleSvc := rolesvc.NewRoleService(roleRepo, logger)
	rolePath, roleHandler := servicesconnect.NewRoleServiceHandler(rolegrpc.NewRoleHandler(roleSvc))
	logger.Info("registering role service route", zap.String("path", rolePath))
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
		logger,
	)
	convPath, convHandler := servicesconnect.NewConversationServiceHandler(conversationgrpc.NewConversationHandler(convSvc))
	logger.Info("registering conversation service route", zap.String("path", convPath))
	mux.Handle(convPath, withCORS(convHandler))

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok") //nolint:errcheck
	})

	errs := make(chan error, 2)
	go func() {
		logger.Info("listening on :9000")
		errs <- http.ListenAndServe(":9000", h2c.NewHandler(otelhttp.NewHandler(mux, "grey-seal"), &http2.Server{}))
	}()
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errs <- fmt.Errorf("signal: %s", <-c)
	}()

	logger.Info("terminated", zap.String("reason", (<-errs).Error()))
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

func (s *shrikeSearcher) Search(ctx context.Context, query string, limit int32, resourceUUIDs []string) ([]conversationsvc.SearchResult, error) {
	resp, err := s.client.Search(ctx, connect.NewRequest(&shrikev1.SearchRequest{
		Query: query,
		Limit: limit,
		Mode:  "hybrid",
	}))
	if err != nil {
		return nil, err
	}

	// Build lookup set for fast filtering.
	var uuidSet map[string]bool
	if len(resourceUUIDs) > 0 {
		uuidSet = make(map[string]bool, len(resourceUUIDs))
		for _, id := range resourceUUIDs {
			uuidSet[id] = true
		}
	}

	results := make([]conversationsvc.SearchResult, 0, len(resp.Msg.GetResults()))
	for _, r := range resp.Msg.GetResults() {
		// If the conversation scopes to specific resources, skip unrelated results.
		if uuidSet != nil && !uuidSet[r.GetEntityUuid()] {
			continue
		}
		results = append(results, conversationsvc.SearchResult{
			EntityUUID: r.GetEntityUuid(),
			Title:      r.GetTitle(),
			Snippet:    r.GetSnippet(),
			Score:      r.GetScore(),
		})
	}
	return results, nil
}
