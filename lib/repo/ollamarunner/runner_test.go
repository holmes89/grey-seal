package ollamarunner_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

// ollamaScript serves the given response bodies in order (the last one is
// repeated once exhausted) and records every request body it received.
type ollamaScript struct {
	srv      *httptest.Server
	mu       sync.Mutex
	requests []string
}

func newOllamaScript(bodies ...string) *ollamaScript {
	s := &ollamaScript{}
	var i atomic.Int32
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		s.requests = append(s.requests, string(body))
		s.mu.Unlock()
		n := int(i.Add(1)) - 1
		if n >= len(bodies) {
			n = len(bodies) - 1
		}
		if bodies[n] == "__ERROR__" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("boom"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(bodies[n] + "\n"))
	}))
	return s
}

func (s *ollamaScript) reqs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.requests...)
}

const (
	respCreateTicket  = `{"message":{"role":"assistant","tool_calls":[{"function":{"name":"create_ticket","arguments":{"project_uuid":"p1","title":"Add X","draft":true}}}]},"done":true}`
	respDone          = `{"message":{"role":"assistant","content":"created 1 draft ticket"},"done":true}`
	respProse         = `{"message":{"role":"assistant","content":"Here is how I would break this down: ..."},"done":true}`
	respThinkThenDone = `{"message":{"role":"assistant","content":"<think>the user wants tickets</think>all done"},"done":true}`
)

func toolMocks(t *testing.T) (*mocks.MockToolProvider, *mocks.MockToolSession) {
	sess := mocks.NewMockToolSession(t)
	sess.On("ListTools", mock.Anything).Return([]agentsvc.ToolDef{
		{Name: "list_projects", Description: "list", InputSchema: map[string]any{"type": "object"}},
		{Name: "list_tickets", Description: "list", InputSchema: map[string]any{"type": "object"}},
		{Name: "create_ticket", Description: "create", InputSchema: map[string]any{"type": "object"}},
	}, nil)
	// the runner resolves the project list itself before the loop
	sess.On("CallTool", mock.Anything, "list_projects", mock.Anything).
		Return(`{"projects":[{"uuid":"p1","name":"ahh"}]}`, nil).Maybe()
	sess.On("Close").Return(nil)
	prov := mocks.NewMockToolProvider(t)
	prov.On("Session", mock.Anything).Return(sess, nil)
	return prov, sess
}

func run(t *testing.T, script *ollamaScript, prov agentsvc.ToolProvider) (string, string) {
	t.Helper()
	r := ollamarunner.NewSessionRunner(prov, ollamatools.New(script.srv.URL), "qwen3:8b", zap.NewNop())
	id, err := r.StartSession(context.Background(), agentsvc.RunAgentTaskRequest{
		Provider: "ollama:qwen3:8b", TaskDescription: "Design: add feature X",
	})
	require.NoError(t, err)
	return waitTerminal(t, r, id)
}

func (s *RunnerSuite) TestRun_CreatesDraftThenFinishes() {
	script := newOllamaScript(respCreateTicket, respDone)
	defer script.srv.Close()
	prov, sess := toolMocks(s.T())
	var createArgs map[string]any
	sess.On("CallTool", mock.Anything, "create_ticket", mock.Anything).
		Run(func(a mock.Arguments) { createArgs = a.Get(2).(map[string]any) }).
		Return(`{"data":{"uuid":"t1","status":"draft"}}`, nil)

	status, outcome := run(s.T(), script, prov)
	require.Equal(s.T(), "terminated", status)
	require.Equal(s.T(), "satisfied", outcome)
	require.Equal(s.T(), true, createArgs["draft"])
	require.Equal(s.T(), "p1", createArgs["project_uuid"])
}

func (s *RunnerSuite) TestRun_EmitsProgressHeartbeats() {
	script := newOllamaScript(respCreateTicket, respDone)
	defer script.srv.Close()
	prov, sess := toolMocks(s.T())
	sess.On("CallTool", mock.Anything, "create_ticket", mock.Anything).
		Return(`{"data":{"uuid":"t1"}}`, nil)

	r := ollamarunner.NewSessionRunner(prov, ollamatools.New(script.srv.URL), "qwen3:8b", zap.NewNop())
	id, err := r.StartSession(context.Background(), agentsvc.RunAgentTaskRequest{
		Provider: "ollama:qwen3:8b", TaskDescription: "d",
	})
	require.NoError(s.T(), err)
	waitTerminal(s.T(), r, id)

	var msgs []string
	_ = r.StreamSession(context.Background(), id, func(e agentsvc.AgentRunEvent) error {
		if e.Type == "agent.message" {
			msgs = append(msgs, e.Message)
		}
		return nil
	})
	joined := strings.Join(msgs, "\n")
	require.Contains(s.T(), joined, "connecting to the ticket tools")
	require.Contains(s.T(), joined, "resolving the project list")
	require.Contains(s.T(), joined, "turn 1/")
}

