# Grey-Seal — Coding Conventions

## Project Overview

Grey-seal is a personal AI assistant / chat service. It exposes a ConnectRPC API that manages conversation roles, resources (attachments to conversations), and conversations themselves. It uses Ollama as the LLM backend, Shrike for vector-search-based RAG context retrieval, and an optional Redis cache for per-conversation resource state.

**Ecosystem position:** Grey-seal consumes resources published to it by other services (via the Resource service) and optionally queries Shrike for relevant document snippets before sending prompts to Ollama.

## Tech Stack

| Concern | Library / Tool |
|---|---|
| Language | Go 1.26 |
| RPC | `connectrpc.com/connect` over HTTP/2 (h2c) |
| Schemas | Protocol Buffers → `lib/schemas/greyseal/v1/` |
| Database | PostgreSQL 16 via `lib/pq` + `github.com/Masterminds/squirrel` |
| Migrations | `github.com/pressly/goose/v3` (embedded SQL) |
| Kafka | `github.com/holmes89/archaea/kafka` |
| LLM | Ollama via `lib/repo/ollama/` |
| Vector Search | Shrike client (`github.com/holmes89/shrike`) |
| Cache | Redis (optional) via `github.com/redis/go-redis/v9` |
| Frontend | `github.com/maxence-charriere/go-app/v10` (WASM) |
| Logging | `go.uber.org/zap` |
| Observability | OpenTelemetry (`go.opentelemetry.io/otel` v1.42.0) |
| Mocks | `github.com/vektra/mockery/v2` |
| Tests | `github.com/stretchr/testify/suite` + `github.com/ory/dockertest/v3` |
| Lint | `golangci-lint` (errcheck, govet, ineffassign, staticcheck, misspell, unused) |

## Repository Layout

```
cmd/
  api/        # ConnectRPC API server (main.go, telemetry.go)
  worker/     # Kafka consumer runner
lib/
  greyseal/
    conversation/ # Core chat/conversation domain
    resource/     # Resource management (attachments, context docs)
    role/         # System/user role management
  repo/         # PostgreSQL connection + repos
  repo/ollama/  # Ollama LLM adapter
  repo/cache/   # Redis resource cache adapter
  schemas/      # Generated protobuf Go code — never hand-edit
```

## Domain Layer Conventions

### File layout per domain package

Every domain package (`lib/greyseal/<entity>/`) contains:

| File | Purpose |
|---|---|
| `interface.go` | Public service interface + request/response types |
| `service.go` | `*entityService` struct implementing the interface |
| `grpc/service.go` | ConnectRPC handler wrapping the service |
| `mocks/` | mockery-generated mocks |

### Interface assertions

Every service implementation must have a compile-time assertion at the top of `service.go`:

```go
var _ ConversationService = (*conversationService)(nil)
```

### Constructor pattern

```go
func NewConversationService(repo ConversationRepository, ..., logger *zap.Logger) ConversationService {
    return &conversationService{...}
}
```

- Constructors return the **interface**, not the concrete type.
- Optional dependencies use `With*` chaining on the grpc handler.

### Service methods

- Always accept `context.Context` as the first argument.
- Log entry with `logger.Info`, errors with `logger.Error(zap.Error(err))`.

## Shrike Searcher Adapter

`cmd/api/main.go` contains a `shrikeSearcher` adapter that bridges the Shrike `SearchServiceClient` to the `conversation.Searcher` interface:

```go
type shrikeSearcher struct {
    client shrikeconnect.SearchServiceClient
}

func (s *shrikeSearcher) Search(ctx context.Context, query string, limit int32, resourceUUIDs []string) ([]conversationsvc.SearchResult, error)
```

This keeps the domain package free of shrike dependencies.

## Optional Dependencies (env-var gated)

| Env Var | Effect |
|---|---|
| `DATABASE_URL` | Required — PostgreSQL DSN |
| `KAFKA_BROKERS` | Optional — enables resource Kafka indexer |
| `REDIS_URL` | Optional — enables per-conversation Redis resource cache |
| `SHRIKE_URL` | Optional — Shrike search endpoint (default `http://shrike:9000`) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Optional — enables OpenTelemetry export |

Services degrade gracefully when optional dependencies are absent — pass `nil` for optional interfaces.

## Repository Conventions

- All repos accept `context.Context` as the first argument.
- Use `github.com/Masterminds/squirrel` for query building.
- Migrations are embedded SQL files using `//go:embed migrations/*.sql`.
- Do **not** write raw SQL strings inline.

### Database connection

`repo.NewDatabase(dsn)` handles OTel-instrumented driver registration via `github.com/XSAM/otelsql`, connection, and goose migrations on startup.

## Observability

- `cmd/api/telemetry.go` — `initOTel(ctx, "grey-seal", logger)` — noop when `OTEL_EXPORTER_OTLP_ENDPOINT` unset.
- HTTP handler wrapped with `otelhttp.NewHandler(mux, "grey-seal")`.
- DB: `github.com/XSAM/otelsql` (NOT `go.opentelemetry.io/contrib/instrumentation/database/sql/otelsql` — that path does not exist).

## Testing

### Unit tests

- Package: `package conversation_test`
- Use `testify/suite`, inject `zap.NewNop()`, use mockery mocks

### Integration tests

- Build tag: `//go:build integration`
- Spin up `postgres:16-alpine` via dockertest; run migrations; test real repo

### Makefile targets

```
make test             # unit tests with -race
make test-integration # integration tests with -race
make lint             # golangci-lint
make vet              # go vet
make generate         # regenerate mocks via mockery
make build            # build bin/api
```

## Linting

`.golangci.yml`: errcheck, govet, ineffassign, staticcheck, misspell, unused.
- `lib/schemas/` excluded from errcheck, unused, staticcheck.
- `_test.go` excluded from errcheck.
- Use `//nolint:errcheck` with a comment for intentional suppressions.

## Schemas

- `lib/schemas/greyseal/v1/` — generated protobuf — **never hand-edit**.
- Import aliases: `greysealv1 "…/schemas/greyseal/v1"`, `servicev1 "…/schemas/greyseal/v1/services"`.
- ConnectRPC stubs in `lib/schemas/greyseal/v1/services/servicesconnect/`.

## CORS

All Connect handlers wrapped with `withCORS(handler)` using `connectcors.AllowedMethods/Headers/ExposedHeaders()`.

## What NOT To Do

- Do not hand-edit `lib/schemas/` files.
- Do not mock the database in integration tests.
- Do not use `go.opentelemetry.io/contrib/instrumentation/database/sql/otelsql` — it does not exist; use `github.com/XSAM/otelsql`.
- Do not return concrete types from constructors — always return the interface.
