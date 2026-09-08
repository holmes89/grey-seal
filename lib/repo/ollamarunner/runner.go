// Package ollamarunner implements agent.SessionRunner as an in-process
// tool-calling loop against a local Ollama model, with tools supplied over
// MCP (agent.ToolProvider). It is the "ollama:<model>" agent provider: no
// Docker, no repo, no PR — it drives an orchestration task such as
// decomposing a design into draft tickets.
package ollamarunner

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	agentsvc "github.com/holmes89/grey-seal/lib/greyseal/agent"
	"github.com/holmes89/grey-seal/lib/repo/ollamatools"
)

var _ agentsvc.SessionRunner = (*SessionRunner)(nil)

const (
	defaultMaxIterations = 16
	defaultRunTimeout    = 10 * time.Minute
	streamPollInterval   = 300 * time.Millisecond
)

// systemPrompt steers the model through the design→draft-tickets task. It is
// deliberately imperative: a descriptive version made qwen3:8b answer with
// prose and never emit tool_calls.
const systemPrompt = `You turn a software design into a set of DRAFT tickets in a planning system called Rabbit, using ONLY the tools provided.

Your FIRST action MUST be to call list_projects. Never write prose before or instead of a tool call.

Then:
- Pick the project_uuid whose name best matches the design. If none fits, reply with exactly NO_PROJECT and call no tools.
- Call list_tickets for that project so you do not duplicate tickets that already exist.
- Break the design into small, independent units of work. For each one call create_ticket with:
  project_uuid (from list_projects); a short imperative title; a markdown body saying what to do and why;
  type (one of TICKET_TYPE_FEATURE, TICKET_TYPE_BUG, TICKET_TYPE_TASK);
  priority (one of PRIORITY_LOW, PRIORITY_MEDIUM, PRIORITY_HIGH);
  and draft set to true — always.
- When every unit has a ticket, reply with ONE short summary line listing the titles you created, and call no more tools. Only then may you write prose.`

// nudge is appended once if a turn produces no tool call and nothing has been
// created yet, so a single stray prose turn cannot end the run.
const nudge = `You did not call a tool. Call list_projects now — or, if no project matches the design, reply with exactly NO_PROJECT.`

// allowedTools is the subset of the MCP server's tools this provider exposes
// to the model.
var allowedTools = map[string]bool{
	"list_projects": true,
	"list_tickets":  true,
	"get_ticket":    true,
	"create_ticket": true,
}

// SessionRunner drives ollama:<model> agent runs.
type SessionRunner struct {
	tools      agentsvc.ToolProvider
	chat       *ollamatools.Client
	model      string
	logger     *zap.Logger
	maxIter    int
	runTimeout time.Duration
	allowed    map[string]bool

	mu       sync.Mutex
	sessions map[string]*session
}

type session struct {
	status  string // "running" | "terminated"
	outcome string // "" | "satisfied" | "failed"
	events  []agentsvc.AgentRunEvent
	done    chan struct{}
}

// NewSessionRunner builds a runner. model is the Ollama model name (the part
// after "ollama:" in the provider string).
func NewSessionRunner(tools agentsvc.ToolProvider, chat *ollamatools.Client, model string, logger *zap.Logger) *SessionRunner {
	return &SessionRunner{
		tools:      tools,
		chat:       chat,
		model:      model,
		logger:     logger,
		maxIter:    defaultMaxIterations,
		runTimeout: defaultRunTimeout,
		allowed:    allowedTools,
		sessions:   make(map[string]*session),
	}
}

// StartSession kicks off the agent loop in a goroutine and returns a
// synthetic session ID immediately.
func (r *SessionRunner) StartSession(_ context.Context, req agentsvc.RunAgentTaskRequest) (string, error) {
	if strings.TrimSpace(req.TaskDescription) == "" {
		return "", fmt.Errorf("ollamarunner: task_description is required")
	}
	id := uuid.New().String()
	s := &session{status: "running", done: make(chan struct{})}
	r.mu.Lock()
	r.sessions[id] = s
	r.mu.Unlock()

	go r.run(id, req.TaskDescription)
	return id, nil
}

// GetSessionStatus reports the run's state from the in-memory session map.
func (r *SessionRunner) GetSessionStatus(_ context.Context, sessionID string) (string, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[sessionID]
	if !ok {
		return "", "", fmt.Errorf("ollamarunner: unknown session %q", sessionID)
	}
	return s.status, s.outcome, nil
}

// StreamSession replays buffered events, tails new ones, and emits a final
// session.status_terminated once the run ends.
func (r *SessionRunner) StreamSession(ctx context.Context, sessionID string, stream func(agentsvc.AgentRunEvent) error) error {
	r.mu.Lock()
	s, ok := r.sessions[sessionID]
	r.mu.Unlock()
	if !ok {
		return fmt.Errorf("ollamarunner: unknown session %q", sessionID)
	}

	cursor := 0
	for {
		r.mu.Lock()
		pending := append([]agentsvc.AgentRunEvent(nil), s.events[cursor:]...)
		cursor = len(s.events)
		status, outcome := s.status, s.outcome
		r.mu.Unlock()

		for _, e := range pending {
			if err := stream(e); err != nil {
				return err
			}
		}

		if status == "terminated" {
			return stream(agentsvc.AgentRunEvent{Type: "session.status_terminated", Status: outcome})
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.done:
			// loop once more to flush any final events, then terminate
		case <-time.After(streamPollInterval):
		}
	}
}

