package conversation

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/holmes89/grey-seal/lib/greyseal/draft"
	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"go.uber.org/zap"
)

// Errors KeepDraft returns for caller mistakes, so transports can map them
// to a client error rather than an internal one.
var (
	ErrKeepUnavailable = errors.New("keeping drafts needs REMORA_MCP_URL to be configured")
	ErrKeepInvalid     = errors.New("invalid keep request")
)

// RemoraWriter writes kept drafts to Remora. Each method returns the new
// entity's uuid.
type RemoraWriter interface {
	CreateDesign(ctx context.Context, projectUUID, title, description, body, narwhalDocUUID string) (string, error)
	CreateDiscoveryDoc(ctx context.Context, productUUID, title, slug, description, body string) (string, error)
	CreateTicket(ctx context.Context, projectUUID, designUUID, title, body string) (string, error)
}

// KeepDraftInput identifies the draft and where in Remora it should go.
type KeepDraftInput struct {
	ConversationUUID string
	MessageUUID      string
	// ProjectUUID is required for design and tickets drafts.
	ProjectUUID string
	// ProductUUID is required for discovery drafts.
	ProductUUID string
	// DesignUUID optionally links kept tickets to a design.
	DesignUUID string
	// NarwhalDocUUID optionally links a kept design back to the discovery
	// doc it was drafted from.
	NarwhalDocUUID string
}

var (
	headingPrefix = regexp.MustCompile(`^#\s+(?:Feature Discovery|Design):\s*`)
	slugUnsafe    = regexp.MustCompile(`[^a-z0-9]+`)
)

func (srv *conversationService) KeepDraft(ctx context.Context, in KeepDraftInput) (*greysealv1.Message, error) {
	if srv.remora == nil {
		return nil, ErrKeepUnavailable
	}
	msg, err := srv.messageRepo.Get(ctx, in.MessageUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to load message: %w", err)
	}
	if msg.ConversationUuid != in.ConversationUUID || msg.DraftKind == "" {
		return nil, fmt.Errorf("%w: message is not a draft in this conversation", ErrKeepInvalid)
	}
	if isDraftStub(msg) {
		// The body lives in the DraftStore until kept; if it is gone the
		// draft expired and there is nothing to write.
		body, ok, err := srv.drafts.GetDraft(ctx, msg.Uuid)
		if err != nil {
			return nil, fmt.Errorf("failed to load draft: %w", err)
		}
		if !ok {
			return nil, fmt.Errorf("%w: this draft expired; send your message again to draft it again", ErrKeepInvalid)
		}
		msg.Content = body
	}
	if msg.KeptRef != "" {
		return nil, fmt.Errorf("%w: draft was already kept (%s)", ErrKeepInvalid, msg.KeptRef)
	}
	conv, err := srv.conversationRepo.Get(ctx, in.ConversationUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to load conversation: %w", err)
	}

	title := draftTitle(msg.Content, conv.Title)
	var ref string
	switch msg.DraftKind {
	case "design":
		if in.ProjectUUID == "" {
			return nil, fmt.Errorf("%w: project_uuid is required to keep a design", ErrKeepInvalid)
		}
		id, err := srv.remora.CreateDesign(ctx, in.ProjectUUID, title, "", msg.Content, in.NarwhalDocUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to write design to remora: %w", err)
		}
		ref = "design:" + id
	case "discovery":
		if in.ProductUUID == "" {
			return nil, fmt.Errorf("%w: product_uuid is required to keep a discovery doc", ErrKeepInvalid)
		}
		id, err := srv.remora.CreateDiscoveryDoc(ctx, in.ProductUUID, title, slugify(title), "", msg.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to write discovery doc to remora: %w", err)
		}
		ref = "doc:" + id
	case "tickets":
		if in.ProjectUUID == "" {
			return nil, fmt.Errorf("%w: project_uuid is required to keep tickets", ErrKeepInvalid)
		}
		tickets := draft.ParseProposedTickets(msg.Content)
		if len(tickets) == 0 {
			return nil, fmt.Errorf("%w: the draft has no tickets", ErrKeepInvalid)
		}
		var ids []string
		for _, t := range tickets {
			id, err := srv.remora.CreateTicket(ctx, in.ProjectUUID, in.DesignUUID, t.Title, ticketBody(t))
			if err != nil {
				// Tickets already written are not rolled back; say how far we got.
				return nil, fmt.Errorf("failed to write ticket %q to remora (%d of %d created): %w", t.Title, len(ids), len(tickets), err)
			}
			ids = append(ids, id)
		}
		ref = "tickets:" + strings.Join(ids, ",")
	default:
		return nil, fmt.Errorf("%w: unsupported draft kind %q", ErrKeepInvalid, msg.DraftKind)
	}

	msg.KeptRef = ref
	if err := srv.messageRepo.Update(ctx, msg.Uuid, msg); err != nil {
		// The entity exists in Remora; surface the ref so it is not lost.
		srv.logger.Error("draft kept in remora but message update failed", zap.String("message_uuid", msg.Uuid), zap.String("kept_ref", ref), zap.Error(err))
		return nil, fmt.Errorf("draft written to remora (%s) but recording it failed: %w", ref, err)
	}
	srv.logger.Info("draft kept", zap.String("message_uuid", msg.Uuid), zap.String("kept_ref", ref))
	_ = srv.drafts.DeleteDraft(ctx, msg.Uuid) // the body now lives on the message

	srv.offerChain(ctx, conv, msg.DraftKind, msg.Content)

	return msg, nil
}

// draftTitle takes the title from the draft's "# Design: <Name>" heading,
// falling back to the conversation title and then a generic one.
func draftTitle(body, convTitle string) string {
	first, _, _ := strings.Cut(strings.TrimSpace(body), "\n")
	if h := strings.TrimSpace(headingPrefix.ReplaceAllString(strings.TrimSpace(first), "")); strings.HasPrefix(strings.TrimSpace(first), "#") && h != "" {
		h = strings.TrimSpace(strings.TrimLeft(h, "#"))
		// A discovery heading is "<Name> - <One-line description>".
		name, _, _ := strings.Cut(h, " - ")
		if name = strings.TrimSpace(name); name != "" {
			return name
		}
	}
	if t := strings.TrimSpace(convTitle); t != "" {
		return t
	}
	return "Untitled draft"
}

func slugify(s string) string {
	return strings.Trim(slugUnsafe.ReplaceAllString(strings.ToLower(s), "-"), "-")
}

// ticketBody folds acceptance criteria and dependencies into the body, since
// Remora's create_ticket cannot set them directly.
func ticketBody(t draft.Ticket) string {
	var b strings.Builder
	b.WriteString(t.Body)
	if len(t.AcceptanceCriteria) > 0 {
		b.WriteString("\n\n## Acceptance criteria")
		for _, ac := range t.AcceptanceCriteria {
			b.WriteString("\n- " + ac)
		}
	}
	if len(t.DependsOn) > 0 {
		b.WriteString("\n\nDepends on: " + strings.Join(t.DependsOn, ", "))
	}
	return strings.TrimSpace(b.String())
}
