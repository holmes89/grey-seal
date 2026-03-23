# grey-seal

grey-seal is a Retrieval-Augmented Generation (RAG) chat backend written in Go. It manages **conversations**, **roles** (system prompts), and **resources** (indexed documents). At runtime it wires together a PostgreSQL store, an Ollama LLM, and an external vector-search service called **shrike** to answer user queries grounded in an indexed knowledge base.

## Features

- Streaming chat via a Connect-RPC server-streaming RPC (`Chat`)
- Role-based system prompts that can be assigned per conversation
- Resource scoping: a conversation can restrict retrieval to a named set of indexed documents
- Message-level feedback recording (-1 / 0 / 1)
- Automatic schema migrations using goose (embedded in the binary)
- CLI (`ingest`) for submitting URLs or raw text to the knowledge base
- Optional management web UI (compiled with go-app, currently excluded from the default build)

## Quick Start

### Prerequisites

| Dependency | Purpose |
|---|---|
| PostgreSQL 17 | Persistent store |
| Ollama | LLM inference and embeddings |
| shrike | Vector search / hybrid retrieval |
| Qdrant | Vector database (used by the worker) |
| Redpanda | Kafka-compatible message broker (used by the worker) |

### Run with Docker Compose

```sh
docker compose up
```

The compose file starts PostgreSQL, Qdrant, Ollama, Redpanda, the API server, and the worker. The API is reachable on port **9000**.

### Environment Variables

#### API server (`cmd/api/main.go`)

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | _(required)_ | PostgreSQL connection string |
| `OLLAMA_HOST` | `http://localhost:11434` | Ollama base URL |
| `OLLAMA_CHAT_MODEL` | `deepseek-r1` | Model name for chat completions |
| `SHRIKE_URL` | `http://shrike:9000` | Vector search service URL |

#### Worker (`cmd/worker/main.go`)

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | _(required)_ | PostgreSQL connection string |

### Ingest a resource from the CLI

```sh
# Ingest a website
grey-seal ingest --name "Go docs" --url https://go.dev/doc

# Ingest literal text
grey-seal ingest --name "My note" --text "Some content to index"

# Target a non-default server
grey-seal ingest --name "Doc" --url https://example.com --server api.example.com:9000
```

`--name` is required; exactly one of `--url` or `--text` must be supplied.

## Building

```sh
# API server
go build -o api ./cmd/api/main.go

# Worker
go build -o worker ./cmd/worker/main.go
```

Docker images use a multi-stage `scratch`-based build for minimal image size.

## Protobuf / Connect-RPC

Schemas live in `schemas/greyseal/v1/`. Generated Go code is committed under `lib/schemas/`. Regenerate with:

```sh
buf generate
```

`buf.gen.yaml` produces protobuf Go types, Connect-RPC handlers, and gRPC stubs.

## Project Layout

```
cmd/
  api/        – HTTP/2 API server (Connect-RPC, port 9000)
  worker/     – Background worker skeleton
  ui/         – WebAssembly management UI (build-tagged ignore)
  ingest.go   – CLI subcommand for ingesting resources
  root.go     – Cobra root command
lib/
  greyseal/
    conversation/ – Chat domain: service, interfaces, gRPC handler
    role/         – Role domain: service, interfaces, gRPC handler
  repo/           – PostgreSQL repository implementations + goose migrations
  repo/ollama/    – Ollama LLM adapter
  schemas/        – Generated protobuf + Connect-RPC Go code
  ui/             – go-app UI pages and components (build-tagged ignore)
schemas/          – Protobuf source files
```
