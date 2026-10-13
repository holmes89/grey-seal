package conversation

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/holmes89/grey-seal/lib/greyseal/draft"
	"github.com/holmes89/grey-seal/lib/greyseal/worktype"
	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var draftKinds = map[string]draft.Kind{
	"discovery": draft.KindDiscovery,
	"design":    draft.KindDesign,
	"tickets":   draft.KindTickets,
}

const maxDraftTitleLen = 80

// draftReply answers a message in a drafting work type (discovery, design,
// tickets) by running the user's input through the work type's template
// instead of free chat. The result is stored as a draft message the user can
// retry or keep. The source is everything the user has said since the type's
// prompt; the latest draft, if any, is passed back as the current draft so
// follow-up messages refine it rather than start over.
func (srv *conversationService) draftReply(ctx context.Context, conv *greysealv1.Conversation, wt worktype.WorkType, retry bool, stream func(token string) error) (*greysealv1.Message, error) {
	kind, ok := draftKinds[wt.DraftKind]
	if !ok {
		return nil, fmt.Errorf("work type %q has unsupported draft kind %q", wt.ID, wt.DraftKind)
	}
	history, err := srv.messageRepo.ListByConversation(ctx, conv.Uuid)
	if err != nil {
		return nil, fmt.Errorf("failed to load history: %w", err)
	}

	var current string
	if retry {
		// Discard the latest draft and draft again from the same input.
		if last := lastDraft(history); last != nil {
			if err := srv.messageRepo.Delete(ctx, last.Uuid); err != nil {
				return nil, fmt.Errorf("failed to discard draft: %w", err)
			}
		}
	} else if last := lastDraft(history); last != nil {
		current = last.Content
	}

	source := draftSource(history, wt.Prompt)
	if strings.TrimSpace(source) == "" {
		return nil, fmt.Errorf("nothing to draft from yet: send a message first")
	}
	title := strings.TrimSpace(conv.Title)
	if title == "" {
		title = firstLine(source)
	}

	srv.logger.Info("drafting in chat", zap.String("conversation_uuid", conv.Uuid), zap.String("work_type", wt.ID), zap.Bool("retry", retry))
	res, err := srv.drafter.Draft(ctx, draft.DraftRequest{Kind: kind, Title: title, Source: source, Current: current}, stream)
	if err != nil {
		srv.logger.Error("chat draft failed", zap.String("conversation_uuid", conv.Uuid), zap.Error(err))
		return nil, fmt.Errorf("draft failed: %w", err)
	}

	body := res.Body
	if kind == draft.KindTickets {
		body = renderTickets(res.Tickets)
	}
	if strings.TrimSpace(body) == "" {
		return nil, fmt.Errorf("the model returned an empty draft; try again")
	}

	msg := &greysealv1.Message{
		Uuid:             uuid.New().String(),
		ConversationUuid: conv.Uuid,
		Role:             greysealv1.MessageRole_MESSAGE_ROLE_ASSISTANT,
		Content:          body,
		DraftKind:        wt.DraftKind,
		CreatedAt:        timestamppb.New(time.Now()),
	}
	if err := srv.messageRepo.Create(ctx, msg); err != nil {
		return nil, fmt.Errorf("failed to save draft: %w", err)
	}

	// Surface any open questions the template left in the draft as a
	// follow-up nudge, rather than leaving them sitting inertly in the
	// markdown for the user to notice on their own. Best-effort: the draft
	// itself already succeeded, so a failure here is logged, not fatal.
	if questions := extractOpenQuestions(body); len(questions) > 0 {
		nudge := &greysealv1.Message{
			Uuid:             uuid.New().String(),
			ConversationUuid: conv.Uuid,
			Role:             greysealv1.MessageRole_MESSAGE_ROLE_ASSISTANT,
			Content:          openQuestionsNudge(questions),
			CreatedAt:        timestamppb.New(time.Now()),
		}
		if err := srv.messageRepo.Create(ctx, nudge); err != nil {
			srv.logger.Warn("failed to save open-questions nudge", zap.String("conversation_uuid", conv.Uuid), zap.Error(err))
		}
	}

	return msg, nil
}

func (srv *conversationService) Regenerate(ctx context.Context, conversationUUID string, stream func(token string) error) (*greysealv1.Message, error) {
	conv, err := srv.conversationRepo.Get(ctx, conversationUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to load conversation: %w", err)
	}
	wt, ok := srv.workType(conv.WorkType)
	if !ok || wt.DraftKind == "" || srv.drafter == nil {
		return nil, fmt.Errorf("conversation has no draft to retry")
	}
	return srv.draftReply(ctx, conv, wt, true, stream)
}

// lastDraft returns the most recent draft message, or nil.
func lastDraft(history []*greysealv1.Message) *greysealv1.Message {
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].DraftKind != "" {
			return history[i]
		}
	}
	return nil
}

// draftSource joins the user's messages that came after the work type's
// prompt (the last assistant message equal to it), oldest first.
func draftSource(history []*greysealv1.Message, prompt string) string {
	start := 0
	for i, m := range history {
		if m.Role == greysealv1.MessageRole_MESSAGE_ROLE_ASSISTANT && prompt != "" && m.Content == prompt {
			start = i + 1
		}
	}
	var parts []string
	for _, m := range history[start:] {
		if m.Role == greysealv1.MessageRole_MESSAGE_ROLE_USER {
			parts = append(parts, strings.TrimSpace(m.Content))
		}
	}
	return strings.Join(parts, "\n\n")
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	line = strings.TrimSpace(line)
	if utf8.RuneCountInString(line) > maxDraftTitleLen {
		line = string([]rune(line)[:maxDraftTitleLen])
	}
	return line
}

// renderTickets writes tickets back out in the markdown table the tickets
// draft template uses, so a kept message can be re-parsed with
// draft.ParseProposedTickets.
func renderTickets(tickets []draft.Ticket) string {
	cell := func(s string) string {
		return strings.TrimSpace(strings.NewReplacer("|", "/", "\n", " ").Replace(s))
	}
	var b strings.Builder
	b.WriteString("| Title | Body | Acceptance criteria | Depends on |\n| --- | --- | --- | --- |\n")
	for _, t := range tickets {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n",
			cell(t.Title), cell(t.Body), cell(strings.Join(t.AcceptanceCriteria, "; ")), cell(strings.Join(t.DependsOn, ", ")))
	}
	return b.String()
}
