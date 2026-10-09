---
uuid: "2c866742-e11c-5323-a603-e4105addd44b"
kind: "feature"
product_uuid: "aa14cb8f-ba36-5afe-abc9-40d26b422f89"
project_uuid: "9f94971e-2224-5948-bcd4-eed08b50d911"
design_uuid: "aead0381-278c-500e-901f-f649417a3a77"
portal_url: "https://joel.holmes.haus/discovery/2c866742-e11c-5323-a603-e4105addd44b"
design_url: "https://joel.holmes.haus/designs/aead0381-278c-500e-901f-f649417a3a77"
file: "docs/features/08-grey-seal-rag.md"
body_sha256: "fb9cde8e4aeb2ede24b712a7e2ab006ff7d884d0bb0a72223764330d078b01d1"
product: joel.holmes.haus
type: feature
status: draft
source: "narwhal-catalog:joel.holmes.haus/features/08-grey-seal-rag.md"
parent: "https://github.com/holmes89/narwhal/blob/main/designs/joel.holmes.haus/system.md"
---
# Feature: Grey-seal — Conversational Knowledge Assistant

> _"How does it feel to be so wise."_ — Elton John. The goal is to make that wisdom earned: grounded in the user's actual curated content, not LLM priors.

## Overview

Grey-seal is the chat interface for the joelholmes.dev knowledge platform. Its job is to help the user think through and explore their collected content — saved websites (Lynx), books and papers (Owl), documents, and anything else ingested through the platform.

Grey-seal is not an indexing or ingestion service. It owns conversation state, prompt assembly, and streaming responses. All content retrieval is delegated to Shrike.

## Architecture

```
User message
      │
      ▼
      ┌──────────────────────────────────────────┐
      │              grey-seal                   │
      │                                          │
      │  1. Load conversation + history          │
      │  2. Call Shrike.Search                   │
      │     merge results into Redis cache       │
      │  3. Assemble RAG prompt                  │
      │     [ system prompt / role ]             │
      │     [ resource context block ]           │
      │     [ conversation history ]             │
      │     [ user message ]                     │
      │  4. golangchain → Ollama (streaming)     │
      │  5. Persist messages + resource refs     │
      │  6. Stream tokens + citations to UI      │
      └──────────────────────────────────────────┘
```

### Responsibilities

| Concern | Owner |
|---|---|
| Entity ingestion (websites, books, etc.) | Lynx, Owl, and other upstream services |
| Tagging and labelling | Magpie |
| Chunking, embedding, vector index | Shrike |
| Semantic search | Shrike (`SearchService` gRPC) |
| Conversation state and history | Grey-seal (Postgres) |
| Resource context cache | Grey-seal (Redis) |
| Prompt assembly and RAG loop | Grey-seal |
| LLM interaction | Grey-seal via golangchain → Ollama |

## Conversations and Resources

A `Conversation` is a persistent chat session. It has:

- **Messages** — user and assistant turns stored in Postgres
- **Resources** — references to indexed entities that provide context

Resources attach to a conversation two ways:

1. **User-supplied** — the user explicitly scopes the conversation to specific resources ("talk to me about this book"). Stored as `resource_uuids` on the `Conversation`; used as a filter on every Shrike search in that thread.
2. **Retrieved** — grey-seal searches Shrike each turn and records the returned entity UUIDs as `resource_uuids` on the assistant `Message`.

Both are tracked identically. The UI shows a resource panel alongside the chat — each cited resource links back to its source entity in the originating service (Lynx, Owl, etc.).

### Resource Cache (Redis)

Key: `greyseal:conv:{uuid}:resources` — a JSON map of `entity_uuid → CachedResource`.

```go
type CachedResource struct {
    EntityUUID string
    Title      string
    Snippet    string  // best snippet seen for this entity in this conversation
    Service    string  // "lynx", "owl", etc.
    SourceURL  string
    Score      float32
}
```

**Cache behaviour in the Chat loop:**

1. Call Shrike on every turn — fresh results may surface different or better chunks.
2. Merge new results into the cache: higher-scored results overwrite lower for the same `entity_uuid`.
3. Assemble context from the merged cache, up to 8 entries sorted by score descending.
4. TTL: 24 hours, reset on each write. Covers active conversation lifetime without manual eviction.

**Why a cache?**
`messages.resource_uuids` stores UUIDs but not snippet text. Re-fetching the enriched form (snippet + metadata) from Shrike on every prompt assembly adds latency and repeats calls for resources already seen in the conversation. The cache is the enrichment layer on top of the Postgres record.