func (s *RunnerSuite) TestRun_TurnTimeout() {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
		}
	}))
	defer slow.Close()
	prov, _ := toolMocks(s.T())

	r := ollamarunner.NewSessionRunner(prov, ollamatools.New(slow.URL), "m", zap.NewNop(),
		ollamarunner.WithTimeouts(150*time.Millisecond, 5*time.Second))
	id, err := r.StartSession(context.Background(), agentsvc.RunAgentTaskRequest{
		Provider: "ollama:m", TaskDescription: "d",
	})
	require.NoError(s.T(), err)

	status, outcome := waitTerminal(s.T(), r, id)
	require.Equal(s.T(), "terminated", status)
	require.Equal(s.T(), "failed", outcome)

	var msgs []string
	_ = r.StreamSession(context.Background(), id, func(e agentsvc.AgentRunEvent) error {
		msgs = append(msgs, e.Message)
		return nil
	})
	require.Contains(s.T(), strings.Join(msgs, "\n"), "timed out")
}

func (s *RunnerSuite) TestRun_NudgesPastAPreTextTurn() {
	script := newOllamaScript(respProse, respCreateTicket, respDone)
	defer script.srv.Close()
	prov, sess := toolMocks(s.T())
	sess.On("CallTool", mock.Anything, "create_ticket", mock.Anything).
		Return(`{"data":{"uuid":"t1"}}`, nil)

	status, outcome := run(s.T(), script, prov)
	require.Equal(s.T(), "terminated", status)
	require.Equal(s.T(), "satisfied", outcome)
	// the 2nd request must carry the nudge as a user message
	require.GreaterOrEqual(s.T(), len(script.reqs()), 2)
	require.Contains(s.T(), script.reqs()[1], "You did not call a tool")
}

func (s *RunnerSuite) TestRun_InjectsProjectListAndCountsOnlyRealCreates() {
	// turn 1 -> create_ticket that the backend rejects (error envelope as text);
	// turn 2 -> "done".
	script := newOllamaScript(respCreateTicket, respDone)
	defer script.srv.Close()
	prov, sess := toolMocks(s.T())
	sess.On("CallTool", mock.Anything, "create_ticket", mock.Anything).
		Return(`{"code":"not_found","message":"project not found"}`, nil)

	status, outcome := run(s.T(), script, prov)
	require.Equal(s.T(), "terminated", status)
	require.Equal(s.T(), "failed", outcome) // the create did not actually succeed
	// the resolved project list was injected into the first prompt
	require.GreaterOrEqual(s.T(), len(script.reqs()), 1)
	require.Contains(s.T(), script.reqs()[0], "p1")
	require.Contains(s.T(), script.reqs()[0], "ahh")
}

func (s *RunnerSuite) TestRun_PartialSuccessOnModelError() {
	script := newOllamaScript(respCreateTicket, "__ERROR__")
	defer script.srv.Close()
	prov, sess := toolMocks(s.T())
	sess.On("CallTool", mock.Anything, "create_ticket", mock.Anything).
		Return(`{"data":{"uuid":"t1"}}`, nil)

	status, outcome := run(s.T(), script, prov)
	require.Equal(s.T(), "terminated", status)
	require.Equal(s.T(), "satisfied", outcome) // 1 ticket already created
}

func (s *RunnerSuite) TestRun_FailsWhenNoTicketsCreated() {
	script := newOllamaScript(respProse) // prose forever
	defer script.srv.Close()
	prov, _ := toolMocks(s.T())

	status, outcome := run(s.T(), script, prov)
	require.Equal(s.T(), "terminated", status)
	require.Equal(s.T(), "failed", outcome)
}

func (s *RunnerSuite) TestRun_StripsThinkBlocks() {
	script := newOllamaScript(respCreateTicket, respThinkThenDone)
	defer script.srv.Close()
	prov, sess := toolMocks(s.T())
	sess.On("CallTool", mock.Anything, "create_ticket", mock.Anything).
		Return(`{"data":{"uuid":"t1"}}`, nil)

	r := ollamarunner.NewSessionRunner(prov, ollamatools.New(script.srv.URL), "qwen3:8b", zap.NewNop())
	id, err := r.StartSession(context.Background(), agentsvc.RunAgentTaskRequest{
		Provider: "ollama:qwen3:8b", TaskDescription: "d",
	})
	require.NoError(s.T(), err)
	waitTerminal(s.T(), r, id)

	var msgs []string
	_ = r.StreamSession(context.Background(), id, func(e agentsvc.AgentRunEvent) error {
		msgs = append(msgs, e.Message)
		return nil
	})
	for _, m := range msgs {
		require.NotContains(s.T(), m, "<think>")
	}
}

func (s *RunnerSuite) TestRun_ToolSessionFailure() {
	prov := mocks.NewMockToolProvider(s.T())
	prov.On("Session", mock.Anything).Return(nil, assertErr{})

	r := ollamarunner.NewSessionRunner(prov, ollamatools.New("http://127.0.0.1:0"), "m", zap.NewNop())
	id, err := r.StartSession(context.Background(), agentsvc.RunAgentTaskRequest{
		Provider: "ollama:m", TaskDescription: "x",
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
