# Grey-Seal — Claude Code Instructions

Full conventions are in [`docs/CONVENTIONS.md`](docs/CONVENTIONS.md).

## What This Project Is

A personal AI assistant / chat service: ConnectRPC API → PostgreSQL, Ollama LLM backend, Shrike for RAG vector search, optional Redis resource cache, and a go-app WASM frontend.

## Before You Write Any Code

1. Read the file you're modifying.
2. Read `docs/CONVENTIONS.md` for the full rule set.
3. Run `make vet && make lint` mentally — your output must pass both.

## Domain Layer Pattern (critical)

Every entity follows this exact structure:

```
lib/greyseal/<entity>/
  interface.go   ← public interface + request types
  service.go     ← *entityService implementing the interface
  grpc/service.go ← ConnectRPC handler
  mocks/         ← mockery-generated, never hand-edit
```

- `service.go` must have `var _ ConversationService = (*conversationService)(nil)` at top.
- Constructors return the **interface**, not the concrete struct.
- Optional deps use `With*` builder methods on the **grpc handler**.
- `context.Context` is always the first argument.
- Log entry with `logger.Info`, errors with `logger.Error(zap.Error(err))`.

## Optional Dependencies

Pass `nil` for optional interfaces when env vars are absent:
- `KAFKA_BROKERS` → Kafka indexer (resource service)
- `REDIS_URL` → Redis resource cache (conversation service)
- `SHRIKE_URL` → Shrike searcher (conversation service)

The `shrikeSearcher` adapter in `cmd/api/main.go` bridges `shrikeconnect.SearchServiceClient` to `conversation.Searcher` — keep domain packages free of shrike imports.

## Database

- Use `squirrel` builders — no raw SQL strings inline.
- `repo.NewDatabase(dsn)` handles OTel instrumentation, connection, and migrations.
- OTel DB driver is `github.com/XSAM/otelsql` — do NOT use `go.opentelemetry.io/contrib/instrumentation/database/sql/otelsql` (doesn't exist).

## Testing Rules

- Unit tests: `package conversation_test`, use `testify/suite`, inject `zap.NewNop()`.
- Integration tests: `//go:build integration` tag, use dockertest (`postgres:16-alpine`), never mock the DB.
- Mocks: run `make generate` to regenerate; never hand-edit `mocks/` files.

## Linting

Lint config is `.golangci.yml`. Enabled: `errcheck govet ineffassign staticcheck misspell unused`.
- Schemas (`lib/schemas/`) are excluded.
- Add `//nolint:errcheck` with reason for intentional ignores.

## Schemas

- `lib/schemas/` is generated — **never hand-edit**.
- Import aliases: `greysealv1 "…/schemas/greyseal/v1"`, `servicev1 "…/schemas/greyseal/v1/services"`.

## Observability

- Telemetry is a graceful noop when `OTEL_EXPORTER_OTLP_ENDPOINT` is unset.
- HTTP: wrapped with `otelhttp.NewHandler(mux, "grey-seal")`.
- DB: `otelsql.Register("postgres", ...)` with `semconv.DBSystemPostgreSQL`.

## Dos and Don'ts

| Do | Don't |
|---|---|
| Return interfaces from constructors | Return concrete types |
| Thread `context.Context` through all calls | Use `context.Background()` mid-stack |
| Use `squirrel` for queries | Write raw SQL strings inline |
| Pass `nil` for absent optional deps | Panic on missing optional deps |
| Use dockertest for integration tests | Mock the DB in integration tests |
| Regenerate mocks with `make generate` | Hand-edit `mocks/` files |
