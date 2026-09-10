package ollamarunner_test

import (
	"context"
	"encoding/json"
	"fmt"
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

// planBody builds an Ollama /api/chat response whose assistant content is the
// structured plan JSON the runner unmarshals. tickets are raw JSON objects.
func planBody(projectUUID string, tickets ...string) string {
	planJSON := fmt.Sprintf(`{"project_uuid":%q,"tickets":[%s]}`, projectUUID, strings.Join(tickets, ","))
	content, _ := json.Marshal(planJSON)
	return fmt.Sprintf(`{"message":{"role":"assistant","content":%s},"done":true}`, content)
}

// rawBody wraps an arbitrary assistant content string (e.g. non-JSON, or with
// a <think> block) in an Ollama /api/chat response.
func rawBody(content string) string {
	c, _ := json.Marshal(content)
	return fmt.Sprintf(`{"message":{"role":"assistant","content":%s},"done":true}`, c)
}

func tkt(title string) string {
	return fmt.Sprintf(
		`{"title":%q,"body":"work: %s","type":"TICKET_TYPE_TASK","priority":"PRIORITY_MEDIUM"}`,
		title, title)
}

func toolMocks(t *testing.T) (*mocks.MockToolProvider, *mocks.MockToolSession) {
	sess := mocks.NewMockToolSession(t)
	// the runner resolves the project list itself before planning
	sess.On("CallTool", mock.Anything, "list_projects", mock.Anything).
		Return(`{"projects":[{"uuid":"p1","name":"ahh"}]}`, nil).Maybe()
	sess.On("Close").Return(nil)
	prov := mocks.NewMockToolProvider(t)
	prov.On("Session", mock.Anything).Return(sess, nil)
	return prov, sess
}

// exec runs one session to termination and returns its outcome plus every
// agent.message it emitted.
func exec(t *testing.T, script *ollamaScript, prov agentsvc.ToolProvider) (status, outcome string, msgs []string) {
	t.Helper()
	r := ollamarunner.NewSessionRunner(prov, ollamatools.New(script.srv.URL), "qwen3:8b", zap.NewNop())
	id, err := r.StartSession(context.Background(), agentsvc.RunAgentTaskRequest{
		Provider: "ollama:qwen3:8b", TaskDescription: "Design: add feature X",
	})
	require.NoError(t, err)
	status, outcome = waitTerminal(t, r, id)
	_ = r.StreamSession(context.Background(), id, func(e agentsvc.AgentRunEvent) error {
		if e.Type == "agent.message" {
			msgs = append(msgs, e.Message)
		}
		return nil
	})
	return
}

func run(t *testing.T, script *ollamaScript, prov agentsvc.ToolProvider) (string, string) {
	t.Helper()
	status, outcome, _ := exec(t, script, prov)
	return status, outcome
}

// execReq runs one session with a caller-supplied request (e.g. carrying a
// pinned ProjectUUID) and returns its outcome plus every agent.message.
func execReq(t *testing.T, script *ollamaScript, prov agentsvc.ToolProvider, req agentsvc.RunAgentTaskRequest) (status, outcome string, msgs []string) {
	t.Helper()
	r := ollamarunner.NewSessionRunner(prov, ollamatools.New(script.srv.URL), "qwen3:8b", zap.NewNop())
	id, err := r.StartSession(context.Background(), req)
	require.NoError(t, err)
	status, outcome = waitTerminal(t, r, id)
	_ = r.StreamSession(context.Background(), id, func(e agentsvc.AgentRunEvent) error {
		if e.Type == "agent.message" {
			msgs = append(msgs, e.Message)
		}
		return nil
	})
	return
}

func (s *RunnerSuite) TestRun_PlansThenCreatesAllTickets() {
	script := newOllamaScript(planBody("p1", tkt("A"), tkt("B"), tkt("C")))
	defer script.srv.Close()
	prov, sess := toolMocks(s.T())
	var calls []map[string]any
	sess.On("CallTool", mock.Anything, "create_ticket", mock.Anything).
		Run(func(a mock.Arguments) { calls = append(calls, a.Get(2).(map[string]any)) }).
		Return(`{"data":{"uuid":"11111111-1111-1111-1111-111111111111"}}`, nil)

	status, outcome := run(s.T(), script, prov)
	require.Equal(s.T(), "terminated", status)
	require.Equal(s.T(), "satisfied", outcome)
	require.Len(s.T(), calls, 3)
	for _, c := range calls {
		require.Equal(s.T(), true, c["draft"])
		require.Equal(s.T(), "p1", c["project_uuid"])
		require.NotEmpty(s.T(), c["title"])
	}
	// exactly one model call — no tool loop
	require.Len(s.T(), script.reqs(), 1)
}

func (s *RunnerSuite) TestRun_StripsThinkFromPlan() {
	body := rawBody(`<think>which project?</think>{"project_uuid":"p1","tickets":[` + tkt("A") + `]}`)
	script := newOllamaScript(body)
	defer script.srv.Close()
	prov, sess := toolMocks(s.T())
	sess.On("CallTool", mock.Anything, "create_ticket", mock.Anything).
		Return(`{"data":{"uuid":"11111111-1111-1111-1111-111111111111"}}`, nil)

	status, outcome := run(s.T(), script, prov)
	require.Equal(s.T(), "terminated", status)
	require.Equal(s.T(), "satisfied", outcome)
}

func (s *RunnerSuite) TestRun_NormalizesTypeAndPriority() {
	body := rawBody(`{"project_uuid":"p1","tickets":[{"title":"A","body":"b","type":"bug","priority":"high"}]}`)
	script := newOllamaScript(body)
	defer script.srv.Close()
	prov, sess := toolMocks(s.T())
	var got map[string]any
	sess.On("CallTool", mock.Anything, "create_ticket", mock.Anything).
		Run(func(a mock.Arguments) { got = a.Get(2).(map[string]any) }).
		Return(`{"data":{"uuid":"11111111-1111-1111-1111-111111111111"}}`, nil)

	_, outcome := run(s.T(), script, prov)
	require.Equal(s.T(), "satisfied", outcome)
	require.Equal(s.T(), "TICKET_TYPE_BUG", got["type"])
	require.Equal(s.T(), "PRIORITY_HIGH", got["priority"])
}

func (s *RunnerSuite) TestRun_RetriesOnUnusablePlan() {
	script := newOllamaScript("__ERROR__", planBody("p1", tkt("A")))
	defer script.srv.Close()
	prov, sess := toolMocks(s.T())
	sess.On("CallTool", mock.Anything, "create_ticket", mock.Anything).
		Return(`{"data":{"uuid":"11111111-1111-1111-1111-111111111111"}}`, nil)

	status, outcome := run(s.T(), script, prov)
	require.Equal(s.T(), "terminated", status)
	require.Equal(s.T(), "satisfied", outcome)
	require.GreaterOrEqual(s.T(), len(script.reqs()), 2)
	require.Contains(s.T(), script.reqs()[1], "That was unusable")
}

func (s *RunnerSuite) TestRun_FailsWhenProjectUnmatched() {
	script := newOllamaScript(planBody("not-a-project", tkt("A")))
	defer script.srv.Close()
	prov, _ := toolMocks(s.T())
	// no create_ticket expectation — it must never be called

	status, outcome, msgs := exec(s.T(), script, prov)
	require.Equal(s.T(), "terminated", status)
	require.Equal(s.T(), "failed", outcome)
	require.Contains(s.T(), strings.Join(msgs, "\n"), "no listed project matched")
}

func (s *RunnerSuite) TestRun_UsesPinnedProject() {
	// model reply carries a bogus project_uuid; the pinned one must win, and
	// the model is served the tickets-only schema.
	body := rawBody(`{"project_uuid":"wrong","tickets":[` + tkt("A") + `,` + tkt("B") + `]}`)
	script := newOllamaScript(body)
	defer script.srv.Close()
	prov, sess := toolMocks(s.T())
	var calls []map[string]any
	sess.On("CallTool", mock.Anything, "create_ticket", mock.Anything).
		Run(func(a mock.Arguments) { calls = append(calls, a.Get(2).(map[string]any)) }).
		Return(`{"data":{"uuid":"11111111-1111-1111-1111-111111111111"}}`, nil)

	status, outcome, msgs := execReq(s.T(), script, prov, agentsvc.RunAgentTaskRequest{
		Provider: "ollama:qwen3:8b", TaskDescription: "Design: add feature X", ProjectUUID: "p1",
	})
	require.Equal(s.T(), "terminated", status)
	require.Equal(s.T(), "satisfied", outcome)
	require.Len(s.T(), calls, 2)
	for _, c := range calls {
		require.Equal(s.T(), "p1", c["project_uuid"])
		require.Equal(s.T(), true, c["draft"])
	}
	require.Contains(s.T(), strings.Join(msgs, "\n"), `drafting tickets for project "ahh"`)
}

func (s *RunnerSuite) TestRun_FailsWhenPinnedProjectUnknown() {
	script := newOllamaScript(planBody("p1", tkt("A")))
	defer script.srv.Close()
	prov, _ := toolMocks(s.T()) // list_projects returns only p1/"ahh"

	status, outcome, msgs := execReq(s.T(), script, prov, agentsvc.RunAgentTaskRequest{
		Provider: "ollama:qwen3:8b", TaskDescription: "d", ProjectUUID: "ghost",
	})
	require.Equal(s.T(), "terminated", status)
	require.Equal(s.T(), "failed", outcome)
	require.Contains(s.T(), strings.Join(msgs, "\n"), "pinned project is not in Rabbit")
}

func (s *RunnerSuite) TestRun_MatchesProjectByName() {
	script := newOllamaScript(planBody("ahh", tkt("A")))
	defer script.srv.Close()
	prov, sess := toolMocks(s.T())
	var got map[string]any
	sess.On("CallTool", mock.Anything, "create_ticket", mock.Anything).
		Run(func(a mock.Arguments) { got = a.Get(2).(map[string]any) }).
		Return(`{"data":{"uuid":"11111111-1111-1111-1111-111111111111"}}`, nil)

	_, outcome := run(s.T(), script, prov)
	require.Equal(s.T(), "satisfied", outcome)
	require.Equal(s.T(), "p1", got["project_uuid"]) // name "ahh" resolved to its uuid
}

func (s *RunnerSuite) TestRun_FailsWhenPlanEmpty() {
	script := newOllamaScript(planBody("p1")) // valid project, zero tickets
	defer script.srv.Close()
	prov, _ := toolMocks(s.T())

	status, outcome := run(s.T(), script, prov)
	require.Equal(s.T(), "terminated", status)
	require.Equal(s.T(), "failed", outcome)
}

func (s *RunnerSuite) TestRun_CountsOnlyRealCreates() {
	script := newOllamaScript(planBody("p1", tkt("A"), tkt("B")))
	defer script.srv.Close()
	prov, sess := toolMocks(s.T())
	// first create succeeds, second comes back as a Connect error envelope
	sess.On("CallTool", mock.Anything, "create_ticket", mock.Anything).
		Return(`{"data":{"uuid":"11111111-1111-1111-1111-111111111111"}}`, nil).Once()
	sess.On("CallTool", mock.Anything, "create_ticket", mock.Anything).
		Return(`{"code":"not_found","message":"project not found"}`, nil).Once()

	status, outcome := run(s.T(), script, prov)
	require.Equal(s.T(), "terminated", status)
	require.Equal(s.T(), "satisfied", outcome) // 1 real create is still a partial success
}

func (s *RunnerSuite) TestRun_FailsWhenAllCreatesRejected() {
	script := newOllamaScript(planBody("p1", tkt("A")))
	defer script.srv.Close()
	prov, sess := toolMocks(s.T())
	sess.On("CallTool", mock.Anything, "create_ticket", mock.Anything).
		Return(`{"code":"invalid_argument","message":"nope"}`, nil)

	status, outcome := run(s.T(), script, prov)
	require.Equal(s.T(), "terminated", status)
	require.Equal(s.T(), "failed", outcome)
}

func (s *RunnerSuite) TestRun_EmitsProgressHeartbeats() {
	script := newOllamaScript(planBody("p1", tkt("A")))
	defer script.srv.Close()
	prov, sess := toolMocks(s.T())
	sess.On("CallTool", mock.Anything, "create_ticket", mock.Anything).
		Return(`{"data":{"uuid":"11111111-1111-1111-1111-111111111111"}}`, nil)

	_, _, msgs := exec(s.T(), script, prov)
	joined := strings.Join(msgs, "\n")
	require.Contains(s.T(), joined, "connecting to the ticket tools")
	require.Contains(s.T(), joined, "resolving the project list")
	require.Contains(s.T(), joined, "asking qwen")
	require.Contains(s.T(), joined, "planning 1 ticket")
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

func (s *RunnerSuite) TestRun_FailsWhenNoProjectList() {
	script := newOllamaScript(planBody("p1", tkt("A")))
	defer script.srv.Close()
	sess := mocks.NewMockToolSession(s.T())
	sess.On("CallTool", mock.Anything, "list_projects", mock.Anything).
		Return(``, assertErr{})
	sess.On("Close").Return(nil)
	prov := mocks.NewMockToolProvider(s.T())
	prov.On("Session", mock.Anything).Return(sess, nil)

	status, outcome := run(s.T(), script, prov)
	require.Equal(s.T(), "terminated", status)
	require.Equal(s.T(), "failed", outcome)
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
