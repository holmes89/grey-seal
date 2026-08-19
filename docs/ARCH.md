# Architecture

## System Context

```mermaid
C4Context
  title System Context — Grey Seal within joel.holmes.haus

  Person(admin, "Admin", "Manages conversations and resources; chats with the assistant")

  Boundary(platform, "joel.holmes.haus Platform") {
    System(ui, "joel.holmes.haus", "Go-app WASM admin SPA")
    System(greyseal, "Grey Seal", "RAG-powered conversation, resource, and agent-run management service")
    System(shrike, "Shrike", "Search — provides hybrid semantic/keyword context retrieval")
    System(magpie, "Magpie", "Resource hub — receives ingested resources from Grey Seal")
  }

  SystemDb(postgres, "PostgreSQL", "Conversations, messages, roles, resources, agent runs")
  SystemDb(redis, "Redis", "Per-conversation resource snippet cache (24 h TTL)")
  System_Ext(ollama, "Ollama", "Local LLM (deepseek-r1) — streaming chat completions")
  SystemQueue(kafka, "Kafka", "greyseal.v1.Resource (ingest queue) · shrike.v1.TextExtractedEvent")
  System_Ext(managedagents, "Anthropic Managed Agents", "Claude — hosted agentic coding-task runs against a target repo")
  System_Ext(github, "GitHub REST API", "Opens pull requests once an agent run's outcome is satisfied")

  Rel(admin, ui, "Uses")
  Rel(ui, greyseal, "ConnectRPC")
  Rel(greyseal, postgres, "Reads / writes")
  Rel(greyseal, redis, "Resource snippet cache")
  Rel(greyseal, ollama, "Streaming LLM chat")
  Rel(greyseal, shrike, "ConnectRPC hybrid search for context")
  Rel(greyseal, kafka, "Publishes greyseal.v1.Resource")
  Rel(kafka, greyseal, "async ingest queue")
  Rel(kafka, magpie, "greyseal.v1.Resource")
  Rel(greyseal, managedagents, "Starts/polls/streams coding-agent sessions")
  Rel(greyseal, github, "Opens pull request via REST once outcome is satisfied")
```

## Container Diagram

```mermaid
C4Container
  title Grey Seal — Internal Containers

  Boundary(greyseal, "Grey Seal") {
    Container(api, "cmd/api", "Go / ConnectRPC h2c :9000", "ConversationService · ResourceService · RoleService")
    Container(worker, "cmd/worker", "Go / Kafka", "Async content fetch — scrapes websites/PDFs, publishes TextExtractedEvent to Shrike")

    Container(convSvc, "conversation.Service", "Go", "Chat (RAG pipeline) · CRUD · SubmitFeedback · summarisation")
    Container(resourceSvc, "resource.Service", "Go", "Ingest · CRUD — triggers async indexing via KafkaIndexer")
    Container(roleSvc, "role.Service", "Go", "CRUD for system prompt roles")
    Container(cache, "RedisResourceCache", "Go / Redis", "Per-conversation snippet cache keyed greyseal:conv:{uuid}:resources")
    Container(agentSvc, "agent.Service", "Go", "RunAgentTask · GetAgentRun · StreamAgentRun — watches sessions to completion, opens PR")
    Container(sessionRunner, "managedagents.SessionRunner", "Go / Anthropic SDK", "Starts/polls/streams Managed Agents sessions")
    Container(prOpener, "github.Client", "Go / net-http", "Opens a pull request via the GitHub REST API")

    ContainerDb(convRepo, "ConversationRepo + MessageRepo", "PostgreSQL / squirrel", "conversations · messages tables")
    ContainerDb(resourceRepo, "ResourceRepo", "PostgreSQL / squirrel", "resources table")
    ContainerDb(agentRepo, "AgentRunRepo", "PostgreSQL / squirrel", "agent_runs table")
  }

  SystemDb(postgres, "PostgreSQL", "")
  SystemDb(redis, "Redis", "")
  System_Ext(ollama, "Ollama", "POST /api/chat stream")
  System_Ext(shrike, "Shrike", "ConnectRPC SearchService")
  SystemQueue(kafka, "Kafka", "")
  System_Ext(managedagents, "Anthropic Managed Agents", "")
  System_Ext(github, "GitHub REST API", "")

  Rel(api, convSvc, "Chat · CRUD")
  Rel(api, resourceSvc, "Ingest · CRUD")
  Rel(api, roleSvc, "CRUD")
  Rel(api, agentSvc, "RunAgentTask · GetAgentRun · StreamAgentRun")
  Rel(convSvc, cache, "Cache-first context lookup")
  Rel(convSvc, shrike, "Hybrid search with EntityUUIDs filter")
  Rel(convSvc, ollama, "Streaming completion")
  Rel(convSvc, convRepo, "Persist messages + summary")
  Rel(resourceSvc, resourceRepo, "CRUD")
  Rel(resourceSvc, kafka, "Publishes TextExtractedEvent or greyseal.v1.Resource")
  Rel(kafka, worker, "greyseal.v1.Resource")
  Rel(worker, kafka, "Publishes shrike.v1.TextExtractedEvent")
  Rel(convRepo, postgres, "SQL")
  Rel(resourceRepo, postgres, "SQL")
  Rel(agentRepo, postgres, "SQL")
  Rel(cache, redis, "GET / SET")
  Rel(agentSvc, sessionRunner, "StartSession · GetSessionStatus · StreamSession")
  Rel(agentSvc, prOpener, "OpenPullRequest once outcome is satisfied")
  Rel(agentSvc, agentRepo, "Persist run status/PR URL")
  Rel(sessionRunner, managedagents, "Beta Sessions API")
  Rel(prOpener, github, "POST /repos/{owner}/{repo}/pulls")
```

