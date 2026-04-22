package main

import (
	"context"
	"net/http"
	"os"

	"connectrpc.com/connect"
	"github.com/holmes89/archaea/server"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/zap"

	conversationsvc "github.com/holmes89/grey-seal/lib/greyseal/conversation"
	conversationgrpc "github.com/holmes89/grey-seal/lib/greyseal/conversation/grpc"
	resourcesvc "github.com/holmes89/grey-seal/lib/greyseal/resource"
	resourcegrpc "github.com/holmes89/grey-seal/lib/greyseal/resource/grpc"
	rolesvc "github.com/holmes89/grey-seal/lib/greyseal/role"
	rolegrpc "github.com/holmes89/grey-seal/lib/greyseal/role/grpc"
	"github.com/holmes89/grey-seal/lib/repo"
	"github.com/holmes89/grey-seal/lib/repo/cache"
	"github.com/holmes89/grey-seal/lib/repo/ollama"
	"github.com/holmes89/grey-seal/lib/repo/transcript"
	"github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services/servicesconnect"
	shrikev1 "github.com/holmes89/shrike/lib/schemas/shrike/v1/services"
	shrikeconnect "github.com/holmes89/shrike/lib/schemas/shrike/v1/services/servicesv1connect"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	srv := server.New(":9000",
		func(h http.Handler) http.Handler { return otelhttp.NewHandler(h, "grey-seal") },
	)

	// Role service
	roleRepo := &repo.RoleRepo{Conn: store}
	roleSvc := rolesvc.NewRoleService(roleRepo, logger)
	rolePath, roleHandler := servicesconnect.NewRoleServiceHandler(rolegrpc.NewRoleHandler(roleSvc))
	logger.Info("registering role service route", zap.String("path", rolePath))
	srv.Handle(rolePath, roleHandler)

	// Resource service (Kafka indexer is wired only when KAFKA_BROKERS is set)
	var indexer resourcesvc.Indexer
	if brokers := os.Getenv("KAFKA_BROKERS"); brokers != "" {
		indexer = resourcesvc.NewKafkaIndexer(brokers, logger)
	}
	resourceRepo := &repo.ResourceRepo{Conn: store}
	resSvc := resourcesvc.NewResourceService(resourceRepo, indexer, logger)
	resourcePath, resourceHandler := servicesconnect.NewResourceServiceHandler(resourcegrpc.NewResourceHandler(resSvc))
	logger.Info("registering resource service route", zap.String("path", resourcePath))
	srv.Handle(resourcePath, resourceHandler)

	// Per-conversation resource cache (optional; requires REDIS_URL)
	var resourceCache conversationsvc.ResourceCache
	if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
		rdb := redis.NewClient(&redis.Options{Addr: redisURL})
		resourceCache = cache.NewRedisResourceCache(rdb)
	}

	// Conversation service
	convRepo := repo.NewConversationRepo(store)
	messageRepo := &repo.MessageRepo{Conn: store}
	var transcriptWriter conversationsvc.TranscriptWriter
	if dir := os.Getenv("TRANSCRIPT_DIR"); dir != "" {
		tw, err := transcript.NewWriter(dir)
		if err != nil {
			logger.Warn("failed to create transcript writer", zap.Error(err))
		} else {
			transcriptWriter = tw
			logger.Info("transcript writer enabled", zap.String("dir", dir))
		}
	}

	convSvc := conversationsvc.NewConversationService(
		convRepo,
		messageRepo,
		searcher,
		roleRepo,
		ollamaLLM,
		resourceCache,
		logger,
		transcriptWriter,
	)
	convPath, convHandler := servicesconnect.NewConversationServiceHandler(conversationgrpc.NewConversationHandler(convSvc))
	logger.Info("registering conversation service route", zap.String("path", convPath))
	srv.Handle(convPath, convHandler)

	srv.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok")) //nolint:errcheck
	})

	if err := srv.Run(ctx); err != nil {
		logger.Info("terminated", zap.String("reason", err.Error()))
	}
}

// shrikeSearcher adapts the shrike SearchServiceClient to conversation.Searcher.
type shrikeSearcher struct {
	client shrikeconnect.SearchServiceClient
}

func (s *shrikeSearcher) Search(ctx context.Context, query string, limit int32, resourceUUIDs []string) ([]conversationsvc.SearchResult, error) {
	req := &shrikev1.SearchRequest{
		Query: query,
		Limit: limit,
		Mode:  "hybrid",
	}
	if len(resourceUUIDs) > 0 {
		req.Filter = &shrikev1.SearchFilter{EntityUuids: resourceUUIDs}
	}
	resp, err := s.client.Search(ctx, connect.NewRequest(req))
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
