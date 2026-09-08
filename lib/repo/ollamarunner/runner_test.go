package ollamarunner_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"

	agentsvc "github.com/holmes89/grey-seal/lib/greyseal/agent"
	"github.com/holmes89/grey-seal/lib/greyseal/agent/mocks"
	"github.com/holmes89/grey-seal/lib/repo/ollamarunner"
	"github.com/holmes89/grey-seal/lib/repo/ollamatools"
)

type RunnerSuite struct {
	suite.Suite
}

func TestRunnerSuite(t *testing.T) { suite.Run(t, new(RunnerSuite)) }

// ollamaStub serves: call 1 → a create_ticket tool call; call 2 → a plain
// "done" message.
func ollamaStub() *httptest.Server {
	var calls atomic.Int32
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if calls.Add(1) == 1 {
			_, _ = w.Write([]byte(`{"message":{"role":"assistant","tool_calls":[{"function":{"name":"create_ticket","arguments":{"project_uuid":"p1","title":"Add X","draft":true}}}]},"done":true}` + "\n"))
			return
		}
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"created 1 draft ticket"},"done":true}` + "\n"))
	}))
}

func (s *RunnerSuite) TestRun_CreatesDraftThenFinishes() {
	ollama := ollamaStub()
	defer ollama.Close()

	toolSession := mocks.NewMockToolSession(s.T())
	toolSession.On("ListTools", mock.Anything).Return([]agentsvc.ToolDef{
		{Name: "list_projects", Description: "list", InputSchema: map[string]any{"type": "object"}},
		{Name: "create_ticket", Description: "create", InputSchema: map[string]any{"type": "object"}},
		{Name: "search_knowledge", Description: "not allowed", InputSchema: map[string]any{"type": "object"}},
	}, nil)
	var createArgs map[string]any
	toolSession.On("CallTool", mock.Anything, "create_ticket", mock.Anything).
		Run(func(args mock.Arguments) { createArgs = args.Get(2).(map[string]any) }).
		Return(`{"data":{"uuid":"t1","status":"draft"}}`, nil)
	toolSession.On("Close").Return(nil)

	provider := mocks.NewMockToolProvider(s.T())
	provider.On("Session", mock.Anything).Return(toolSession, nil)

	r := ollamarunner.NewSessionRunner(provider, ollamatools.New(ollama.URL), "qwen3:8b", zap.NewNop())

	id, err := r.StartSession(context.Background(), agentsvc.RunAgentTaskRequest{
		Provider:        "ollama:qwen3:8b",
		TaskDescription: "Design: add feature X",
	})
	require.NoError(s.T(), err)

	status, outcome := waitTerminal(s.T(), r, id)
	require.Equal(s.T(), "terminated", status)
	require.Equal(s.T(), "satisfied", outcome)
	require.Equal(s.T(), true, createArgs["draft"])
	require.Equal(s.T(), "p1", createArgs["project_uuid"])
}

func (s *RunnerSuite) TestRun_ToolSessionFailure() {
	provider := mocks.NewMockToolProvider(s.T())
	provider.On("Session", mock.Anything).Return(nil, assertErr{})

	r := ollamarunner.NewSessionRunner(provider, ollamatools.New("http://127.0.0.1:0"), "m", zap.NewNop())
	id, err := r.StartSession(context.Background(), agentsvc.RunAgentTaskRequest{
		Provider:        "ollama:m",
		TaskDescription: "x",
	})
	require.NoError(s.T(), err)

	status, outcome := waitTerminal(s.T(), r, id)
	require.Equal(s.T(), "terminated", status)
	require.Equal(s.T(), "failed", outcome)
}

func (s *RunnerSuite) TestStartSession_RequiresTask() {
	r := ollamarunner.NewSessionRunner(mocks.NewMockToolProvider(s.T()), ollamatools.New(""), "m", zap.NewNop())
	_, err := r.StartSession(context.Background(), agentsvc.RunAgentTaskRequest{Provider: "ollama:m"})
	require.Error(s.T(), err)
}

func waitTerminal(t *testing.T, r *ollamarunner.SessionRunner, id string) (string, string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		status, outcome, err := r.GetSessionStatus(context.Background(), id)
		require.NoError(t, err)
		if status == "terminated" {
			return status, outcome
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("run did not terminate in time")
	return "", ""
}

type assertErr struct{}

func (assertErr) Error() string { return "dial failed" }