**Open question — upstream enrichment:**
Should grey-seal optionally call the originating service (Lynx, Owl) for richer content than Shrike's 512-token snippet? Starting with Shrike snippets only. Revisit if snippet quality proves insufficient for deep single-resource questions.

## LLM — golangchain + Ollama

Grey-seal uses [golangchain](https://github.com/tmc/langchaingo) for all LLM interaction. The existing `LLM` interface in the conversation package is retained; the implementation moves from a hand-rolled HTTP client to a golangchain adapter.

```go
// lib/repo/llm/golangchain.go
type LangchainLLM struct {
    model llms.Model
}

func New(ollamaHost, modelName string) (*LangchainLLM, error) {
    m, err := ollama.New(
        ollama.WithModel(modelName),
        ollama.WithServerURL(ollamaHost),
    )
    // ...
    return &LangchainLLM{model: m}, nil
}

func (l *LangchainLLM) Chat(ctx context.Context, messages []conversation.LLMMessage, stream func(token string) error) (string, error) {
    // convert to []llms.MessageContent
    // call GenerateContent with llms.WithStreamingFunc(...)
}
```

Config: `OLLAMA_HOST` and `OLLAMA_CHAT_MODEL` (unchanged from current).

## Prompt Structure

```
┌────────────────────────────────────────────────────┐
│ SYSTEM PROMPT (from Role, or default below)        │
├────────────────────────────────────────────────────┤
│ RESOURCE CONTEXT                                   │
│ [Source 1: "Title" — service]                      │
│ snippet text...                                    │
│                                                    │
│ [Source 2: "Title" — service]                      │
│ snippet text...                                    │
├────────────────────────────────────────────────────┤
│ CONVERSATION HISTORY  (last 10 turns)              │
├────────────────────────────────────────────────────┤
│ USER MESSAGE                                       │
└────────────────────────────────────────────────────┘
```

**Default system prompt:**

```
You are grey-seal, a personal knowledge assistant. You have access to the
user's curated library — saved websites, books, papers, and documents.

Use the retrieved context passages below to ground your answers. Prefer
evidence from those passages over your general knowledge. If the context
is insufficient, say so — do not fabricate.

After your answer, cite the sources you drew from:
  • Title (service) — url if available

Skip citations for conversational exchanges.
```

**Context limits:**
- Max 8 resources per turn (highest score first from merged cache)
- ~512 tokens per snippet → ~4,000 tokens for the context block
- Last 10 message turns in history
- Truncation priority: system → user message → top 3 resources → history → remaining

## Schema

No schema changes required for the cache (Redis only). Existing Postgres schema supports the model:

- `conversations.resource_uuids` — user-scoped resources for the whole conversation
- `messages.resource_uuids` — resources cited in each assistant response

The `resources` table stores entity metadata. The cache is the runtime enrichment layer (snippet + score) on top.

## Implementation Plan

| Step | Deliverable | Status |
|---|---|---|
| 1 | Replace `lib/repo/ollama/llm.go` with `lib/repo/llm/golangchain.go` — same `LLM` interface | Done |
| 2 | Add `ResourceCache` interface to `lib/greyseal/conversation/interface.go` | Done |
| 3 | Implement Redis `ResourceCache` in `lib/repo/cache/resource_cache.go` | Done |
| 4 | Inject `ResourceCache` into `conversationService`; update `Chat` to merge cache and populate `resource_uuids` on assistant message | Pending |
| 5 | Wire up Redis client in `cmd/`; add `REDIS_URL` env var | Pending |

## Open Questions

| # | Question | Status |
|---|---|---|
| 1 | Upstream enrichment — call Lynx/Owl for full content vs. trust Shrike snippets? | Open — start with snippets only |
| 2 | Cache eviction — 24h TTL or tie to conversation `updated_at`? | Resolved — 24h TTL, reset on each write |
| 3 | Warm cache when user manually attaches resources to a conversation? | Planned — call Shrike `GetEntity` on attach |
| 4 | Intent classification to skip retrieval on conversational turns? | Defer — default to always retrieve |
| 5 | Context summarisation for long conversations? | Defer — `Conversation.summary` field already reserved |

## Dependencies

- Shrike `SearchService` gRPC (see [03-shrike-mvp.md](03-shrike-mvp.md))
- `github.com/tmc/langchaingo` + Ollama backend
- Redis
- Postgres (existing)

## Success Metrics

- Time to first token < 2s (p95)
- Resource attribution present on all Shrike-grounded responses
- No redundant Shrike snippet fetches for resources already in the conversation cache
- Conversations and resource references survive service restarts