## Overview

grey-seal is a single-binary Go service (`cmd/api`) that exposes a Connect-RPC API over HTTP/2 (h2c). It uses a layered architecture: a thin gRPC/Connect handler layer delegates to domain service interfaces, which are backed by PostgreSQL repositories. LLM inference is delegated to a local Ollama instance; semantic search is delegated to the external **shrike** service. Resources are ingested asynchronously via **Redpanda/Kafka**: the API enqueues events that the **worker** process consumes to fetch content and forward it to shrike for chunking, embedding, and vector indexing. A fourth domain, **agent**, orchestrates agentic coding-task runs against Claude via Anthropic's Managed Agents API and opens the resulting pull request directly through the GitHub REST API; its route is registered only when `AGENT_ID`/`ENVIRONMENT_ID` are configured (see [`cmd/setup-agent`](#process-inventory)).

```mermaid
graph TD
  Client["Clients\nCLI · Browser · Other services"]
  API["cmd/api :9000\nhttp.ServeMux · CORS · /health"]

  Client -->|"Connect-RPC HTTP/2"| API

  API --> RH["RoleHandler (CRUD)"] --> RS["RoleService"] --> RoleRepo["RoleRepo"]
  API --> CH["ConversationHandler\nCRUD · Chat · Feedback"] --> CS["ConversationService"]
  API --> RsH["ResourceHandler (CRUD)"] --> ResourceSvc["ResourceService"]

  CS --> ConvRepo["ConvRepo · MessageRepo"]
  CS -->|"hybrid search"| Shrike["shrike SearchService"]
  CS -->|"streaming completion"| Ollama["Ollama LLM"]
  CS --> Cache["Redis Cache\nper-conversation snippets"]
  ResourceSvc --> ResourceRepo["ResourceRepo"]
  ResourceSvc -->|"KafkaIndexer"| Kafka[("Kafka")]

  RoleRepo --> PG[("PostgreSQL")]
  ConvRepo --> PG
  ResourceRepo --> PG
```

```mermaid
graph TD
  subgraph ingest["Async Ingestion Pipeline"]
    Ingest["ResourceService.Ingest"]
    Ingest -->|"SOURCE_TEXT"| KTE["Kafka: shrikev1.TextExtractedEvent"]
    Ingest -->|"SOURCE_WEBSITE / PDF"| KRes["Kafka: greysealv1.Resource"]

    KRes --> Worker["cmd/worker\nFetchContent HTTP scrape / PDF"]
    Worker -->|"text extracted"| KTE2["Kafka: shrikev1.TextExtractedEvent"]
  end

  KTE -->|"consumed by"| Shrike["Shrike\nchunk → embed → Qdrant"]
  KTE2 -->|"consumed by"| Shrike
```

## Process Inventory

| Process | Source | Port | Notes |
|---|---|---|---|
| API server | `cmd/api/main.go` | 9000 | Active, ships in `Dockerfile` |
| Worker | `cmd/worker/main.go` | — | Kafka consumer; fetches web/PDF content and forwards to shrike |
| setup-agent | `cmd/setup-agent/main.go` | — | One-time (or occasional-update) operator command; provisions the Managed Agents Agent + Environment and prints `AGENT_ID`/`ENVIRONMENT_ID` for the API server's env. Never run from the request path |
| UI | `cmd/ui/main.go` | 8000 | `//go:build ignore`; excluded from normal builds |

