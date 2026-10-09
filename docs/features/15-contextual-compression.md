---
uuid: "e260fd0f-82b2-5d9d-8447-751f89ba0b4a"
kind: "feature"
product_uuid: "aa14cb8f-ba36-5afe-abc9-40d26b422f89"
project_uuid: "9f94971e-2224-5948-bcd4-eed08b50d911"
design_uuid: "4e11da25-604c-5135-92cd-4e0f2fe85068"
portal_url: "https://joel.holmes.haus/discovery/e260fd0f-82b2-5d9d-8447-751f89ba0b4a"
design_url: "https://joel.holmes.haus/designs/4e11da25-604c-5135-92cd-4e0f2fe85068"
file: "docs/features/15-contextual-compression.md"
body_sha256: "434db64f9b3764a57c78464cdb81dd91e3fd07c6ef24d985a45aeca7252f4a50"
product: joel.holmes.haus
type: feature
status: draft
source: "narwhal-catalog:joel.holmes.haus/features/15-contextual-compression.md"
parent: "https://github.com/holmes89/narwhal/blob/main/designs/joel.holmes.haus/system.md"
---
# Feature: Contextual Compression & Re-ranking in the RAG Pipeline

> The retrieval layer already finds the right sources. This feature ensures only the right *parts* of those sources reach the LLM.

## Overview

The current grey-seal RAG pipeline passes full 512-token Shrike snippets verbatim into the LLM context window. At five snippets per turn, that can consume ~2,500 tokens of context budget on chunks that may contain only one or two sentences genuinely relevant to the user's query.

A **contextual compressor retriever** adds a post-retrieval step that:

1. Filters out chunks with a relevance score below a configurable threshold.
2. Extracts only the sentence(s) from each retained chunk that are most relevant to the query.
3. Optionally re-ranks the compressed results before they enter the prompt.

The pattern is well-established in LangChain but doesn't require LangChain — the logic belongs naturally in grey-seal's `contextSearch` path, between the Shrike call and prompt assembly.

## Background

### Current retrieval flow

```
User query
    │
    ▼
grey-seal contextSearch()
    │
    ├── Redis cache hit → return cached snippets (stale, full-chunk)
    │
    └── Cache miss → Shrike.Search(query, limit=5, mode="hybrid")
                         │
                         ▼
                    5 × 512-token chunks (with scores)
                         │
                         ▼
                    Deduplicate by entity (keep highest score)
                    Sort descending by score
                         │
                         ▼
                    All chunks → system message verbatim
```

### What Shrike provides

- **Score** — cosine similarity (semantic) or `ts_rank` (keyword), depending on which path succeeded.
- **Snippet** — a 512-word chunk with 64-word overlap from the source text.
- **Entity UUID + Title** — used for attribution.

### Gaps

| Gap | Impact |
|---|---|
| `min_score` defined in `SearchRequest` proto but ignored in shrike handler | No threshold filtering; low-quality chunks reach the LLM |
| Full chunks injected verbatim | Context window wasteful; irrelevant sentences dilute the answer |
| No re-ranking | Hybrid score is a proxy for relevance, not a cross-encoder score |
| Cache returns stale snippets on subsequent turns | Later turns never benefit from better chunks on the same resource |

## Design

### Architecture

```
User query
    │
    ▼
grey-seal contextSearch()
    │
    ├── Redis cache hit
    │       │
    │       └── compressor.Extract(query, cachedSnippets)  ← NEW
    │
    └── Cache miss → Shrike.Search(query, limit=8, min_score=0.3, mode="hybrid")
                         │
                         ▼
                    Raw chunks (scored)
                         │
                         ▼
                    [1] Score threshold filter (≥ min_score)  ← NEW
                         │
                         ▼
                    [2] Extractive compression per chunk       ← NEW
                         │
                         ▼
                    [3] Deduplicate by entity (keep best)
                    [4] Sort descending by score
                    [5] Cache compressed results (Redis)
                         │
                         ▼
                    Compressed context → system message
```

### Components

#### 1. Score threshold (Shrike — minimal change)

Wire up the existing `min_score` field in the `SearchRequest` handler so Shrike filters results server-side before returning them. Grey-seal passes `min_score: 0.3` (configurable via `SHRIKE_MIN_SCORE` env var).

This is a one-line fix in shrike's `SearchService` handler — the field already exists in the proto.

#### 2. Extractive compressor (grey-seal)

A new `Compressor` interface in `lib/greyseal/conversation/interface.go`:

```go
type Compressor interface {
    // Extract returns the most relevant portion of snippet for query.
    // If nothing is relevant, returns ("", false).
    Extract(ctx context.Context, query, snippet string) (string, bool)
}
```

Two implementations, selectable by config:

**a) Embedding similarity (default, no extra LLM calls)**

Split each snippet into sentences. Embed each sentence and the query using the same model Shrike uses (Ollama `nomic-embed-text`). Keep sentences whose cosine similarity to the query exceeds a threshold (e.g. 0.6). Concatenate retained sentences.

```
lib/repo/compressor/embedding_compressor.go
```

**b) LLM-based compressor (higher quality, slower)**

Single Ollama call per chunk:

```
System: You extract relevant text. Given a query and a passage, copy verbatim only the sentences from the passage that directly answer or inform the query. If none are relevant, respond with exactly: [NONE].
User:   Query: {query}\n\nPassage: {snippet}
```

