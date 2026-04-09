package conversation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/holmes89/archaea/base"
	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ ConversationService = (*conversationService)(nil)

// LLM streams an assistant response given a list of chat messages.
// Each token is passed to the stream callback; the full response is returned.
type LLM interface {
	Chat(ctx context.Context, messages []LLMMessage, stream func(token string) error) (string, error)
}

// LLMMessage is a single message in the LLM chat format.
type LLMMessage struct {
	Role    string // "system", "user", "assistant"
	Content string
}

type conversationService struct {
	conversationRepo base.Repository[*greysealv1.Conversation]
	messageRepo      MessageRepository
	searcher         Searcher       // optional
	roleRepo         RoleRepository // optional
	llm              LLM            // optional
	cache            ResourceCache  // optional; disables per-conversation snippet caching when nil
	logger           *zap.Logger
}

func NewConversationService(
	conversationRepo base.Repository[*greysealv1.Conversation],
	messageRepo MessageRepository,
	searcher Searcher,
	roleRepo RoleRepository,
	llm LLM,
	cache ResourceCache,
	logger *zap.Logger,
) ConversationService {
	return &conversationService{
		conversationRepo: conversationRepo,
		messageRepo:      messageRepo,
		searcher:         searcher,
		roleRepo:         roleRepo,
		llm:              llm,
		cache:            cache,
		logger:           logger,
	}
}

func (srv *conversationService) List(ctx context.Context, lis base.ListRequest) (base.ListResponse[*greysealv1.Conversation], error) {
	srv.logger.Info("listing conversations")
	data, err := srv.conversationRepo.List(ctx, lis.GetCursor(), uint(lis.GetCount()), nil)
	if err != nil {
		srv.logger.Error("failed to list conversations", zap.Error(err))
	}
	return &base.ListGenericResponse[*greysealv1.Conversation]{
		Cursor: "",
		Count:  int32(len(data)),
		Data:   data,
	}, err
}

func (srv *conversationService) Get(ctx context.Context, get base.GetRequest[*greysealv1.Conversation]) (base.GetResponse[*greysealv1.Conversation], error) {
	srv.logger.Info("getting conversation", zap.String("uuid", get.GetUuid()))
	data, err := srv.conversationRepo.Get(ctx, get.GetUuid())
	if err != nil {
		srv.logger.Error("failed to get conversation", zap.String("uuid", get.GetUuid()), zap.Error(err))
	}
	return &base.GetGenericResponse[*greysealv1.Conversation]{Data: data}, err
}

func (srv *conversationService) Create(ctx context.Context, data *greysealv1.Conversation) (*greysealv1.Conversation, error) {
	if data.Uuid == "" {
		data.Uuid = uuid.New().String()
	}
	now := timestamppb.New(time.Now())
	data.CreatedAt = now
	data.UpdatedAt = now

	srv.logger.Info("creating conversation", zap.String("title", data.GetTitle()))
	err := srv.conversationRepo.Create(ctx, data)
	if err != nil {
		srv.logger.Error("failed to create conversation", zap.Error(err))
		return nil, err
	}
	srv.logger.Info("conversation created", zap.String("uuid", data.Uuid))
	return data, nil
}

func (srv *conversationService) Update(ctx context.Context, id string, data *greysealv1.Conversation) (*greysealv1.Conversation, error) {
	srv.logger.Info("updating conversation", zap.String("uuid", id))
	data.UpdatedAt = timestamppb.New(time.Now())
	err := srv.conversationRepo.Update(ctx, id, data)
	if err != nil {
		srv.logger.Error("failed to update conversation", zap.String("uuid", id), zap.Error(err))
		return nil, err
	}
	return data, nil
}

func (srv *conversationService) Delete(ctx context.Context, id string) error {
	srv.logger.Info("deleting conversation", zap.String("uuid", id))
	err := srv.conversationRepo.Delete(ctx, id)
	if err != nil {
		srv.logger.Error("failed to delete conversation", zap.String("uuid", id), zap.Error(err))
	}
	return err
}

