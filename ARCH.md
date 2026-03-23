# Architecture

## Overview

grey-seal is a single-binary Go service (`cmd/api`) that exposes a Connect-RPC API over HTTP/2 (h2c). It uses a layered architecture: a thin gRPC/Connect handler layer delegates to domain service interfaces, which are backed by PostgreSQL repositories. LLM inference is delegated to a local Ollama instance; semantic search is delegated to the external **shrike** service.

```
┌──────────────────────────────────────────────────────┐
│  Clients (CLI, browser UI, other services)           │
│  Connect-RPC over HTTP/2 (grpc-web compatible)       │
└────────────────────┬─────────────────────────────────┘
                     │ :9000
┌────────────────────▼─────────────────────────────────┐
│  cmd/api/main.go  –  http.ServeMux                   │
│  CORS middleware wraps each route                    │
│  /health  (plaintext liveness probe)                 │
├──────────────────────────────────────────────────────┤
│  Connect-RPC Handlers (lib/greyseal/*/grpc/)         │
│  ┌─────────────────┐  ┌──────────────────────────┐   │
│  │  RoleHandler    │  │  ConversationHandler     │   │
│  │  (CRUD)         │  │  (CRUD + Chat stream +   │   │
│  │                 │  │   SubmitFeedback)        │   │
│  └────────┬────────┘  └────────────┬─────────────┘   │
│           │                        │                  │
│  ┌────────▼────────┐  ┌────────────▼─────────────┐   │
│  │  RoleService    │  │  ConversationService     │   │
│  │  (interface)    │  │  (interface)             │   │
│  └────────┬────────┘  └─┬────────┬───────┬───────┘   │
│           │             │        │       │            │
│      RoleRepo     ConvRepo  MessageRepo Searcher LLM  │
└──────┬────┴─────────┬────┴────┬───┴───────┴──────┴───┘
       │              │         │          │        │
   PostgreSQL      PostgreSQL  PostgreSQL  shrike  Ollama
```

## Process Inventory

| Process | Source | Port | Notes |
|---|---|---|---|
| API server | `cmd/api/main.go` | 9000 | Active, ships in `Dockerfile` |
| Worker | `cmd/worker/main.go` | — | Skeleton only; reads `DATABASE_URL`, waits for signal |
| UI | `cmd/ui/main.go` | 8000 | `//go:build ignore`; excluded from normal builds |

## Transport

The API server uses `h2c` (cleartext HTTP/2) via `golang.org/x/net/http2/h2c`, making it compatible with both native gRPC clients and the Connect-RPC `grpc-web` protocol. CORS is applied per-handler using `connectrpc.com/cors` helper headers, allowing wildcard origins.

## Domain Services

### Role service (`lib/greyseal/role/`)

Thin CRUD service around the `roles` table. No business logic beyond delegation to the repository. Exposes `List`, `Get`, `Create`, `Update`, `Delete`.

### Conversation service (`lib/greyseal/conversation/`)

The core domain service. Handles both CRUD on conversations and the `Chat` method, which orchestrates:

1. Persist the incoming user `Message` to the database.
2. Load the `Conversation` record to read `role_uuid` and `resource_uuids`.
3. If `role_uuid` is set, fetch the `Role` and prepend its `system_prompt` as a system message.
4. Load the 10 most recent prior messages as conversation history.
5. Query **shrike** (`Searcher` interface) with the user's message, optionally scoped to the conversation's `resource_uuids`. Inject returned snippets as a second system message.
6. Append message history and the new user turn.
7. Call the **LLM** (`LLM` interface) with the assembled message list; stream each token back through the Connect server-stream.
8. Persist the assistant response.
9. Update `conversations.updated_at`.

The `SubmitFeedback` method writes -1/0/1 to `messages.feedback`.

### Resource service

The `ResourceService` gRPC handler is wired but its domain service implementation is not present in the codebase. The `ingest` CLI command calls `IngestResource` directly against the Connect endpoint, and `ResourceRepo` provides PostgreSQL persistence of resource metadata.

## Repository Layer (`lib/repo/`)

All repositories embed `*Conn`, which holds a `*sql.DB`. SQL is built with `Masterminds/squirrel` using the `$N` placeholder format. PostgreSQL arrays (`TEXT[]`) are handled with `lib/pq.Array`. Timestamps are stored as `TIMESTAMP WITH TIME ZONE`.

`NewDatabase` runs goose migrations automatically on startup from an embedded FS (`//go:embed migrations/*.sql`).

## LLM Adapter (`lib/repo/ollama/`)

`ollama.LLM` implements `conversation.LLM`. It POSTs to Ollama's `/api/chat` endpoint with `"stream": true` and reads newline-delimited JSON chunks, invoking the provided callback per token. Configuration is via `OLLAMA_HOST` and `OLLAMA_CHAT_MODEL` environment variables (defaults: `http://localhost:11434`, `deepseek-r1`).

## Search Adapter

`shrikeSearcher` implements `conversation.Searcher` by calling `shrikeconnect.SearchServiceClient.Search` with `mode: "hybrid"`. Results are filtered client-side to the conversation's `resource_uuids` set if non-empty.

## Worker

The worker binary connects to PostgreSQL and then blocks waiting for an OS signal. The compose file also provides it with `KAFKA_BROKERS`, `QDRANT_HOST`, and `OLLAMA_EMBED_MODEL`, suggesting its intended purpose is asynchronous resource ingestion (chunking, embedding, indexing into Qdrant via Redpanda events). This logic is not yet implemented in source.

## UI (`lib/ui/`, `cmd/ui/`)

All UI files carry `//go:build ignore` and are excluded from normal compilation. The UI is a WebAssembly single-page application built with `go-app` v9 (Pico CSS for styling). It exposes routes for Messages, Conversations, Resources, and Roles with full CRUD pages.

## CLI (`cmd/`)

The root Cobra command is `grey-seal`. The only active subcommand is `ingest`. The CRUD command files (`conversation_cmd.go`, `resource_cmd.go`, `role_cmd.go`) also carry `//go:build ignore` and are not compiled.

## External Dependencies (key)

| Package | Role |
|---|---|
| `connectrpc.com/connect` | Connect-RPC server and client |
| `connectrpc.com/cors` | CORS headers for Connect |
| `github.com/holmes89/archaea` | Generic base types (Repository, Service, ListRequest/Response) |
| `github.com/holmes89/shrike` | External vector search service (client stubs only) |
| `github.com/Masterminds/squirrel` | SQL query builder |
| `github.com/pressly/goose/v3` | Database migrations |
| `github.com/spf13/cobra` | CLI framework |
| `github.com/google/uuid` | UUID generation |
| `github.com/lib/pq` | PostgreSQL driver + array support |