## Transport

The API server uses `h2c` (cleartext HTTP/2) via `golang.org/x/net/http2/h2c`, making it compatible with both native gRPC clients and the Connect-RPC `grpc-web` protocol. CORS is applied per-handler using `connectrpc.com/cors` helper headers, allowing wildcard origins.

## Domain Services

### Role service (`lib/greyseal/role/`)

Thin CRUD service around the `roles` table. No business logic beyond delegation to the repository. Exposes `List`, `Get`, `Create`, `Update`, `Delete`.

### Resource service (`lib/greyseal/resource/`)

Manages resource metadata (title, URL, source type, timestamps). Exposes `List`, `Get`, `Ingest`, `Delete`.

`Ingest` assigns a UUID and `created_at`, persists the record, then triggers async indexing via the `Indexer` interface. When `KAFKA_BROKERS` is set the real `KafkaIndexer` is wired; otherwise the indexer is `nil` and indexing is skipped (graceful degradation).

`KafkaIndexer` publishes:
- `SOURCE_TEXT` → `shrikev1.TextExtractedEvent` (topic `v1.TextExtractedEvent`) directly to shrike's consumer
- `SOURCE_WEBSITE` / `SOURCE_PDF` → `greysealv1.Resource` (topic `v1.Resource`) to the worker queue

### Conversation service (`lib/greyseal/conversation/`)

The core RAG orchestration service. Handles CRUD on conversations and the `Chat` method, which:

1. Persist the incoming user `Message`.
2. Load the `Conversation` record (`role_uuid`, `resource_uuids`, `summary`).
3. If `role_uuid` is set, fetch the `Role` and prepend its `system_prompt` as a system message.
4. Load prior message history. If history exceeds 10 messages, summarise the overflow via a second LLM call and persist the summary to `conversations.summary`. Prepend the (existing or freshly generated) summary as a system message.
5. Retrieve relevant context via `contextSearch` (cache-first): check the per-conversation `ResourceCache` first; on a miss, call **shrike** (`Searcher`) with `EntityUuids` filter, then populate the cache. Format snippets as `"N. [Title]: snippet"` for source attribution.
6. Append message history and the current user turn.
7. Call the **LLM** (`LLM` interface); stream each token via the Connect server-stream callback.
8. Persist the assistant response and update `conversations.updated_at`.

`SubmitFeedback` writes -1/0/1 to `messages.feedback`.

`ResourceCache` (`lib/repo/cache/RedisResourceCache`) stores per-conversation resource snippets in Redis (key `greyseal:conv:{uuid}:resources`, TTL 24 h). Wired when `REDIS_URL` is set; `nil` otherwise (no caching).

`TranscriptWriter` (`lib/repo/transcript/Writer`), when `TRANSCRIPT_DIR` is set, records a `TranscriptTurn` per `Chat` call — the assembled system prompt, summary, search query/results, full message list sent to the LLM, and the response — for offline review of the RAG pipeline. `nil` (no-op) otherwise.

### Agent service (`lib/greyseal/agent/`)

Orchestrates agentic coding-task runs against Claude via Anthropic's Managed Agents API. Exposes `RunAgentTask`, `GetAgentRun`, `StreamAgentRun`. Only wired when both `AGENT_ID` and `ENVIRONMENT_ID` are set (provisioned once via `cmd/setup-agent`); otherwise the route is skipped entirely.

`RunAgentTask`:
1. Generates a run UUID and derives a branch name `agent/<uuid>`.
2. Appends branch/PR instructions to the task description (push to that branch on `origin`; do not open a PR — grey-seal does that once the outcome is satisfied).
3. Calls `SessionRunner.StartSession` (adapter: `lib/repo/managedagents`, the only package importing the Anthropic SDK) to start a Managed Agents session against the target repo, then persists an `AgentRun` row (`status: "running"`) and returns it immediately.
4. Spawns a detached goroutine, `watchForCompletion`, that polls `SessionRunner.GetSessionStatus` on an interval (default 20s, bounded by a 1h timeout) — not tied to the originating request's context, but also not durable across a process restart.

`watchForCompletion` persists the latest status on each poll and, the first time the provider's outcome-evaluation result is `"satisfied"` (guarded by `AgentRun.PrUrl` being empty so a PR opens at most once), calls `PullRequestOpener.OpenPullRequest` (adapter: `lib/repo/github`, a direct GitHub REST call — deliberately not a GitHub MCP tool, so no MCP server/vault wiring is needed) and stores the resulting PR URL. Polling stops once the session is `"terminated"` or the outcome result is terminal (`satisfied` / `failed` / `max_iterations_reached` / `interrupted`).