func (srv *conversationService) Chat(ctx context.Context, conversationUUID string, content string, stream func(token string) error) (*greysealv1.Message, error) {
	srv.logger.Info("chat request", zap.String("conversation_uuid", conversationUUID))
	// 1. Save user message to DB
	userMsg := &greysealv1.Message{
		Uuid:             uuid.New().String(),
		ConversationUuid: conversationUUID,
		Role:             greysealv1.MessageRole_MESSAGE_ROLE_USER,
		Content:          content,
		CreatedAt:        timestamppb.New(time.Now()),
	}
	if err := srv.messageRepo.Create(ctx, userMsg); err != nil {
		srv.logger.Error("failed to save user message", zap.String("conversation_uuid", conversationUUID), zap.Error(err))
		return nil, fmt.Errorf("failed to save user message: %w", err)
	}

	// 2. Load conversation to get role_uuid and resource_uuids scope
	conv, err := srv.conversationRepo.Get(ctx, conversationUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to load conversation: %w", err)
	}

	// Default system prompt establishes the assistant persona.
	llmMessages := []LLMMessage{
		{
			Role: "system",
			Content: "You are a helpful research assistant. When you use information from the provided context, " +
				"reference it clearly so the user knows which sources informed your answer. " +
				"Be concise, accurate, and cite sources when relevant.",
		},
	}

	// 3. Load role system prompt if role_uuid is set — overrides the default.
	if conv.RoleUuid != "" && srv.roleRepo != nil {
		role, err := srv.roleRepo.Get(ctx, conv.RoleUuid)
		if err == nil && role.SystemPrompt != "" {
			llmMessages = []LLMMessage{{Role: "system", Content: role.SystemPrompt}}
		}
	}

	// 4. Load message history and handle overflow summarisation
	history, err := srv.messageRepo.ListByConversation(ctx, conversationUUID)
	if err != nil {
		history = nil // non-fatal; continue without history
	}
	// Remove the user message we just persisted (it is always last, sorted ASC).
	if len(history) > 0 && history[len(history)-1].Uuid == userMsg.Uuid {
		history = history[:len(history)-1]
	}

	// If history is deeper than 10 messages, summarise the overflow and persist it.
	summaryText := conv.Summary
	if len(history) > 10 {
		overflow := history[:len(history)-10]
		history = history[len(history)-10:]
		if generated := srv.summarizeMessages(ctx, overflow); generated != "" {
			summaryText = generated
			_ = srv.conversationRepo.Update(ctx, conversationUUID, &greysealv1.Conversation{
				Uuid:          conv.Uuid,
				Title:         conv.Title,
				RoleUuid:      conv.RoleUuid,
				ResourceUuids: conv.ResourceUuids,
				Summary:       generated,
				UpdatedAt:     timestamppb.New(time.Now()),
			})
		}
	}

	// Prepend the summary (existing or freshly generated) as a system message.
	if summaryText != "" {
		llmMessages = append(llmMessages, LLMMessage{
			Role:    "system",
			Content: "Summary of earlier conversation: " + summaryText,
		})
	}

	// 5. Retrieve relevant context from shrike (cache-first).
	var usedResourceUUIDs []string
	if contextSnippets := srv.contextSearch(ctx, conversationUUID, content, conv.ResourceUuids); len(contextSnippets) > 0 {
		var parts []string
		for i, r := range contextSnippets {
			parts = append(parts, fmt.Sprintf("%d. [%s]: %s", i+1, r.Title, r.Snippet))
			usedResourceUUIDs = append(usedResourceUUIDs, r.EntityUUID)
		}
		llmMessages = append(llmMessages, LLMMessage{
			Role:    "system",
			Content: "Here is relevant context:\n" + strings.Join(parts, "\n"),
		})
	}

	// 6. Add message history
	for _, msg := range history {
		role := "user"
		if msg.Role == greysealv1.MessageRole_MESSAGE_ROLE_ASSISTANT {
			role = "assistant"
		}
		llmMessages = append(llmMessages, LLMMessage{Role: role, Content: msg.Content})
	}

	// 7. Append current user turn
	llmMessages = append(llmMessages, LLMMessage{Role: "user", Content: content})

	// 8. Call LLM (with streaming) or fall back to placeholder
	var responseContent string
	if srv.llm != nil {
		responseContent, err = srv.llm.Chat(ctx, llmMessages, stream)
		if err != nil {
			srv.logger.Error("LLM chat failed", zap.String("conversation_uuid", conversationUUID), zap.Error(err))
			return nil, fmt.Errorf("LLM chat failed: %w", err)
		}
	} else {
		responseContent = "[LLM response not yet implemented]"
		if err := stream(responseContent); err != nil {
			return nil, err
		}
	}

	// 9. Save assistant message to DB
	assistantMsg := &greysealv1.Message{
		Uuid:             uuid.New().String(),
		ConversationUuid: conversationUUID,
		Role:             greysealv1.MessageRole_MESSAGE_ROLE_ASSISTANT,
		Content:          responseContent,
		ResourceUuids:    usedResourceUUIDs,
		CreatedAt:        timestamppb.New(time.Now()),
	}
	if err := srv.messageRepo.Create(ctx, assistantMsg); err != nil {
		return nil, fmt.Errorf("failed to save assistant message: %w", err)
	}

	// Update conversation updated_at (preserve all existing fields).
	_ = srv.conversationRepo.Update(ctx, conversationUUID, &greysealv1.Conversation{
		Uuid:          conversationUUID,
		Title:         conv.Title,
		RoleUuid:      conv.RoleUuid,
		ResourceUuids: conv.ResourceUuids,
		Summary:       conv.Summary,
		UpdatedAt:     timestamppb.New(time.Now()),
	})

	return assistantMsg, nil
}

