---
uuid: "ccf69d41-9b0b-5169-a9a4-8bbe07487c6c"
kind: "design"
product_uuid: "aa14cb8f-ba36-5afe-abc9-40d26b422f89"
project_uuid: "9f94971e-2224-5948-bcd4-eed08b50d911"
portal_url: "https://joel.holmes.haus/discovery/ccf69d41-9b0b-5169-a9a4-8bbe07487c6c"
design_url: "https://joel.holmes.haus/designs/03a248a4-16e9-520e-91ad-66d197425b2b"
file: "docs/designs/0-mvp.md"
body_sha256: "2f4f7e5b5ea016dccaefc002d8c91512ad65e0fca86caa2cbad4ae71a30a51af"
source: "narwhal-catalog:joel.holmes.haus/research/services/grey-seal/0-mvp.md"
parent: "https://github.com/holmes89/narwhal/blob/main/designs/joel.holmes.haus/system.md"
---

# Grey Seal — RAG Chat Backend

> Retrieval-Augmented Generation chat service. Manages conversations, system-prompt roles, and a knowledge base of indexed resources.

## Goal

Grey Seal provides a persistent, context-aware chat backend grounded in an indexed knowledge base. The system stores conversations and can scope retrieval to a specific set of resources. LLM inference is handled by a local Ollama instance; semantic search is delegated to Shrike. Users interact via streaming Connect-RPC (`Chat`) or the CLI ingest tool.

## Architecture

```
CLI / Browser / Other services
    │ Connect-RPC HTTP/2 (:9000)
    ▼
cmd/api
  ├── RoleService     (CRUD system prompts)
  ├── ConversationService (CRUD + Chat streaming + SubmitFeedback)
  └── ResourceService (CRUD resource metadata)
       │
       ├── PostgreSQL (roles, conversations, messages, resources)
       ├── Ollama     (LLM chat completions — deepseek-r1)
       └── Shrike     (vector semantic search → context chunks)
```

## Key Entities

### `Role`
Reusable named system prompt assigned to a conversation.

| Field | Type | Notes |
|---|---|---|
| `uuid` | string | |
| `name` | string | |
| `system_prompt` | string | Injected as first system message in LLM call |

### `Conversation`

| Field | Type | Notes |
|---|---|---|
| `uuid` | string | |
| `title` | string | Optional; auto-generated from first exchange |
| `role_uuid` | string | FK to Role; empty = no system prompt |
| `resource_uuids` | repeated string | Scope retrieval to specific docs; empty = all |
| `summary` | string | Rolling compressed summary for context management |
| `messages` | repeated Message | Populated on Get; absent on List |

### `Message`

| Field | Type | Notes |
|---|---|---|
| `role` | MessageRole enum | `USER` or `ASSISTANT` |
| `content` | string | |
| `resource_uuids` | repeated string | Resources cited in assistant reply |
| `feedback` | int32 | -1 / 0 / 1 quality rating |

### `Resource`
Metadata record for an indexed document (actual chunks live in Shrike/Qdrant).

| Field | Type | Notes |
|---|---|---|
| `service` | string | Originating service (e.g. `lynx`, `weevil`) |
| `entity` | string | Entity type (e.g. `Website`, `Book`) |
| `source` | Source enum | `WEBSITE`, `PDF`, `TEXT` |
| `path` | string | URL or content |
| `indexed_at` | Timestamp | When embeddings were stored |

## Binaries

| Binary | Port | Purpose |
|---|---|---|
| `cmd/api` | 9000 | ConnectRPC services — Role, Conversation, Resource |
| `cmd/worker` | — | Skeleton; Kafka resource ingestion (future) |
| CLI (`ingest`) | — | `--url` / `--text` submission to knowledge base |

## Service RPCs

`RoleService`: List, Get, Create, Update, Delete  
`ConversationService`: List, Get, Create, Update, Delete, **Chat** (server-streaming), SubmitFeedback  
`ResourceService`: List, Get, Create, Update, Delete

## Dependencies

| Dependency | Purpose |
|---|---|
| PostgreSQL | All entity storage |
| Ollama | LLM chat completions (`OLLAMA_CHAT_MODEL`, default `deepseek-r1`) |
| Shrike | Semantic vector search over indexed chunks |
| Qdrant | Vector DB used by Shrike (not directly queried by grey-seal) |
| Kafka / Redpanda | Future resource ingestion worker |

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | required | PostgreSQL connection string |
| `OLLAMA_HOST` | `http://localhost:11434` | Ollama base URL |
| `OLLAMA_CHAT_MODEL` | `deepseek-r1` | LLM model for chat |
| `SHRIKE_URL` | `http://shrike:9000` | Vector search service |

## MVP Success Criteria

- [ ] Create a Role; assign it to a Conversation and verify the system prompt is injected
- [ ] `Chat` streaming RPC returns tokens as they are generated
- [ ] Ingest a resource via CLI; Shrike returns relevant chunks when queried about it
- [ ] Scoped conversation (specific `resource_uuids`) only retrieves from those documents
- [ ] `SubmitFeedback` records -1/0/1 on a message
- [ ] Conversation rolls up older messages into `summary` to stay within context window
