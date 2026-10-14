package conversation

import (
	"context"
	"time"

	"github.com/holmes89/archaea/base"
	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
)

type ConversationService interface {
	List(ctx context.Context, lis base.ListRequest) (base.ListResponse[*greysealv1.Conversation], error)
	Get(ctx context.Context, get base.GetRequest[*greysealv1.Conversation]) (base.GetResponse[*greysealv1.Conversation], error)
	Create(ctx context.Context, data *greysealv1.Conversation) (*greysealv1.Conversation, error)
	Update(ctx context.Context, id string, data *greysealv1.Conversation) (*greysealv1.Conversation, error)
	Delete(ctx context.Context, id string) error

	// Chat sends a user message and streams back the assistant response token by token.
	// The stream callback is invoked once per token; returning an error aborts streaming.
	// The fully-populated assistant Message is returned when streaming completes.
	Chat(ctx context.Context, conversationUUID string, content string, stream func(token string) error) (*greysealv1.Message, error)

	// Regenerate discards the conversation's latest draft and drafts again
	// from the same input (Retry). It errors when the conversation's work
	// type does not produce drafts.
	Regenerate(ctx context.Context, conversationUUID string, stream func(token string) error) (*greysealv1.Message, error)

	// KeepDraft writes a draft message to Remora (design, discovery doc or
	// tickets) and records where it went on the message. It returns
	// ErrKeepUnavailable when Remora is not configured and ErrKeepInvalid for
	// caller mistakes.
	KeepDraft(ctx context.Context, in KeepDraftInput) (*greysealv1.Message, error)

	// SubmitFeedback records user feedback (-1/0/1) on an assistant message.
	SubmitFeedback(ctx context.Context, messageUUID string, feedback int32) error
}

type MessageRepository interface {
	Create(context.Context, *greysealv1.Message) error
	Update(context.Context, string, *greysealv1.Message) error
	Delete(context.Context, string) error
	Get(context.Context, string) (*greysealv1.Message, error)
	List(context.Context, string, uint, map[string][]any) ([]*greysealv1.Message, error)
	ListByConversation(ctx context.Context, conversationUUID string) ([]*greysealv1.Message, error)
	UpdateFeedback(ctx context.Context, messageUUID string, feedback int32) error
}

var _ base.Entity = (*greysealv1.Message)(nil)
var _ base.Repository[*greysealv1.Message] = (MessageRepository)(nil)

type ConversationRepository interface {
	Create(context.Context, *greysealv1.Conversation) error
	Update(context.Context, string, *greysealv1.Conversation) error
	Delete(context.Context, string) error
	Get(context.Context, string) (*greysealv1.Conversation, error)
	List(context.Context, string, uint, map[string][]any) ([]*greysealv1.Conversation, error)
}

var _ base.Entity = (*greysealv1.Conversation)(nil)
var _ base.Repository[*greysealv1.Conversation] = (ConversationRepository)(nil)

// SearchResult holds a single search result from shrike.
type SearchResult struct {
	EntityUUID string
	Title      string
	Snippet    string
	Score      float32
}

// Searcher retrieves relevant results from the search service (shrike).
// resourceUUIDs restricts results to those entities; if empty all indexed content is searched.
type Searcher interface {
	Search(ctx context.Context, query string, limit int32, resourceUUIDs []string) ([]SearchResult, error)
}

// CachedResource is a resource snippet stored in the cache for a conversation.
type CachedResource struct {
	EntityUUID string
	Title      string
	Snippet    string
	Score      float32
}

// ResourceCache persists per-conversation resource context between requests.
type ResourceCache interface {
	Merge(ctx context.Context, conversationUUID string, resources []CachedResource) error
	List(ctx context.Context, conversationUUID string) ([]CachedResource, error)
}

// TranscriptTurn captures the full context of one user→assistant exchange.
type TranscriptTurn struct {
	ConversationUUID    string
	TurnIndex           int
	Timestamp           time.Time
	UserMessage         string
	SystemPrompt        string
	ConversationSummary string
	HistoryDepth        int
	SearchQuery         string
	SearchResults       []SearchResult
	AssembledMessages   []LLMMessage
	Response            string
	ResourceUUIDs       []string
}

// TranscriptWriter persists a TranscriptTurn for offline review.
type TranscriptWriter interface {
	WriteTurn(ctx context.Context, turn TranscriptTurn) error
}

// ChainOffer is a pending proposal to move onto the next work type after a
// draft is kept: the work type's id, and the kept draft's content, which
// seeds the next draft.
type ChainOffer struct {
	WorkType string
	Source   string
}

// DraftStore holds draft bodies and chain offers that have not been acted on
// yet. Everything in it expires: an unkept draft is scratch work, and once
// kept its body is saved on the (permanent) message instead. Implementations
// live in lib/repo/draftstore.
type DraftStore interface {
	// PutDraft stores a draft message's body under the message uuid.
	PutDraft(ctx context.Context, messageUUID, body string) error
	// GetDraft returns the body, ok=false when it never existed or expired.
	GetDraft(ctx context.Context, messageUUID string) (body string, ok bool, err error)
	DeleteDraft(ctx context.Context, messageUUID string) error

	// PutOffer records the conversation's pending chain offer, replacing any.
	PutOffer(ctx context.Context, conversationUUID string, offer ChainOffer) error
	// GetOffer returns the pending offer, ok=false when there is none.
	GetOffer(ctx context.Context, conversationUUID string) (offer ChainOffer, ok bool, err error)
	DeleteOffer(ctx context.Context, conversationUUID string) error
}
