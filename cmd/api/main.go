package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"connectrpc.com/connect"
	"github.com/holmes89/archaea/server"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	agentsvc "github.com/holmes89/grey-seal/lib/greyseal/agent"
	agentgrpc "github.com/holmes89/grey-seal/lib/greyseal/agent/grpc"
	conversationsvc "github.com/holmes89/grey-seal/lib/greyseal/conversation"
	conversationgrpc "github.com/holmes89/grey-seal/lib/greyseal/conversation/grpc"
	draftsvc "github.com/holmes89/grey-seal/lib/greyseal/draft"
	draftgrpc "github.com/holmes89/grey-seal/lib/greyseal/draft/grpc"
	resourcesvc "github.com/holmes89/grey-seal/lib/greyseal/resource"
	resourcegrpc "github.com/holmes89/grey-seal/lib/greyseal/resource/grpc"
	rolesvc "github.com/holmes89/grey-seal/lib/greyseal/role"
	rolegrpc "github.com/holmes89/grey-seal/lib/greyseal/role/grpc"
	"github.com/holmes89/grey-seal/lib/repo"
	"github.com/holmes89/grey-seal/lib/repo/aiderrunner"
	"github.com/holmes89/grey-seal/lib/repo/cache"
	"github.com/holmes89/grey-seal/lib/repo/github"
	"github.com/holmes89/grey-seal/lib/repo/mcpclient"
	"github.com/holmes89/grey-seal/lib/repo/ollama"
	"github.com/holmes89/grey-seal/lib/repo/ollamarunner"
	"github.com/holmes89/grey-seal/lib/repo/ollamatools"
	"github.com/holmes89/grey-seal/lib/repo/transcript"
	"github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services/servicesconnect"
	shrikev1 "github.com/holmes89/shrike/lib/schemas/shrike/v1/services"
	shrikeconnect "github.com/holmes89/shrike/lib/schemas/shrike/v1/services/servicesv1connect"
)

var (
	commit    = "dev"
	buildTime = "unknown"
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
		func(h http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				sc := trace.SpanFromContext(r.Context()).SpanContext()
				logger.Info("request",
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path),
					zap.String("trace_id", sc.TraceID().String()),
					zap.String("span_id", sc.SpanID().String()),
				)
				h.ServeHTTP(w, r)
			})
		},
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

	// Agent service. Two independent providers, each optional:
	//   - "aider": code-editing runs in disposable Docker containers, gated on
	//     LITELLM_BASE_URL.
	//   - "ollama:<model>": in-process tool-calling runs (e.g. design →
	//     draft tickets) with tools from an MCP server, gated on REMORA_MCP_URL.
	// The route registers when at least one provider is configured.
	var (
		aiderRunner  agentsvc.SessionRunner
		ollamaRunner agentsvc.SessionRunner
	)

	if litellmBaseURL := os.Getenv("LITELLM_BASE_URL"); litellmBaseURL != "" {
		aiderImage := os.Getenv("AIDER_RUNNER_IMAGE")
		if aiderImage == "" {
			aiderImage = "ghcr.io/holmes89/greyseal-aider-runner:latest"
		}
		litellmModel := os.Getenv("LITELLM_MODEL")
		if litellmModel == "" {
			litellmModel = "qwen-coder"
		}
		r, err := aiderrunner.NewSessionRunner(aiderImage, litellmBaseURL, os.Getenv("LITELLM_API_KEY"), litellmModel, os.Getenv("AIDER_RUNNER_NETWORK"), os.Getenv("AIDER_RUNNER_DATA_DIR"), logger)
		if err != nil {
			logger.Warn("failed to create aider session runner — aider provider disabled", zap.Error(err))
		} else {
			aiderRunner = r
		}
	} else {
		logger.Warn("LITELLM_BASE_URL not set — aider agent provider disabled")
	}

	if mcpURL := os.Getenv("REMORA_MCP_URL"); mcpURL != "" {
		agentModel := os.Getenv("OLLAMA_AGENT_CHAT_MODEL")
		if agentModel == "" {
			agentModel = "qwen3:8b"
		}
		ollamaHost := os.Getenv("OLLAMA_HOST")
		toolProvider := mcpclient.New(mcpURL, logger)
		ollamaRunner = ollamarunner.NewSessionRunner(toolProvider, ollamatools.New(ollamaHost), agentModel, logger)
		logger.Info("ollama agent provider enabled", zap.String("mcp_url", mcpURL), zap.String("model", agentModel))
	} else {
		logger.Warn("REMORA_MCP_URL not set — ollama agent provider disabled")
	}

	if aiderRunner != nil || ollamaRunner != nil {
		agentRunRepo := &repo.AgentRunRepo{Conn: store}
		prOpener := github.NewClient()
		agentSvc := agentsvc.NewAgentService(aiderRunner, ollamaRunner, agentRunRepo, prOpener, logger)
		agentPath, agentHandler := servicesconnect.NewAgentServiceHandler(agentgrpc.NewAgentHandler(agentSvc))
		logger.Info("registering agent service route", zap.String("path", agentPath))
		srv.Handle(agentPath, agentHandler)
	} else {
		logger.Warn("no agent provider configured — agent service route disabled")
	}

	// Planning document drafts (discovery docs, designs). Separate model from
	// chat: drafting wants a stronger writer than the small chat default.
	draftModel := os.Getenv("OLLAMA_DRAFT_MODEL")
	if draftModel == "" {
		draftModel = "qwen3:8b"
	}
	draftSvc := draftsvc.NewDraftService(ollama.NewDraftLLM(os.Getenv("OLLAMA_HOST"), draftModel), logger)
	draftPath, draftHandler := servicesconnect.NewDraftServiceHandler(draftgrpc.NewDraftHandler(draftSvc))
	logger.Info("registering draft service route", zap.String("path", draftPath), zap.String("model", draftModel))
	srv.Handle(draftPath, draftHandler)

	srv.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok"}`) //nolint:errcheck
	})

	srv.HandleFunc("/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"service":%q,"commit":%q,"build_time":%q}`, //nolint:errcheck
			"grey-seal", commit, buildTime)
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