func (r *SessionRunner) run(sessionID, design string) {
	ctx, cancel := context.WithTimeout(context.Background(), r.runTimeout)
	defer cancel()

	fail := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		r.logger.Warn("ollama agent run failed", zap.String("session_id", sessionID), zap.String("reason", msg))
		r.emit(sessionID, agentsvc.AgentRunEvent{Type: "agent.message", Message: msg})
		r.finish(sessionID, "failed")
	}

	ts, err := r.tools.Session(ctx)
	if err != nil {
		fail("could not open tool session: %v", err)
		return
	}
	defer ts.Close() //nolint:errcheck

	defs, err := ts.ListTools(ctx)
	if err != nil {
		fail("could not list tools: %v", err)
		return
	}
	tools := make([]ollamatools.Tool, 0, len(defs))
	for _, d := range defs {
		if !r.allowed[d.Name] {
			continue
		}
		tools = append(tools, ollamatools.NewTool(d.Name, d.Description, d.InputSchema))
	}
	if len(tools) == 0 {
		fail("the MCP server exposes none of the expected ticket tools")
		return
	}

	messages := []ollamatools.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: design},
	}

	created := 0
	nudged := false

	finishByCount := func() {
		if created > 0 {
			r.emit(sessionID, agentsvc.AgentRunEvent{
				Type:    "agent.message",
				Message: fmt.Sprintf("done — created %d draft ticket(s)", created),
			})
			r.finish(sessionID, "satisfied")
			return
		}
		r.emit(sessionID, agentsvc.AgentRunEvent{
			Type:    "agent.message",
			Message: "no draft tickets were created",
		})
		r.finish(sessionID, "failed")
	}

	for i := 0; i < r.maxIter; i++ {
		msg, err := r.chat.Chat(ctx, r.model, messages, tools)
		if err != nil {
			fail("model call failed: %v", err)
			return
		}
		text := stripThink(msg.Content)
		msg.Content = text
		messages = append(messages, msg)

		if len(msg.ToolCalls) == 0 {
			if strings.TrimSpace(text) != "" {
				r.emit(sessionID, agentsvc.AgentRunEvent{Type: "agent.message", Message: text})
			}
			if strings.EqualFold(strings.TrimSpace(text), "NO_PROJECT") {
				r.emit(sessionID, agentsvc.AgentRunEvent{
					Type:    "agent.message",
					Message: "no matching project — nothing created",
				})
				r.finish(sessionID, "failed")
				return
			}
			// A single stray prose turn before anything is created gets one
			// explicit nudge rather than silently ending the run.
			if created == 0 && !nudged {
				nudged = true
				messages = append(messages, ollamatools.Message{Role: "user", Content: nudge})
				r.emit(sessionID, agentsvc.AgentRunEvent{
					Type:    "agent.message",
					Message: "model did not call a tool — nudging it to start",
				})
				continue
			}
			finishByCount()
			return
		}

		for _, tc := range msg.ToolCalls {
			name := tc.Function.Name
			argsJSON, _ := json.Marshal(tc.Function.Arguments)
			r.emit(sessionID, agentsvc.AgentRunEvent{
				Type:    "agent.tool_use",
				Message: fmt.Sprintf("%s(%s)", name, truncate(string(argsJSON), 300)),
			})

			result, callErr := ts.CallTool(ctx, name, tc.Function.Arguments)
			content := result
			if callErr != nil {
				content = "ERROR: " + callErr.Error()
			} else if name == "create_ticket" {
				created++
			}
			r.emit(sessionID, agentsvc.AgentRunEvent{
				Type:    "agent.tool_use",
				Message: fmt.Sprintf("%s → %s", name, truncate(content, 300)),
			})
			messages = append(messages, ollamatools.Message{
				Role:     "tool",
				ToolName: name,
				Content:  content,
			})
		}
	}

	r.emit(sessionID, agentsvc.AgentRunEvent{
		Type:    "agent.message",
		Message: fmt.Sprintf("reached the %d-iteration cap; created %d draft ticket(s) so far", r.maxIter, created),
	})
	finishByCount()
}

func (r *SessionRunner) emit(sessionID string, e agentsvc.AgentRunEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.sessions[sessionID]; ok {
		s.events = append(s.events, e)
	}
}

func (r *SessionRunner) finish(sessionID, outcome string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[sessionID]
	if !ok {
		return
	}
	s.status = "terminated"
	s.outcome = outcome
	select {
	case <-s.done:
	default:
		close(s.done)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

var thinkBlock = regexp.MustCompile(`(?is)<think>.*?</think>`)

// stripThink removes <think>…</think> reasoning blocks — qwen3 can emit them
// even with think:false on some Ollama builds — so they stay out of the
// transcript and the event stream.
func stripThink(s string) string {
	return strings.TrimSpace(thinkBlock.ReplaceAllString(s, ""))
}
