package conversation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/holmes89/grey-seal/lib/greyseal/worktype"
	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// welcomeText asks what the user wants help with and lists the work types.
// It is built from the registry, not generated, so the reply it expects can
// be matched exactly.
func welcomeText(types worktype.Registry) string {
	return "Hi! How can I help? " + optionList(types) + "."
}

func retryText(types worktype.Registry) string {
	return "I didn't catch that. Please pick one of: " + optionList(types) + "."
}

func optionList(types worktype.Registry) string {
	var labels []string
	for _, wt := range types.List() {
		labels = append(labels, wt.Label)
	}
	switch len(labels) {
	case 0:
		return ""
	case 1:
		return labels[0]
	}
	return strings.Join(labels[:len(labels)-1], ", ") + ", or " + labels[len(labels)-1]
}

func (srv *conversationService) newAssistantMessage(conversationUUID, content string) *greysealv1.Message {
	return &greysealv1.Message{
		Uuid:             uuid.New().String(),
		ConversationUuid: conversationUUID,
		Role:             greysealv1.MessageRole_MESSAGE_ROLE_ASSISTANT,
		Content:          content,
		CreatedAt:        timestamppb.New(time.Now()),
	}
}

// selectWorkType handles the user's reply while a conversation is still
// pending. An exact match (id or label, any case) fixes the conversation's
// work type and answers with that type's prompt; anything else re-offers the
// options. No LLM is involved.
func (srv *conversationService) selectWorkType(ctx context.Context, conv *greysealv1.Conversation, content string, stream func(token string) error) (*greysealv1.Message, error) {
	reply := retryText(srv.workTypes)
	if wt, ok := srv.workTypes.Match(content); ok {
		reply = wt.Prompt
		if reply == "" {
			reply = "What can I help with?"
		}
		err := srv.conversationRepo.Update(ctx, conv.Uuid, &greysealv1.Conversation{
			Uuid:          conv.Uuid,
			Title:         conv.Title,
			WorkType:      wt.ID,
			ResourceUuids: conv.ResourceUuids,
			Summary:       conv.Summary,
			UpdatedAt:     timestamppb.New(time.Now()),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to set work type: %w", err)
		}
		srv.logger.Info("work type selected", zap.String("conversation_uuid", conv.Uuid), zap.String("work_type", wt.ID))
	}

	if err := stream(reply); err != nil {
		return nil, err
	}
	msg := srv.newAssistantMessage(conv.Uuid, reply)
	if err := srv.messageRepo.Create(ctx, msg); err != nil {
		return nil, fmt.Errorf("failed to save assistant message: %w", err)
	}
	return msg, nil
}
