package ollamatools_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/holmes89/grey-seal/lib/repo/ollamatools"
)

type ClientSuite struct {
	suite.Suite
}

func TestClientSuite(t *testing.T) { suite.Run(t, new(ClientSuite)) }

func (s *ClientSuite) TestChat_ParsesToolCalls() {
	var gotReq map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotReq)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"create_ticket","arguments":{"title":"Do a thing","draft":true}}}]},"done":true,"done_reason":"stop"}` + "\n"))
	}))
	defer srv.Close()

	c := ollamatools.New(srv.URL)
	msg, err := c.Chat(context.Background(), "qwen3:8b",
		[]ollamatools.Message{{Role: "user", Content: "go"}},
		[]ollamatools.Tool{ollamatools.NewTool("create_ticket", "make a ticket", map[string]any{"type": "object"})},
	)
	require.NoError(s.T(), err)
	require.Len(s.T(), msg.ToolCalls, 1)
	require.Equal(s.T(), "create_ticket", msg.ToolCalls[0].Function.Name)
	require.Equal(s.T(), "Do a thing", msg.ToolCalls[0].Function.Arguments["title"])
	require.Equal(s.T(), true, msg.ToolCalls[0].Function.Arguments["draft"])

	// the request carried stream:false, the tool definition, and a low temperature
	require.Equal(s.T(), false, gotReq["stream"])
	require.NotEmpty(s.T(), gotReq["tools"])
	require.Equal(s.T(), 0.2, gotReq["options"].(map[string]any)["temperature"])
}

func (s *ClientSuite) TestChat_PlainMessage() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"all done"},"done":true}` + "\n"))
	}))
	defer srv.Close()

	c := ollamatools.New(srv.URL)
	msg, err := c.Chat(context.Background(), "m", []ollamatools.Message{{Role: "user", Content: "x"}}, nil)
	require.NoError(s.T(), err)
	require.Empty(s.T(), msg.ToolCalls)
	require.Equal(s.T(), "all done", msg.Content)
}

func (s *ClientSuite) TestChat_HTTPError() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	c := ollamatools.New(srv.URL)
	_, err := c.Chat(context.Background(), "m", []ollamatools.Message{{Role: "user", Content: "x"}}, nil)
	require.Error(s.T(), err)
	require.Contains(s.T(), err.Error(), "500")
}