`GetAgentRun` refreshes status from the live provider session (source of truth while a session is active) before returning the persisted row. `StreamAgentRun` relays provider SSE events (`agent.message`, `agent.tool_use`/`agent.mcp_tool_use`, `session.status_idle`, `session.status_terminated`, `session.error`) via `SessionRunner.StreamSession`.

Only `provider: "claude"` is implemented; `"ollama:<model>"` is a reserved, not-yet-implemented value. Persisted in the `agent_runs` table (`uuid`, `provider`, `repo_url`, `status`, `session_id`, `pr_url`, `error`, `created_at`, `updated_at`).

## Worker (`cmd/worker/`)

Consumes the `v1.Resource` Kafka topic via `archaea/kafka.Consumer`. For each resource:
1. Calls `resource.FetchContent` to retrieve the raw text (HTTP scrape for websites; placeholder for PDFs).
2. Publishes a `shrikev1.TextExtractedEvent` to shrike's Kafka topic for chunking, embedding, and Qdrant indexing.
3. Updates `resources.indexed_at` in PostgreSQL.

Requires `KAFKA_BROKERS` and `DATABASE_URL` environment variables.

## Repository Layer (`lib/repo/`)

All repositories embed `*Conn`, which holds a `*sql.DB`. SQL is built with `Masterminds/squirrel` using the `$N` placeholder format. PostgreSQL arrays (`TEXT[]`) are handled with `lib/pq.Array`. Timestamps are stored as `TIMESTAMP WITH TIME ZONE`.

`NewDatabase` runs goose migrations automatically on startup from an embedded FS (`//go:embed migrations/*.sql`).

## LLM Adapter (`lib/repo/ollama/`)

`ollama.LLM` implements `conversation.LLM`. It POSTs to Ollama's `/api/chat` endpoint with `"stream": true` and reads newline-delimited JSON chunks, invoking the provided callback per token. Configuration is via `OLLAMA_HOST` and `OLLAMA_CHAT_MODEL` environment variables (defaults: `http://localhost:11434`, `deepseek-r1`), plus `OLLAMA_THINK=true` to request the model's thinking/reasoning trace.

A second, unwired implementation exists at `lib/repo/llm/golangchain.go`, wrapping the same Ollama backend via `tmc/langchaingo` instead of a hand-rolled HTTP client. `cmd/api` does not construct or use it.

## Search Adapter

`shrikeSearcher` implements `conversation.Searcher` by calling `shrikeconnect.SearchServiceClient.Search` with `mode: "hybrid"` and a `SearchFilter.EntityUuids` field when the conversation is scoped to specific resources. Server-side filtering eliminates the need for a client-side loop.

## UI (`lib/ui/`, `cmd/ui/`)

All UI files carry `//go:build ignore` and are excluded from normal compilation. The UI is a WebAssembly single-page application built with `go-app` v9 (Pico CSS for styling). It exposes routes for Messages, Conversations, Resources, and Roles with full CRUD pages.

## CLI (`cmd/`)

The root Cobra command is `grey-seal`. The only active subcommand is `ingest`. The CRUD command files (`conversation_cmd.go`, `resource_cmd.go`, `role_cmd.go`) also carry `//go:build ignore` and are not compiled.

## External Dependencies (key)

| Package | Role |
|---|---|
| `connectrpc.com/connect` | Connect-RPC server and client |
| `connectrpc.com/cors` | CORS headers for Connect |
| `github.com/holmes89/archaea` | Generic base types + Kafka producer/consumer |
| `github.com/holmes89/shrike` | External vector search + text extraction service |
| `github.com/redis/go-redis/v9` | Redis client for resource snippet cache |
| `github.com/Masterminds/squirrel` | SQL query builder |
| `github.com/pressly/goose/v3` | Database migrations |
| `github.com/spf13/cobra` | CLI framework |
| `github.com/google/uuid` | UUID generation |
| `github.com/lib/pq` | PostgreSQL driver + array support |
| `github.com/anthropics/anthropic-sdk-go` | Managed Agents API — agent session lifecycle, `cmd/setup-agent` provisioning |
| `github.com/twmb/franz-go` | Kafka client underlying `archaea`'s producer/consumer |
| `github.com/XSAM/otelsql` | OTel instrumentation for `database/sql` |
| `github.com/tmc/langchaingo` | Alternate Ollama-backed `conversation.LLM` implementation (`lib/repo/llm`); not wired into `cmd/api` — the active adapter is `lib/repo/ollama` |
