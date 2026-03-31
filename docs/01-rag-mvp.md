# RAG MVP: Grey-seal ↔ Shrike Integration Plan

## Overview

grey-seal is the **decoder**: it assembles role system prompts, retrieves context snippets from shrike, manages conversation history, and streams LLM responses via Ollama. **Shrike is the encoder**: it owns chunking (512-word overlapping windows), embedding (Ollama `nomic-embed-text`, 768d), Qdrant storage, and semantic/keyword search.

grey-seal's job is only to **feed shrike over Kafka** and **query shrike over ConnectRPC**. Grey-seal must not duplicate chunking, embedding, or Qdrant interactions.

### Corrected Runtime Topology

```
[grey-seal CLI ingest]
        │
        ▼
[ResourceService API]
   ├── Postgres (resource metadata)
   └── Kafka publish ──────────────────────────────────────────────────────┐
           │ SOURCE_TEXT: TextExtractedEvent → "shrike-text-extracted"      │
           │ SOURCE_WEBSITE/PDF: Resource proto → "greyseal.resource.pending"│
                                                                             │
[grey-seal Worker] ←──── "greyseal.resource.pending" ──────────────────────┘
   ├── fetch content (HTTP scrape / PDF parse)
   └── publish TextExtractedEvent → "shrike-text-extracted"
                                         │
                                         ▼
                               [shrike Worker]
                           TextExtractedConsumer
                     chunk → embed → Qdrant upsert
                     + IndexRecord in Postgres

[Chat API] → shrike SearchService.Search → Qdrant (semantic) / Postgres (keyword)
           → Ollama LLM stream
```

---

## Shrike Encoding Recommendations

Before building grey-seal's side, these shrike schema changes are recommended. They unblock correct multi-resource scoping and improve overall RAG quality.

### Recommendation A (Required): Add `entity_uuids` to `SearchFilter`

`SearchFilter` currently supports `apps`, `entity_types`, `tags`, `after_date`, `before_date` — but no per-entity scoping. Grey-seal needs to restrict Qdrant search to the specific resources attached to a conversation.

Add to `schemas/shrike/v1/services/search.proto`:

```protobuf
message SearchFilter {
  repeated string apps         = 1;
  repeated string entity_types = 2;
  repeated string tags         = 3;
  string after_date            = 4;
  string before_date           = 5;
  repeated string entity_uuids = 6;  // ADD: restrict to specific indexed entities
}
```

On the shrike side, pass this as a Qdrant payload filter: `entity_uuid IN [...]`. Without this, grey-seal post-filters results client-side — which means if shrike returns limit=5 results but none belong to the conversation's resources, the context is empty even though matching chunks exist.

### Recommendation B (Recommended): Add a `TextExtractedEvent` Kafka topic constant to shrike's public config / docs

Grey-seal needs to know which Kafka topic to publish `TextExtractedEvent` to. Shrike's consumer group is `shrike-text-indexer` but the topic name is not in the README or schema. Standardise and document this (suggestion: `shrike.text-extracted`).

### Recommendation C (Nice to Have): Expose `GetIndexStats` filter

When grey-seal marks a resource `indexed_at`, it would be useful to confirm shrike has actually vectorised it. `GetIndexStats` could optionally accept an `entity_uuid` to return per-entity point counts.

---

## Phase 1 — Resource Domain Layer

> Nothing else can be built until `ResourceService` is registered. Currently `ResourceServiceHandler` is not registered in `cmd/api/main.go` — the ingest CLI gets a 404.

### 1.1 Create `lib/greyseal/resource/interface.go`

Mirror `lib/greyseal/role/interface.go`. Define:

```go
type ResourceService interface {
    List(ctx, base.ListRequest) (base.ListResponse[*greysealv1.Resource], error)
    Get(ctx, base.GetRequest[*greysealv1.Resource]) (base.GetResponse[*greysealv1.Resource], error)
    Ingest(ctx context.Context, data *greysealv1.Resource) (*greysealv1.Resource, error)
    Delete(ctx context.Context, id string) error
}

// Indexer is called after resource metadata is persisted.
// For SOURCE_TEXT it publishes a TextExtractedEvent immediately.
// For SOURCE_WEBSITE and SOURCE_PDF it enqueues async fetch work.
type Indexer interface {
    Index(ctx context.Context, resource *greysealv1.Resource) error
}
```