```
lib/repo/compressor/llm_compressor.go
```

Config: `GREY_SEAL_COMPRESSOR=embedding|llm|none` (default: `embedding`).

#### 3. Re-ranker (optional, phase 2)

After compression, a cross-encoder re-ranking step scores `(query, compressed_snippet)` pairs more precisely than the initial retrieval score. This can be a small local model (e.g. `ms-marco-MiniLM`) running via Ollama or a lightweight HTTP service.

Deferred — requires an additional model deployment. Tracked as a follow-on.

### Updated `contextSearch` signature

```go
func (srv *conversationService) contextSearch(
    ctx context.Context,
    conversationUUID, query string,
    resourceUUIDs []string,
) []SearchResult {
    // 1. Check cache
    if cached, err := srv.cache.List(ctx, conversationUUID); err == nil && len(cached) > 0 {
        return srv.compress(ctx, query, cached)  // compress even cached results
    }

    // 2. Fetch from Shrike (with min_score)
    raw := srv.searcher.Search(ctx, query, 8, resourceUUIDs)

    // 3. Compress each chunk
    compressed := srv.compress(ctx, query, raw)

    // 4. Deduplicate, sort, cache
    if srv.cache != nil {
        _ = srv.cache.Set(ctx, conversationUUID, compressed)
    }
    return compressed
}
```

### Prompt impact

Current context block (5 chunks × ~512 tokens ≈ 2,560 tokens):

```
1. [Book: "Thinking, Fast and Slow"]: <full 512-word chunk>
2. [Web: "LessWrong — Biases"]:       <full 512-word chunk>
...
```

After compression (5 chunks × ~60 tokens ≈ 300 tokens):

```
1. [Book: "Thinking, Fast and Slow"]: "System 1 thinking is fast, automatic, and prone to cognitive biases such as anchoring."
2. [Web: "LessWrong — Biases"]:       "Anchoring is the tendency to over-weight the first piece of information encountered."
...
```

Freeing ~2,200 tokens per turn — space that can go to a longer conversation history window (10 → 20 turns) or a richer system prompt.

## Implementation Plan

| Step | Deliverable | Owner | Status |
|---|---|---|---|
| 1 | Wire `min_score` in shrike `SearchService` handler | shrike | Pending |
| 2 | Pass `min_score: 0.3` (env-configurable) from grey-seal shrike adapter | grey-seal | Pending |
| 3 | Add `Compressor` interface to grey-seal `conversation/interface.go` | grey-seal | Pending |
| 4 | Implement `EmbeddingCompressor` (sentence split + embed + threshold) | grey-seal | Pending |
| 5 | Implement `LLMCompressor` (single Ollama call per chunk) | grey-seal | Pending |
| 6 | Wire `Compressor` into `contextSearch`; add `GREY_SEAL_COMPRESSOR` env var | grey-seal | Pending |
| 7 | Update Redis cache to store compressed snippets (same schema, shorter text) | grey-seal | Pending |
| 8 | Extend cache: call `compress()` on cache hits so stale full-chunks are also compressed | grey-seal | Pending |
| 9 | [Phase 2] Cross-encoder re-ranker service + `Reranker` interface | grey-seal + new service | Deferred |

## Assumptions

- Ollama is already running and accessible from grey-seal (true in current setup).
- `nomic-embed-text` is available in Ollama (already used by Shrike for indexing — same model ensures embedding space consistency).
- Compression adds < 200ms p95 latency per turn for the embedding path.
- The LLM compressor path is opt-in only; default is embedding-based.

## Constraints

- Must not break the existing `Searcher` interface in grey-seal — the shrike adapter changes are internal.
- Compressed snippets must preserve enough text for the LLM to cite and the UI to display a meaningful excerpt.
- `min_score` change in Shrike must be backward-compatible — a zero value means no threshold (existing behaviour).

## Out of Scope

- Query expansion / query rewriting before the Shrike call.
- Multi-hop retrieval (retrieve → answer → retrieve again).
- Cross-encoder re-ranking (Phase 2).
- Changes to Shrike's chunking strategy (separate concern).

## Open Questions

| # | Question | Status |
|---|---|---|
| 1 | What sentence splitter to use? (stdlib `strings.Split` on `.`/`!`/`?` vs. a proper sentence boundary detector) | Open |
| 2 | Should compressed snippets replace or augment the cached entry? (replacing is simpler; augmenting preserves the original for other uses) | Open — start with replace |
| 3 | Embedding the query on every turn adds one Ollama call — acceptable latency? | Open — measure in testing |
| 4 | Should `min_score` be per-conversation configurable (e.g., relaxed for exploratory chats)? | Defer — global env var for now |

## Dependencies

- Shrike `SearchService` — `min_score` fix (Step 1)
- Ollama `nomic-embed-text` model available at `OLLAMA_HOST`
- Existing Redis cache (`lib/repo/cache/resource_cache.go`)
- Existing `Searcher` interface (`lib/greyseal/conversation/interface.go`)

## Success Metrics

- Average context block size reduced by ≥ 60% (tokens) vs. baseline.
- LLM answer relevance maintained or improved (qualitative review).
- p95 Chat latency increase < 300ms for the embedding compressor path.
- No regression in source attribution (title labels still present on compressed snippets).
