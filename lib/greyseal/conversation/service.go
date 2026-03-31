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
	logger           *zap.Logger
}

func NewConversationService(
	conversationRepo base.Repository[*greysealv1.Conversation],
	messageRepo MessageRepository,
	searcher Searcher,
	roleRepo RoleRepository,
	llm LLM,
	logger *zap.Logger,
) ConversationService {
	return &conversationService{
		conversationRepo: conversationRepo,
		messageRepo:      messageRepo,
		searcher:         searcher,
		roleRepo:         roleRepo,
		llm:              llm,
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

	// Build LLM messages list
	var llmMessages []LLMMessage

	// 3. Load role system prompt if role_uuid is set
	if conv.RoleUuid != "" && srv.roleRepo != nil {
		role, err := srv.roleRepo.Get(ctx, conv.RoleUuid)
		if err == nil && role.SystemPrompt != "" {
			llmMessages = append(llmMessages, LLMMessage{
				Role:    "system",
				Content: role.SystemPrompt,
			})
		}
	}

	// 4. Load recent message history (last 10 messages)
	history, err := srv.messageRepo.ListByConversation(ctx, conversationUUID)
	if err != nil {
		history = nil // non-fatal; continue without history
	}
	// Take last 10 messages (excluding the one we just saved)
	if len(history) > 11 {
		history = history[len(history)-11:]
	}
	// Drop the last element (user message we just persisted)
	if len(history) > 0 && history[len(history)-1].Uuid == userMsg.Uuid {
		history = history[:len(history)-1]
	}
	if len(history) > 10 {
		history = history[len(history)-10:]
	}

	// 5. Search shrike for relevant context (scoped to conversation resources if set)
	if srv.searcher != nil {
		results, err := srv.searcher.Search(ctx, content, 5, conv.ResourceUuids)
		if err == nil && len(results) > 0 {
			var contextParts []string
			for i, r := range results {
				contextParts = append(contextParts, fmt.Sprintf("%d. %s", i+1, r.Snippet))
			}
			llmMessages = append(llmMessages, LLMMessage{
				Role:    "system",
				Content: "Here is relevant context:\n" + strings.Join(contextParts, "\n"),
			})
		}
	}

	// 8. Add message history
	for _, msg := range history {
		role := "user"
		if msg.Role == greysealv1.MessageRole_MESSAGE_ROLE_ASSISTANT {
			role = "assistant"
		}
		llmMessages = append(llmMessages, LLMMessage{
			Role:    role,
			Content: msg.Content,
		})
	}

	// Add current user message last
	llmMessages = append(llmMessages, LLMMessage{
		Role:    "user",
		Content: content,
	})

	// 9. Call LLM (with streaming) or fall back to placeholder
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

	// 10. Save assistant message to DB
	assistantMsg := &greysealv1.Message{
		Uuid:             uuid.New().String(),
		ConversationUuid: conversationUUID,
		Role:             greysealv1.MessageRole_MESSAGE_ROLE_ASSISTANT,
		Content:          responseContent,
		CreatedAt:        timestamppb.New(time.Now()),
	}

	if err := srv.messageRepo.Create(ctx, assistantMsg); err != nil {
		return nil, fmt.Errorf("failed to save assistant message: %w", err)
	}

	// Update conversation updated_at (preserve existing fields to avoid overwriting with nulls)
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

func (srv *conversationService) SubmitFeedback(ctx context.Context, messageUUID string, feedback int32) error {
	return srv.messageRepo.UpdateFeedback(ctx, messageUUID, feedback)
}
