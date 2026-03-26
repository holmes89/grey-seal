package conversation

import (
	"context"

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

// RoleRepository fetches role data by UUID.
type RoleRepository interface {
	Get(ctx context.Context, id string) (*greysealv1.Role, error)
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
