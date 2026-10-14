package conversation

import (
	"context"
	"fmt"

	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// expiredDraftNote replaces the body of a draft whose TTL has passed.
const expiredDraftNote = "This draft expired. Send your message again to draft it again."

// An unkept draft is stored as a stub message (kind set, empty content, no
// kept_ref) with its body in the DraftStore, where it expires. Keeping the
// draft saves the body on the message permanently.
func isDraftStub(m *greysealv1.Message) bool {
	return m.DraftKind != "" && m.KeptRef == "" && m.Content == ""
}

// hydrate fills in the body of every unkept draft from the DraftStore. A
// draft whose body has expired becomes a plain assistant message carrying a
// note (its draft kind is cleared, so it no longer offers Keep or Retry and
// is not picked as the "current" draft). A store failure leaves the stub
// as-is and is logged, so one bad lookup does not hide the whole chat.
func (srv *conversationService) hydrate(ctx context.Context, msgs []*greysealv1.Message) {
	for _, m := range msgs {
		if !isDraftStub(m) {
			continue
		}
		body, ok, err := srv.drafts.GetDraft(ctx, m.Uuid)
		switch {
		case err != nil:
			srv.logger.Warn("failed to load draft body", zap.String("message_uuid", m.Uuid), zap.Error(err))
		case ok:
			m.Content = body
		default:
			m.Content = expiredDraftNote
			m.DraftKind = ""
		}
	}
}

// history returns a conversation's messages with draft bodies filled in.
func (srv *conversationService) history(ctx context.Context, conversationUUID string) ([]*greysealv1.Message, error) {
	msgs, err := srv.messageRepo.ListByConversation(ctx, conversationUUID)
	if err != nil {
		return nil, err
	}
	srv.hydrate(ctx, msgs)
	return msgs, nil
}

// storeDraft saves a new draft: the body in the DraftStore, a stub message in
// the database. The returned message carries the body for the caller.
func (srv *conversationService) storeDraft(ctx context.Context, msg *greysealv1.Message) (*greysealv1.Message, error) {
	body := msg.Content
	if err := srv.drafts.PutDraft(ctx, msg.Uuid, body); err != nil {
		return nil, fmt.Errorf("failed to store draft: %w", err)
	}
	stub := proto.Clone(msg).(*greysealv1.Message)
	stub.Content = ""
	if err := srv.messageRepo.Create(ctx, stub); err != nil {
		_ = srv.drafts.DeleteDraft(ctx, msg.Uuid)
		return nil, fmt.Errorf("failed to save draft: %w", err)
	}
	return msg, nil
}
