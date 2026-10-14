package conversation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/holmes89/grey-seal/lib/greyseal/worktype"
	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// offerChain saves a follow-up message proposing the next work type after a
// draft is kept, and records the offer on the conversation so the next Chat
// call can recognize an answer to it. A kind with no NextWorkType, or an
// unknown next id, is silently a no-op — keeping a draft must never fail
// because of this.
func (srv *conversationService) offerChain(ctx context.Context, conv *greysealv1.Conversation, keptKind, keptContent string) {
	if srv.workTypes == nil {
		return
	}
	kept, ok := srv.workTypes.ByDraftKind(keptKind)
	if !ok || kept.NextWorkType == "" {
		return
	}
	next, ok := srv.workTypes.Get(kept.NextWorkType)
	if !ok {
		return
	}

	// Record the offer first: the message below promises it, and a promise
	// with nothing behind it is what made "yes" fall through to the old type.
	if err := srv.drafts.PutOffer(ctx, conv.Uuid, ChainOffer{WorkType: next.ID, Source: keptContent}); err != nil {
		srv.logger.Warn("failed to record chain offer", zap.String("conversation_uuid", conv.Uuid), zap.Error(err))
		return
	}
	offer := srv.newAssistantMessage(conv.Uuid, fmt.Sprintf("Would you like me to move onto %s next?", next.Label))
	if err := srv.messageRepo.Create(ctx, offer); err != nil {
		srv.logger.Warn("failed to save chain offer", zap.String("conversation_uuid", conv.Uuid), zap.Error(err))
		_ = srv.drafts.DeleteOffer(ctx, conv.Uuid)
	}
}

// resolveChainOffer answers a reply to a pending chain offer. ok is false
// when the conversation has no pending offer, in which case the caller
// should fall through to its normal handling of content.
func (srv *conversationService) resolveChainOffer(ctx context.Context, conv *greysealv1.Conversation, content string, stream func(token string) error) (msg *greysealv1.Message, ok bool, err error) {
	offer, found, err := srv.drafts.GetOffer(ctx, conv.Uuid)
	if err != nil {
		// Without the store we cannot tell; do not block ordinary chat.
		srv.logger.Warn("failed to load chain offer", zap.String("conversation_uuid", conv.Uuid), zap.Error(err))
		return nil, false, nil
	}
	if !found {
		return nil, false, nil
	}
	offeredID := offer.WorkType
	source := offer.Source

	// An offer is answered once, whatever the reply.
	if err := srv.drafts.DeleteOffer(ctx, conv.Uuid); err != nil {
		return nil, true, fmt.Errorf("failed to clear chain offer: %w", err)
	}

	if !acceptsChainOffer(content, offeredID, srv.workTypes) {
		// Declined, or said something unrelated: don't block ordinary chat.
		return nil, false, nil
	}

	next, ok := srv.workTypes.Get(offeredID)
	if !ok || next.DraftKind == "" || srv.drafter == nil {
		return nil, false, nil
	}

	transitioned := &greysealv1.Conversation{
		Uuid:          conv.Uuid,
		Title:         conv.Title,
		WorkType:      next.ID,
		ResourceUuids: conv.ResourceUuids,
		Summary:       conv.Summary,
		UpdatedAt:     timestamppb.New(time.Now()),
	}
	if err := srv.conversationRepo.Update(ctx, conv.Uuid, transitioned); err != nil {
		return nil, true, fmt.Errorf("failed to set work type: %w", err)
	}
	srv.logger.Info("chain offer accepted", zap.String("conversation_uuid", conv.Uuid), zap.String("work_type", next.ID))

	msg, err = srv.draftFromSeed(ctx, transitioned, next, source, stream)
	return msg, true, err
}

// acceptsChainOffer reports whether content answers yes to a chain offer:
// either a plain affirmative, or the offered type's own id/label (so
// replying "design" works just like it does for the initial work-type pick).
func acceptsChainOffer(content string, offeredID string, types worktype.Registry) bool {
	switch strings.TrimSpace(strings.ToLower(content)) {
	case "yes", "yeah", "yep", "sure", "ok", "okay", "please", "please do", "go ahead", "do it":
		return true
	}
	if types == nil {
		return false
	}
	wt, ok := types.Match(content)
	return ok && wt.ID == offeredID
}

// draftFromSeed drafts a work type directly from seed content (the previous
// step's kept draft) rather than deriving source material from the chat
// transcript, and without re-asking the type's own prompt.
func (srv *conversationService) draftFromSeed(ctx context.Context, conv *greysealv1.Conversation, wt worktype.WorkType, seed string, stream func(token string) error) (*greysealv1.Message, error) {
	kind, ok := draftKinds[wt.DraftKind]
	if !ok {
		return nil, fmt.Errorf("work type %q has unsupported draft kind %q", wt.ID, wt.DraftKind)
	}
	title := strings.TrimSpace(conv.Title)
	if title == "" {
		title = firstLine(seed)
	}
	srv.logger.Info("drafting chained work type", zap.String("conversation_uuid", conv.Uuid), zap.String("work_type", wt.ID))
	return srv.finishDraft(ctx, conv, wt, kind, title, seed, "", stream)
}
