---
uuid: "c7850c17-a0ae-54f8-9de8-c80f8c15142e"
kind: "feature"
product_uuid: "aa14cb8f-ba36-5afe-abc9-40d26b422f89"
project_uuid: "9f94971e-2224-5948-bcd4-eed08b50d911"
design_uuid: "9ecb519f-1192-52d5-a317-1591bf0196ac"
portal_url: "https://joel.holmes.haus/discovery/c7850c17-a0ae-54f8-9de8-c80f8c15142e"
design_url: "https://joel.holmes.haus/designs/9ecb519f-1192-52d5-a317-1591bf0196ac"
file: "docs/features/19-grey-seal-transcripts.md"
body_sha256: "2eba94f2d4a9e72d586e559e4515246820bb3f286e614bb6d20adff6fb3598af"
product: joel.holmes.haus
type: feature
status: draft
source: "narwhal-catalog:joel.holmes.haus/features/19-grey-seal-transcripts.md"
parent: "https://github.com/holmes89/narwhal/blob/main/designs/joel.holmes.haus/system.md"
---
# Feature: Grey-seal — Conversation Transcripts

## Overview

Transcripts is a debug-and-tuning facility for grey-seal. For each conversation,
it produces an append-only, human-readable log of every `Chat` turn: the exact
prompt assembled and fed to the LLM, the Shrike search query and results used to
build context, the conversation history included, and the raw LLM response. The
log lives in a configurable `TRANSCRIPT_DIR` directory, one file per conversation
UUID, and is never shown to the user — it is a developer artifact.

The goal is to close the feedback loop when tuning RAG quality: instead of
adding ad-hoc `logger.Info` calls and grepping logs, a developer can open
`transcripts/{conversation-uuid}.md` and read exactly what the model saw.

## Problem Statement

Grey-seal's `Chat` path has several moving parts that are hard to observe in
production:

- The assembled prompt is never logged (it would flood structured logs)
- Shrike result quality and scores are logged at INFO but scattered across
  multiple log lines
- Whether the role system prompt was applied is not visible after the fact
- Whether the conversation summary was prepended is not visible
- The raw LLM response (before it is saved to Postgres) is not persisted anywhere
- The `contextSearch` function is currently disabled (cache keying bug), so it
  is unclear what Shrike is actually returning turn-by-turn

None of the above is diagnosable without attaching a debugger or adding temporary
print statements. When trying to tune prompt templates, context window sizing, or
Shrike search parameters, the current observability gap means changes must be
verified by feel.

## Goals

- Write one append-only transcript file per conversation under
  `{TRANSCRIPT_DIR}/{conversation-uuid}.md`
- Capture per-turn: user message, system prompt, conversation summary,
  Shrike search query + results, full assembled LLM messages, raw LLM response,
  and resource UUIDs cited
- Be a completely optional, zero-overhead code path: a `nil` `TranscriptWriter`
  means no files are created and no allocations occur in the hot path
- Follow the existing optional-dependency pattern: activated by a single
  `TRANSCRIPT_DIR` env var, injected into `conversationService` alongside other
  optional deps
- Never write transcripts to the database or expose them via the gRPC API

## Non-Goals

- Structured machine-readable format (JSON/JSONL); markdown is sufficient for
  the intended debugging use case
- Compression or rotation of transcript files
- A UI for browsing transcripts
- Transcript data being used at runtime (e.g., no feedback loop back into the
  service)
- Capturing the streaming token sequence (only the final assembled response is
  recorded)

## Proposed Solution

### Interface

A new `TranscriptWriter` interface is added to
`lib/greyseal/conversation/interface.go` alongside the existing optional
interfaces (`Searcher`, `ResourceCache`):