### 1.2 Create `lib/greyseal/resource/service.go`

Constructor:

```go
func NewResourceService(
    repo base.Repository[*greysealv1.Resource],
    indexer Indexer, // nil-safe
    logger *zap.Logger,
) ResourceService
```

`Ingest`:
1. Assign `uuid` and `created_at` if empty.
2. `repo.Create(ctx, data)`.
3. If `indexer != nil`, call `indexer.Index(ctx, data)` — log and continue on error (best-effort).
4. Return saved resource.

### 1.3 Create `lib/greyseal/resource/grpc/service.go`

Mirror `lib/greyseal/role/grpc/service.go`. Embed `servicesconnect.UnimplementedResourceServiceHandler`. Implement:
- `IngestResource` → `svc.Ingest(ctx, req.Msg.GetData())`
- `GetResource`, `ListResources`, `DeleteResource` → delegate to service

### 1.4 Register handler in `cmd/api/main.go`

```go
resourceRepo := &repo.ResourceRepo{Conn: store}
resourceSvc := resourcesvc.NewResourceService(resourceRepo, nil /* indexer wired in Phase 2 */, logger)
resourcePath, resourceHandler := servicesconnect.NewResourceServiceHandler(
    resourcegrpc.NewResourceHandler(resourceSvc),
)
mux.Handle(resourcePath, withCORS(resourceHandler))
```

**Files:** `lib/greyseal/resource/interface.go` (new), `lib/greyseal/resource/service.go` (new), `lib/greyseal/resource/grpc/service.go` (new), `cmd/api/main.go` (modify)

**Gate:** `grey-seal ingest --name test --text "hello"` returns a UUID.

---

## Phase 2 — Kafka Indexer (Encoder Bridge)

> This is the core gap. Resources are saved to Postgres but never reach shrike, so Qdrant is always empty and every search returns nothing.

### 2.1 Create `lib/greyseal/resource/kafka_indexer.go`

Implements `Indexer`. On `Index(ctx, resource)`:

**For `SOURCE_TEXT`** — publish a `TextExtractedEvent` directly to the shrike topic (no fetch needed):
```go
event := &shrikev1.TextExtractedEvent{
    EntityUuid: resource.Uuid,
    App:        "grey-seal",
    EntityType: "Resource",
    FullText:   resource.Path,    // inline text is in Path for SOURCE_TEXT
    Title:      resource.Name,
    SourceUrl:  resource.Path,
}
// marshal + publish to topic "shrike.text-extracted" (confirm topic name with shrike team)
```

**For `SOURCE_WEBSITE` / `SOURCE_PDF`** — publish a minimal message to the internal topic `greyseal.resource.pending` for the worker (Phase 3) to pick up asynchronously. Do not block the ingest RPC on content fetching.

The publisher wraps a thin Kafka producer (`base.Producer` from archaea, matching the pattern in the consumer files). Read `KAFKA_BROKERS` from env (already present in docker-compose worker env block).

### 2.2 Wire into `cmd/api/main.go`

```go
var indexer resource.Indexer
if brokers := os.Getenv("KAFKA_BROKERS"); brokers != "" {
    indexer = resource.NewKafkaIndexer(brokers, logger)
}
resourceSvc := resourcesvc.NewResourceService(resourceRepo, indexer, logger)
```

**Files:** `lib/greyseal/resource/kafka_indexer.go` (new), `cmd/api/main.go` (modify ~5 lines)

**Gate:** After `grey-seal ingest --text "some content"`, shrike's `IndexRecord` table has a new row and Qdrant has vector points for the resource UUID. Chat returns a relevant context snippet.

---

## Phase 3 — Grey-seal Worker: Content Fetching Pipeline

