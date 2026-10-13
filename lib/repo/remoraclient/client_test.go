package remoraclient_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/holmes89/grey-seal/lib/greyseal/agent"
	"github.com/holmes89/grey-seal/lib/repo/remoraclient"
)

type call struct {
	name string
	args map[string]any
}

type fakeProvider struct {
	calls   []call
	result  string
	err     error
	sessErr error
	closed  int
}

func (f *fakeProvider) Session(context.Context) (agent.ToolSession, error) {
	if f.sessErr != nil {
		return nil, f.sessErr
	}
	return &fakeSession{f}, nil
}

type fakeSession struct{ f *fakeProvider }

func (s *fakeSession) ListTools(context.Context) ([]agent.ToolDef, error) { return nil, nil }
func (s *fakeSession) CallTool(_ context.Context, name string, args map[string]any) (string, error) {
	s.f.calls = append(s.f.calls, call{name, args})
	return s.f.result, s.f.err
}
func (s *fakeSession) Close() error { s.f.closed++; return nil }

func TestCreateDesign(t *testing.T) {
	f := &fakeProvider{result: `{"data":{"uuid":"d-1","title":"T"}}`}
	id, err := remoraclient.New(f).CreateDesign(context.Background(), "proj", "T", "desc", "# body")
	require.NoError(t, err)
	require.Equal(t, "d-1", id)
	require.Equal(t, "create_design", f.calls[0].name)
	require.Equal(t, "proj", f.calls[0].args["project_uuid"])
	require.Equal(t, "# body", f.calls[0].args["body"])
	require.Equal(t, 1, f.closed)
}

func TestCreateDiscoveryDoc(t *testing.T) {
	f := &fakeProvider{result: `{"data":{"uuid":"doc-1"}}`}
	id, err := remoraclient.New(f).CreateDiscoveryDoc(context.Background(), "prod", "T", "t", "desc", "body")
	require.NoError(t, err)
	require.Equal(t, "doc-1", id)
	require.Equal(t, "DOC_TYPE_DISCOVERY", f.calls[0].args["type"])
	require.Equal(t, "prod", f.calls[0].args["product_uuid"])
}

func TestCreateTicket_IsADraftLinkedToTheDesign(t *testing.T) {
	f := &fakeProvider{result: `{"data":{"uuid":"t-1"}}`}
	id, err := remoraclient.New(f).CreateTicket(context.Background(), "proj", "d-1", "Do it", "body")
	require.NoError(t, err)
	require.Equal(t, "t-1", id)
	require.Equal(t, true, f.calls[0].args["draft"])
	require.Equal(t, "d-1", f.calls[0].args["design_uuid"])

	_, err = remoraclient.New(f).CreateTicket(context.Background(), "proj", "", "No design", "body")
	require.NoError(t, err)
	require.NotContains(t, f.calls[1].args, "design_uuid")
}

func TestErrors(t *testing.T) {
	ctx := context.Background()

	_, err := remoraclient.New(&fakeProvider{sessErr: errors.New("down")}).CreateDesign(ctx, "p", "t", "", "b")
	require.ErrorContains(t, err, "down")

	f := &fakeProvider{err: errors.New("tool failed")}
	_, err = remoraclient.New(f).CreateDesign(ctx, "p", "t", "", "b")
	require.ErrorContains(t, err, "tool failed")
	require.Equal(t, 1, f.closed, "session is closed on failure")

	_, err = remoraclient.New(&fakeProvider{result: "not json"}).CreateDesign(ctx, "p", "t", "", "b")
	require.ErrorContains(t, err, "returned no uuid")
	_, err = remoraclient.New(&fakeProvider{result: `{"data":{}}`}).CreateDesign(ctx, "p", "t", "", "b")
	require.ErrorContains(t, err, "returned no uuid")
}