// contextSearch retrieves relevant snippets for the given query and resource scope.
// It checks the per-conversation Redis cache first; on a miss it calls shrike and
// populates the cache with the results.
func (srv *conversationService) contextSearch(ctx context.Context, conversationUUID, query string, resourceUUIDs []string) []SearchResult {
	if srv.cache != nil {
		if cached, err := srv.cache.List(ctx, conversationUUID); err == nil && len(cached) > 0 {
			results := make([]SearchResult, len(cached))
			for i, c := range cached {
				results[i] = SearchResult(c)
			}
			return results
		}
	}
	if srv.searcher == nil {
		return nil
	}
	results, err := srv.searcher.Search(ctx, query, 5, resourceUUIDs)
	if err != nil || len(results) == 0 {
		return nil
	}
	if srv.cache != nil {
		cached := make([]CachedResource, len(results))
		for i, r := range results {
			cached[i] = CachedResource(r)
		}
		_ = srv.cache.Merge(ctx, conversationUUID, cached)
	}
	return results
}

// summarizeMessages calls the LLM to produce a concise summary of the given messages.
// Returns an empty string if the LLM is unavailable or returns an error.
func (srv *conversationService) summarizeMessages(ctx context.Context, messages []*greysealv1.Message) string {
	if srv.llm == nil || len(messages) == 0 {
		return ""
	}
	prompt := []LLMMessage{
		{Role: "system", Content: "Summarize the following conversation in a few sentences, preserving key facts and decisions."},
	}
	for _, msg := range messages {
		role := "user"
		if msg.Role == greysealv1.MessageRole_MESSAGE_ROLE_ASSISTANT {
			role = "assistant"
		}
		prompt = append(prompt, LLMMessage{Role: role, Content: msg.Content})
	}
	summary, err := srv.llm.Chat(ctx, prompt, func(_ string) error { return nil })
	if err != nil {
		srv.logger.Warn("failed to summarize conversation history", zap.Error(err))
		return ""
	}
	return summary
}

func (srv *conversationService) SubmitFeedback(ctx context.Context, messageUUID string, feedback int32) error {
	return srv.messageRepo.UpdateFeedback(ctx, messageUUID, feedback)
}
