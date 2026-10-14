package conversation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/holmes89/grey-seal/lib/greyseal/conversation"
	"github.com/holmes89/grey-seal/lib/greyseal/draft"
	"github.com/holmes89/grey-seal/lib/greyseal/worktype"
	v1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// memConvRepo is an in-memory ConversationRepository that mirrors the real
// repo's persistence rules (lib/repo/conversation.go): only the persisted
// columns survive Update/Get, work_type is kept when an update omits it, and
// the chain-offer fields are written as-is. Mocks cannot catch a field that
// the real repo forgets to store; this fake can.
type memConvRepo struct{ rows map[string]*v1.Conversation }

func newMemConvRepo() *memConvRepo { return &memConvRepo{rows: map[string]*v1.Conversation{}} }

func (r *memConvRepo) Create(_ context.Context, c *v1.Conversation) error {
	r.rows[c.Uuid] = &v1.Conversation{
		Uuid: c.Uuid, Title: c.Title, WorkType: c.WorkType, ResourceUuids: c.ResourceUuids,
		Summary: c.Summary, PendingChainOffer: c.PendingChainOffer, PendingChainOfferSource: c.PendingChainOfferSource,
	}
	return nil
}

func (r *memConvRepo) Update(_ context.Context, id string, c *v1.Conversation) error {
	row, ok := r.rows[id]
	if !ok {
		return errors.New("not found")
	}
	row.Title = c.Title
	if c.WorkType != "" {
		row.WorkType = c.WorkType
	}
	row.ResourceUuids = c.ResourceUuids
	row.Summary = c.Summary
	row.PendingChainOffer = c.PendingChainOffer
	row.PendingChainOfferSource = c.PendingChainOfferSource
	return nil
}

func (r *memConvRepo) Delete(_ context.Context, id string) error { delete(r.rows, id); return nil }

func (r *memConvRepo) Get(_ context.Context, id string) (*v1.Conversation, error) {
	row, ok := r.rows[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return proto.Clone(row).(*v1.Conversation), nil
}

func (r *memConvRepo) List(context.Context, string, uint, map[string][]any) ([]*v1.Conversation, error) {
	return nil, nil
}

type memMsgRepo struct{ msgs []*v1.Message }

func (r *memMsgRepo) Create(_ context.Context, m *v1.Message) error {
	r.msgs = append(r.msgs, proto.Clone(m).(*v1.Message))
	return nil
}

func (r *memMsgRepo) Update(_ context.Context, id string, m *v1.Message) error {
	for i, x := range r.msgs {
		if x.Uuid == id {
			r.msgs[i] = proto.Clone(m).(*v1.Message)
		}
	}
	return nil
}

func (r *memMsgRepo) Delete(_ context.Context, id string) error {
	for i, x := range r.msgs {
		if x.Uuid == id {
			r.msgs = append(r.msgs[:i], r.msgs[i+1:]...)
			return nil
		}
	}
	return nil
}

func (r *memMsgRepo) Get(_ context.Context, id string) (*v1.Message, error) {
	for _, x := range r.msgs {
		if x.Uuid == id {
			return proto.Clone(x).(*v1.Message), nil
		}
	}
	return nil, errors.New("not found")
}

func (r *memMsgRepo) List(context.Context, string, uint, map[string][]any) ([]*v1.Message, error) {
	return r.msgs, nil
}

func (r *memMsgRepo) ListByConversation(_ context.Context, id string) ([]*v1.Message, error) {
	var out []*v1.Message
	for _, m := range r.msgs {
		if m.ConversationUuid == id {
			out = append(out, proto.Clone(m).(*v1.Message))
		}
	}
	return out, nil
}

func (r *memMsgRepo) UpdateFeedback(context.Context, string, int32) error { return nil }

type recordingDrafter struct{ reqs []draft.DraftRequest }

func (d *recordingDrafter) Draft(_ context.Context, req draft.DraftRequest, _ func(string) error) (*draft.Result, error) {
	d.reqs = append(d.reqs, req)
	switch req.Kind {
	case draft.KindDesign:
		return &draft.Result{Body: "# Design: Toggle\n\n## Summary\nx"}, nil
	default:
		return &draft.Result{Body: "# Feature Discovery: Toggle - one line\n\n## Overview\nx"}, nil
	}
}

type chainFlow struct {
	t       *testing.T
	convs   *memConvRepo
	msgs    *memMsgRepo
	drafter *recordingDrafter
	svc     conversation.ConversationService
}

func newChainFlow(t *testing.T) *chainFlow {
	types, err := worktype.Load("")
	require.NoError(t, err)
	f := &chainFlow{t: t, convs: newMemConvRepo(), msgs: &memMsgRepo{}, drafter: &recordingDrafter{}}
	f.svc = conversation.NewConversationService(f.convs, f.msgs, nil, types, f.drafter, &fakeRemora{}, nil, nil, zap.NewNop(), nil)
	return f
}

func (f *chainFlow) say(content string) *v1.Message {
	msg, err := f.svc.Chat(context.Background(), "c1", content, func(string) error { return nil })
	require.NoError(f.t, err)
	return msg
}

// keptDiscovery walks a new chat through pick Discovery -> blurb -> Keep.
func (f *chainFlow) keptDiscovery() {
	ctx := context.Background()
	_, err := f.svc.Create(ctx, &v1.Conversation{Uuid: "c1", Title: "Runner toggle"})
	require.NoError(f.t, err)
	f.say("discovery")
	draftMsg := f.say("Toggle between cloud and self-hosted runners")
	require.Equal(f.t, "discovery", draftMsg.GetDraftKind())

	_, err = f.svc.KeepDraft(ctx, conversation.KeepDraftInput{ConversationUUID: "c1", MessageUUID: draftMsg.GetUuid(), ProductUUID: "prod"})
	require.NoError(f.t, err)
}

func TestChainOffer_YesAfterKeepDraftsADesignNotAnotherDiscovery(t *testing.T) {
	f := newChainFlow(t)
	f.keptDiscovery()

	conv, err := f.convs.Get(context.Background(), "c1")
	require.NoError(t, err)
	require.Equal(t, "design", conv.GetPendingChainOffer(), "the offer must be stored, not just set in memory")

	msg := f.say("yes")

	require.Equal(t, "design", msg.GetDraftKind())
	require.Contains(t, msg.GetContent(), "# Design:")
	last := f.drafter.reqs[len(f.drafter.reqs)-1]
	require.Equal(t, draft.KindDesign, last.Kind)
	require.Contains(t, last.Source, "# Feature Discovery:", "the design is seeded from the kept discovery doc")

	conv, err = f.convs.Get(context.Background(), "c1")
	require.NoError(t, err)
	require.Equal(t, "design", conv.GetWorkType())
	require.Empty(t, conv.GetPendingChainOffer(), "an answered offer is cleared")
}

func TestChainOffer_DeclineFallsThroughToTheCurrentWorkType(t *testing.T) {
	f := newChainFlow(t)
	f.keptDiscovery()
	before := len(f.drafter.reqs)

	msg := f.say("no thanks, add a section on rollout")

	require.Equal(t, "discovery", msg.GetDraftKind())
	require.Equal(t, draft.KindDiscovery, f.drafter.reqs[before].Kind)
	conv, err := f.convs.Get(context.Background(), "c1")
	require.NoError(t, err)
	require.Equal(t, "discovery", conv.GetWorkType())
	require.Empty(t, conv.GetPendingChainOffer())
}
