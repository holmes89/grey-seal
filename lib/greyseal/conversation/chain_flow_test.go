package conversation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/holmes89/grey-seal/lib/greyseal/conversation"
	"github.com/holmes89/grey-seal/lib/greyseal/draft"
	"github.com/holmes89/grey-seal/lib/greyseal/worktype"
	"github.com/holmes89/grey-seal/lib/repo/draftstore"
	v1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// memConvRepo is an in-memory ConversationRepository that mirrors the real
// repo's persistence rules (lib/repo/conversation.go): only the persisted
// columns survive Update/Get (the chain-offer fields are deliberately not
// among them) and work_type is kept when an update omits it. Mocks cannot
// catch state that the real repo forgets to store; this fake can.
type memConvRepo struct {
	rows map[string]*v1.Conversation
	msgs *memMsgRepo // Get loads messages from here, as the real repo does
}

func newMemConvRepo() *memConvRepo { return &memConvRepo{rows: map[string]*v1.Conversation{}} }

func (r *memConvRepo) Create(_ context.Context, c *v1.Conversation) error {
	r.rows[c.Uuid] = &v1.Conversation{
		Uuid: c.Uuid, Title: c.Title, WorkType: c.WorkType, ResourceUuids: c.ResourceUuids,
		Summary: c.Summary,
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
	return nil
}

func (r *memConvRepo) Delete(_ context.Context, id string) error { delete(r.rows, id); return nil }

func (r *memConvRepo) Get(_ context.Context, id string) (*v1.Conversation, error) {
	row, ok := r.rows[id]
	if !ok {
		return nil, errors.New("not found")
	}
	out := proto.Clone(row).(*v1.Conversation)
	if r.msgs != nil {
		out.Messages, _ = r.msgs.ListByConversation(context.Background(), id)
	}
	return out, nil
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
	store   *draftstore.Memory
	convs   *memConvRepo
	msgs    *memMsgRepo
	drafter *recordingDrafter
	svc     conversation.ConversationService
}

func newChainFlow(t *testing.T) *chainFlow {
	types, err := worktype.Load("")
	require.NoError(t, err)
	f := &chainFlow{t: t, store: draftstore.NewMemory(time.Hour), convs: newMemConvRepo(), msgs: &memMsgRepo{}, drafter: &recordingDrafter{}}
	f.convs.msgs = f.msgs
	f.svc = conversation.NewConversationService(f.convs, f.msgs, nil, types, f.drafter, &fakeRemora{}, f.store, nil, nil, zap.NewNop(), nil)
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

	offer, ok, err := f.store.GetOffer(context.Background(), "c1")
	require.NoError(t, err)
	require.True(t, ok, "the offer must be stored, not just set in memory")
	require.Equal(t, "design", offer.WorkType)

	msg := f.say("yes")

	require.Equal(t, "design", msg.GetDraftKind())
	require.Contains(t, msg.GetContent(), "# Design:")
	last := f.drafter.reqs[len(f.drafter.reqs)-1]
	require.Equal(t, draft.KindDesign, last.Kind)
	require.Contains(t, last.Source, "# Feature Discovery:", "the design is seeded from the kept discovery doc")

	conv, err := f.convs.Get(context.Background(), "c1")
	require.NoError(t, err)
	require.Equal(t, "design", conv.GetWorkType())
	_, ok, err = f.store.GetOffer(context.Background(), "c1")
	require.NoError(t, err)
	require.False(t, ok, "an answered offer is cleared")
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
	_, ok, err := f.store.GetOffer(context.Background(), "c1")
	require.NoError(t, err)
	require.False(t, ok)
}

// --- unkept drafts expire; kept drafts do not ---

func (f *chainFlow) get() *v1.Conversation {
	resp, err := f.svc.Get(context.Background(), &getReq{uuid: "c1"})
	require.NoError(f.t, err)
	return resp.GetData()
}

type getReq struct{ uuid string }

func (g *getReq) GetUuid() string { return g.uuid }

func (f *chainFlow) draftRows() []*v1.Message {
	var out []*v1.Message
	for _, m := range f.msgs.msgs {
		if m.DraftKind != "" {
			out = append(out, m)
		}
	}
	return out
}

func newClockedFlow(t *testing.T) (*chainFlow, *time.Time) {
	f := newChainFlow(t)
	now := time.Now()
	f.store.SetClock(func() time.Time { return now })
	return f, &now
}

func TestDrafts_UnkeptBodyLivesInTheStoreNotTheDatabase(t *testing.T) {
	f, _ := newClockedFlow(t)
	_, err := f.svc.Create(context.Background(), &v1.Conversation{Uuid: "c1"})
	require.NoError(t, err)
	f.say("design")
	msg := f.say("A runner toggle")

	require.Contains(t, msg.GetContent(), "# Design:", "the caller still gets the body")
	rows := f.draftRows()
	require.Len(t, rows, 1)
	require.Empty(t, rows[0].Content, "the database row is a stub")

	var viaGet *v1.Message
	for _, m := range f.get().GetMessages() {
		if m.Uuid == msg.GetUuid() {
			viaGet = m
		}
	}
	require.NotNil(t, viaGet)
	require.Equal(t, msg.GetContent(), viaGet.Content, "Get hydrates the body from the store")
	require.Equal(t, "design", viaGet.DraftKind)
}

func TestDrafts_ExpiredDraftShowsANoteAndOffersNothing(t *testing.T) {
	f, now := newClockedFlow(t)
	_, err := f.svc.Create(context.Background(), &v1.Conversation{Uuid: "c1"})
	require.NoError(t, err)
	f.say("design")
	msg := f.say("A runner toggle")

	*now = now.Add(25 * time.Hour)

	var viaGet *v1.Message
	for _, m := range f.get().GetMessages() {
		if m.Uuid == msg.GetUuid() {
			viaGet = m
		}
	}
	require.NotNil(t, viaGet)
	require.Contains(t, viaGet.Content, "expired")
	require.Empty(t, viaGet.DraftKind, "no Keep/Retry on an expired draft")

	_, err = f.svc.KeepDraft(context.Background(), conversation.KeepDraftInput{ConversationUUID: "c1", MessageUUID: msg.GetUuid(), ProjectUUID: "p"})
	require.ErrorIs(t, err, conversation.ErrKeepInvalid)
	require.ErrorContains(t, err, "expired")
}

func TestDrafts_KeepSavesTheBodyPermanently(t *testing.T) {
	f, now := newClockedFlow(t)
	_, err := f.svc.Create(context.Background(), &v1.Conversation{Uuid: "c1"})
	require.NoError(t, err)
	f.say("design")
	msg := f.say("A runner toggle")

	kept, err := f.svc.KeepDraft(context.Background(), conversation.KeepDraftInput{ConversationUUID: "c1", MessageUUID: msg.GetUuid(), ProjectUUID: "p"})
	require.NoError(t, err)
	require.NotEmpty(t, kept.GetKeptRef())

	row, err := f.msgs.Get(context.Background(), msg.GetUuid())
	require.NoError(t, err)
	require.Equal(t, msg.GetContent(), row.Content, "the body is on the message row once kept")

	*now = now.Add(48 * time.Hour)
	for _, m := range f.get().GetMessages() {
		if m.Uuid == msg.GetUuid() {
			require.Equal(t, msg.GetContent(), m.Content, "a kept draft outlives the ttl")
			require.Equal(t, "design", m.DraftKind)
			return
		}
	}
	t.Fatal("kept draft missing from the conversation")
}

func TestDrafts_RetryDropsTheOldBodyAndStoresTheNew(t *testing.T) {
	f, _ := newClockedFlow(t)
	_, err := f.svc.Create(context.Background(), &v1.Conversation{Uuid: "c1"})
	require.NoError(t, err)
	f.say("design")
	first := f.say("A runner toggle")

	second, err := f.svc.Regenerate(context.Background(), "c1", func(string) error { return nil })
	require.NoError(t, err)
	require.NotEqual(t, first.GetUuid(), second.GetUuid())

	_, ok, err := f.store.GetDraft(context.Background(), first.GetUuid())
	require.NoError(t, err)
	require.False(t, ok, "the discarded draft's body is gone")
	require.Len(t, f.draftRows(), 1, "only the new draft remains")
}