> For SOURCE_WEBSITE and SOURCE_PDF, content must be fetched before shrike can index it. The worker handles this asynchronously so the ingest RPC returns immediately.

### 3.1 Create `lib/greyseal/resource/fetcher.go`

```go
// FetchContent retrieves the full text of a resource based on its source type.
// SOURCE_TEXT: returns Path directly (no network call).
// SOURCE_WEBSITE: HTTP GET + HTML text extraction.
// SOURCE_PDF: placeholder returning "" for MVP; real implementation requires a PDF library.
func FetchContent(ctx context.Context, r *greysealv1.Resource) (string, error)
```

For `SOURCE_WEBSITE`, fetch `r.Path` and strip HTML using `golang.org/x/net/html` (already an indirect dep via `golang.org/x/net`).

For `SOURCE_PDF`, log a warning and return `""` with a descriptive error — do not silently swallow it.

### 3.2 Implement `cmd/worker/main.go` consumer loop

Read `DATABASE_URL`, `KAFKA_BROKERS` from env (already in docker-compose worker block).

```
Consume from topic "greyseal.resource.pending":
  1. Unmarshal Resource proto
  2. content, err := fetcher.FetchContent(ctx, resource)
  3. If err != nil → log, dead-letter or skip
  4. Publish TextExtractedEvent{EntityUuid, App:"grey-seal", EntityType:"Resource", FullText:content, Title:resource.Name, SourceUrl:resource.Path}
     to topic "shrike.text-extracted"
  5. Update resources.indexed_at = NOW() in Postgres (repo.ResourceRepo.Update)
```

No Qdrant client, no Ollama call, no chunking — shrike owns all of that.

**Files:** `lib/greyseal/resource/fetcher.go` (new), `cmd/worker/main.go` (rewrite)

**Gate:** `grey-seal ingest --url https://example.com` → worker fetches HTML, publishes `TextExtractedEvent` → shrike indexes vectors → Chat returns relevant snippet.

---

## Phase 4 — Server-Side Entity UUID Filtering

> Depends on Shrike Recommendation A being merged. Until then the client-side fallback in `cmd/api/main.go` remains.

### 4.1 Update `shrikeSearcher.Search` in `cmd/api/main.go`

Once shrike's `SearchFilter.entity_uuids` is available:

```go
resp, err := s.client.Search(ctx, connect.NewRequest(&shrikev1.SearchRequest{
    Query: query,
    Limit: limit,
    Mode:  "hybrid",
    Filter: &shrikev1.SearchFilter{
        EntityUuids: resourceUUIDs, // server-side Qdrant filter
    },
}))
// Remove the client-side uuidSet filtering loop
```

Until then, keep the existing client-side filter but bump the limit to `limit * 3` to reduce miss probability when resources are sparse in results.

**Files:** `cmd/api/main.go` — update `shrikeSearcher.Search` (~15 lines)

---

## Phase 5 — Wire ResourceCache into Chat

> `RedisResourceCache` in `lib/repo/cache/resource_cache.go` is complete but never injected. This avoids redundant shrike calls within a conversation turn.

### 5.1 Add cache field to `conversationService`

In `lib/greyseal/conversation/service.go`, add optional `cache ResourceCache` (interface already defined in `interface.go`). Update `NewConversationService` to accept it (nil-safe).

### 5.2 Update `Chat` to use cache

```go
// Try cache first
if srv.cache != nil {
    if cached, err := srv.cache.List(ctx, conversationUUID); err == nil && len(cached) > 0 {
        // use cached resources as context
    }
}
// Fall through to shrike on cache miss
results, err := srv.searcher.Search(...)
if err == nil && len(results) > 0 {
    if srv.cache != nil {
        srv.cache.Merge(ctx, conversationUUID, toCachedResources(results))
    }
}
```

### 5.3 Wire in `cmd/api/main.go`

```go
var resourceCache conversation.ResourceCache
if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
    rdb := redis.NewClient(&redis.Options{Addr: redisURL})
    resourceCache = &cache.RedisResourceCache{Client: rdb}
}
convSvc := conversationsvc.NewConversationService(
    convRepo, messageRepo, searcher, roleRepo, ollamaLLM, resourceCache, logger,
)
```

