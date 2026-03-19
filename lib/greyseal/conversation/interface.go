package conversation

import (
	"context"

	"github.com/holmes89/archaea/base"
	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	. "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
)

type ConversationService interface {
	List(ctx context.Context, lis base.ListRequest) (base.ListResponse[*Conversation], error)
	Get(ctx context.Context, get base.GetRequest[*Conversation]) (base.GetResponse[*Conversation], error)
	Create(ctx context.Context, data *Conversation) (*Conversation, error)
	Update(ctx context.Context, id string, data *Conversation) (*Conversation, error)
	Delete(ctx context.Context, id string) error

	// Chat sends a user message and streams back the assistant response token by token.
	// The stream callback is invoked once per token; returning an error aborts streaming.
	// The fully-populated assistant Message is returned when streaming completes.
	Chat(ctx context.Context, conversationUUID string, content string, stream func(token string) error) (*Message, error)

	// SubmitFeedback records user feedback (-1/0/1) on an assistant message.
	SubmitFeedback(ctx context.Context, messageUUID string, feedback int32) error
}

type MessageRepository interface {
	Create(context.Context, *Message) error
	Update(context.Context, string, *Message) error
	Delete(context.Context, string) error
	Get(context.Context, string) (*Message, error)
	List(context.Context, string, uint, map[string][]any) ([]*Message, error)
	ListByConversation(ctx context.Context, conversationUUID string) ([]*Message, error)
	UpdateFeedback(ctx context.Context, messageUUID string, feedback int32) error
}

var _ base.Entity = (*Message)(nil)
var _ base.Repository[*Message] = (MessageRepository)(nil)

type ConversationRepository interface {
	Create(context.Context, *Conversation) error
	Update(context.Context, string, *Conversation) error
	Delete(context.Context, string) error
	Get(context.Context, string) (*Conversation, error)
	List(context.Context, string, uint, map[string][]any) ([]*Conversation, error)
}

var _ base.Entity = (*Conversation)(nil)
var _ base.Repository[*Conversation] = (ConversationRepository)(nil)

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