```go
// TranscriptTurn holds everything that happened in a single Chat turn.
// All fields are best-effort; zero values are omitted from the written output.
type TranscriptTurn struct {
    ConversationUUID    string
    TurnIndex           int       // 1-based, derived from message count before this turn
    Timestamp           time.Time

    // Input
    UserMessage         string

    // Prompt assembly
    SystemPrompt        string    // final system prompt used (role override or default)
    ConversationSummary string    // summary prepended as a system message, if any
    HistoryDepth        int       // number of history messages included

    // Shrike context retrieval
    SearchQuery         string    // query passed to Shrike
    SearchResults       []SearchResult // results returned (title, score, snippet)

    // Full prompt fed to LLM (ordered slice of role+content pairs)
    AssembledMessages   []LLMMessage

    // Output
    Response            string    // raw LLM response text
    ResourceUUIDs       []string  // entity UUIDs cited in the assistant message
}

// TranscriptWriter appends a turn record to the conversation's transcript.
// Implementations must be safe for concurrent calls on different conversation UUIDs.
type TranscriptWriter interface {
    WriteTurn(ctx context.Context, turn TranscriptTurn) error
}
```

### File-Based Implementation

Path: `lib/repo/transcript/writer.go`

```go
type FileTranscriptWriter struct {
    dir string // TRANSCRIPT_DIR
}

func NewFileTranscriptWriter(dir string) (*FileTranscriptWriter, error) {
    if err := os.MkdirAll(dir, 0o700); err != nil {
        return nil, fmt.Errorf("transcript: create dir: %w", err)
    }
    return &FileTranscriptWriter{dir: dir}, nil
}
```

Each `WriteTurn` call:
1. Opens (or creates) `{dir}/{conversation-uuid}.md` in append mode
2. Writes a new `## Turn N — {RFC3339 timestamp}` section
3. Closes the file

The file is opened and closed per write — no file handles are held across calls.
Concurrent writes to different conversations are safe (different files). Concurrent
writes to the same conversation (two requests interleaving) are made safe by
`os.OpenFile` with `O_APPEND`, which is atomic on POSIX for writes under the
filesystem buffer size. If this proves insufficient a per-conversation `sync.Mutex`
can be added; for a single-user debugging tool it is not necessary.

### Transcript Format

Each turn is rendered as a markdown section appended to the file:

```markdown
## Turn 3 — 2026-04-16T14:32:01Z

**User message**
What are the main arguments in Chapter 4?

---

**System prompt**
You are grey-seal, a personal knowledge assistant...

---

**Conversation summary** _(prepended as context)_
Earlier the user asked about chapters 1 and 2. The assistant summarised...

---

**Shrike search** · query: `"main arguments chapter 4"` · 3 results

| # | Title | Score | Snippet |
|---|-------|-------|---------|
| 1 | Deep Work — Chapter 4 | 0.91 | "Newport argues that the ability to perform..." |
| 2 | Deep Work — Chapter 3 | 0.74 | "The previous chapter established the value..." |
| 3 | Deep Work — Introduction | 0.61 | "This book has two goals, each corresponding..." |

---

**Assembled prompt** _(4 messages, history depth: 4)_

```system
You are grey-seal, a personal knowledge assistant...
```

```system
Here is relevant context:
1. [Deep Work — Chapter 4]: Newport argues that...
2. [Deep Work — Chapter 3]: The previous chapter...
```

```user
[earlier turn 1 content]
```

```assistant
[earlier turn 1 response]
```

```user
What are the main arguments in Chapter 4?
```

---

**Response**
Newport's core argument in Chapter 4 is that...

**Resources cited:** `d1e2f3a4`, `b5c6d7e8`

---
```

The prompt messages section uses fenced code blocks with the role as the language
label — parseable if needed later, readable immediately.

### Integration into `conversationService`

`TranscriptWriter` is added as an optional field following the existing pattern:

```go
type conversationService struct {
    // ... existing fields ...
    transcriptWriter TranscriptWriter // optional; nil = no-op
}

func NewConversationService(
    // ... existing params ...
    transcriptWriter TranscriptWriter,
    logger *zap.Logger,
) ConversationService {
    return &conversationService{
        // ... existing assignments ...
        transcriptWriter: transcriptWriter,
    }
}
```

Inside `Chat`, a `TranscriptTurn` is assembled incrementally as the turn
progresses. The write call happens after the assistant message is saved to
Postgres — a failure to write the transcript logs a warning but does not
fail the request:

```go
// At the end of Chat, after saving the assistant message:
if srv.transcriptWriter != nil {
    turn := TranscriptTurn{
        ConversationUUID:    conversationUUID,
        TurnIndex:           len(history) + 1,
        Timestamp:           time.Now(),
        UserMessage:         content,
        SystemPrompt:        systemPromptText,   // captured earlier
        ConversationSummary: summaryText,         // captured earlier
        HistoryDepth:        len(history),
        SearchQuery:         content,             // current: query == user message
        SearchResults:       contextSnippets,
        AssembledMessages:   llmMessages,
        Response:            responseContent,
        ResourceUUIDs:       usedResourceUUIDs,
    }
    if err := srv.transcriptWriter.WriteTurn(ctx, turn); err != nil {
        srv.logger.Warn("failed to write transcript",
            zap.String("conversation_uuid", conversationUUID),
            zap.Error(err),
        )
    }
}
```

A few intermediate variables need to be promoted out of their current inner
scopes in `Chat` to be readable at the end of the function (`systemPromptText`,
`summaryText`). These are small, local refactors with no behaviour change.

### `cmd/api/main.go` Wiring

```go
// Transcript writer (optional; requires TRANSCRIPT_DIR)
var transcriptWriter conversationsvc.TranscriptWriter
if dir := os.Getenv("TRANSCRIPT_DIR"); dir != "" {
    tw, err := transcript.NewFileTranscriptWriter(dir)
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
    transcriptWriter, // new
    logger,
)
```

### File Layout

```
lib/
  greyseal/
    conversation/
      interface.go     ← add TranscriptWriter interface + TranscriptTurn
      service.go       ← inject + call TranscriptWriter at end of Chat
  repo/
    transcript/
      writer.go        ← FileTranscriptWriter implementation
      writer_test.go   ← unit tests (temp dir, append behaviour)
cmd/
  api/
    main.go            ← wire TRANSCRIPT_DIR → FileTranscriptWriter
```

### Example Directory Structure at Runtime

```
/var/grey-seal/transcripts/
  3f2a1b4c-…-conversation-uuid-1.md
  7e8d9f0a-…-conversation-uuid-2.md
```

Each file grows by one section per `Chat` turn. A conversation with 20 turns
produces roughly 80–120 KB depending on snippet length and context window usage.

## User Stories

- As a developer, I want to open a transcript file and see exactly what the LLM
  was fed so that I can understand why it produced a given response
- As a developer, I want to see Shrike's search results for each turn with scores
  so that I can tune retrieval parameters without adding temporary log statements
- As a developer, I want transcripts to be off by default so that production
  deployments are unaffected
- As a developer, I want the transcript to be plain readable text so that I can
  diff two conversations side-by-side in a terminal

## Open Questions

- [ ] Should `TRANSCRIPT_DIR` accept `stdout` as a special value for local dev
  (writes all turns to stdout instead of files)?
- [ ] Should the search query always equal `content` (the user message), or should
  grey-seal allow a separate query-rewriting step in future? If so, the
  `SearchQuery` field is already the right place to capture it.
- [ ] Should transcript files be retained indefinitely, or should there be a
  `TRANSCRIPT_MAX_AGE` env var that prunes files older than N days?
- [ ] Is per-character content in assembled messages too verbose for long
  conversations? Consider a `--truncate-snippets` mode that caps each snippet
  at 200 chars in the transcript while preserving the full text in the prompt.

## Success Metrics

- A transcript file is created for every conversation that has at least one
  `Chat` call when `TRANSCRIPT_DIR` is set
- The assembled prompt in the transcript exactly matches what was passed to
  `llm.Chat` (verified by test)
- A failed transcript write does not affect the `Chat` response or gRPC status
- No performance regression on `Chat` latency (p95) when `TRANSCRIPT_DIR` is unset

## Dependencies

- [08-grey-seal-rag.md](08-grey-seal-rag.md) — `ConversationService`, `Chat`
  implementation, `LLMMessage` type
- No new external dependencies; uses only `os`, `fmt`, `strings` from stdlib

## Milestones

- Add `TranscriptTurn` and `TranscriptWriter` to `lib/greyseal/conversation/interface.go`
- Implement `FileTranscriptWriter` in `lib/repo/transcript/writer.go` with unit tests
- Refactor `Chat` to promote `systemPromptText` and `summaryText` to function scope;
  assemble and write `TranscriptTurn` at end of function
- Wire `TRANSCRIPT_DIR` in `cmd/api/main.go`
- Smoke test: run a single-turn conversation locally, verify transcript file is
  created with all sections populated