Also add `redis` service to `docker-compose.yml` if not already present.

**Files:** `lib/greyseal/conversation/service.go` (modify), `cmd/api/main.go` (modify), `docker-compose.yml` (add redis if needed)

---

## Phase 6 — Source Attribution in Context Message

> `SearchResult.Title` and `EntityUUID` are available but discarded. The LLM can't cite sources.

### 6.1 Update context assembly in `lib/greyseal/conversation/service.go`

Change:
```
Here is relevant context:
1. snippet text
```
To:
```
Here is relevant context:
1. [Resource Name] snippet text
2. [Resource Name] snippet text
```

Using `r.Title` from `SearchResult`. No schema changes.

**Files:** `lib/greyseal/conversation/service.go` — ~5 lines in `Chat`

---

## Phase 7 — Conversation Summary for Long Histories

> `conversations.summary` exists but is never generated or used. Messages beyond 10 are silently dropped.

### 7.1 Add summary generation to `Chat`

When `len(history) > 10`, call the LLM to summarise the oldest overflow messages before truncating:

```go
if len(history) > 10 {
    overflow := history[:len(history)-10]
    summary := srv.summarize(ctx, overflow) // non-streaming LLM call
    conv.Summary = summary
    srv.conversationRepo.Update(ctx, conv.Uuid, conv)
    history = history[len(history)-10:]
}
```

If `conv.Summary != ""` on entry, prepend it as a system message before the history window:

```go
if conv.Summary != "" {
    llmMessages = append(llmMessages, LLMMessage{
        Role:    "system",
        Content: "Summary of earlier conversation: " + conv.Summary,
    })
}
```

**Files:** `lib/greyseal/conversation/service.go` — add `summarize` helper + logic in `Chat`

---

## File Reference

| File | Action |
|---|---|
| `lib/greyseal/resource/interface.go` | **Create** |
| `lib/greyseal/resource/service.go` | **Create** |
| `lib/greyseal/resource/grpc/service.go` | **Create** |
| `lib/greyseal/resource/kafka_indexer.go` | **Create** — publishes `TextExtractedEvent` or enqueues pending |
| `lib/greyseal/resource/fetcher.go` | **Create** — HTTP scrape + PDF stub |
| `lib/greyseal/conversation/service.go` | **Modify** — cache, attribution, summary |
| `cmd/api/main.go` | **Modify** — register resource handler, wire indexer + cache |
| `cmd/worker/main.go` | **Rewrite** — consume pending resources, fetch, publish `TextExtractedEvent` |
| `lib/repo/cache/resource_cache.go` | No change — already complete |
| `lib/repo/ollama/embedder.go` | **Not needed** — shrike owns embedding |
| `lib/repo/qdrant/client.go` | **Not needed** — shrike owns Qdrant |

---

## Verification Plan

1. **`make test`** stays green throughout all phases.
2. `grey-seal ingest --name test --text "the capital of France is Paris"` returns a UUID.
3. Shrike `IndexRecord` table has a row for the UUID; Qdrant has vector points.
4. `Chat` with "What is the capital of France?" includes "Paris" in context system message.
5. Create two conversations with different `resource_uuids`. Confirm each only surfaces its own resource content (Phase 4 gates full correctness; Phase 1–2 gates partial correctness with client-side filter).
6. Second chat turn in same conversation hits Redis cache (Phase 5).
7. `grey-seal ingest --url https://example.com` → worker log shows fetch + publish → shrike indexes.

---

## What grey-seal Does NOT Own

| Concern | Owner |
|---|---|
| Text chunking | shrike (`lib/chunker`, 512/64 words) |
| Text embedding | shrike (Ollama `nomic-embed-text`, 768d) |
| Qdrant storage & search | shrike |
| Keyword full-text search | shrike (Postgres tsvector GIN) |
| PDF content extraction (full) | Out of scope for MVP |
