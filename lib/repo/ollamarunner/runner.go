// Package ollamarunner implements agent.SessionRunner as a single
// structured-output planning call against a local Ollama model, with the
// ticket tools supplied over MCP (agent.ToolProvider). It is the
// "ollama:<model>" agent provider: no Docker, no repo, no PR — it decomposes
// a software design into DRAFT rabbit tickets.
//
// The model is asked once, constrained by an Ollama `format` JSON schema, for
// the plan; the runner then calls the create_ticket MCP tool for each item
// itself. Small local models are weak at multi-turn tool-calling but reliable
// at schema-constrained JSON, so this removes that failure class. One
// corrective retry covers a bad plan.
//
// If RunAgentTaskRequest.ProjectUUID is set the runner pins that project and
// the model only returns {tickets[]}; otherwise the model also picks a
// project by name and the runner validates its choice against list_projects.
package ollamarunner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
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
	// Per model call. The planning call is CPU-bound on a local host and can
	// take a few minutes; one that blows past this is treated as wedged and
	// the run stops rather than hanging until defaultRunTimeout.
	defaultTurnTimeout = 10 * time.Minute
	// Whole-run ceiling (planning call + one retry + the create_ticket calls).
	defaultRunTimeout   = 25 * time.Minute
	resolveToolsTimeout = 30 * time.Second
	streamPollInterval  = 300 * time.Millisecond
	// The planning call plus at most one corrective retry.
	maxPlanAttempts = 2
)

// planPrompt steers the model to emit the whole plan as JSON in one shot. It
// is deliberately imperative: descriptive phrasing makes small local models
// answer with prose instead of structured output.
const planPrompt = `You decompose a software design into DRAFT tickets in a planning system called Rabbit.

Output ONLY JSON matching the schema — no prose, no code fences.

project_uuid MUST be copied verbatim from the Projects list below: choose the row whose name best matches the design. If no listed project fits, set project_uuid to "" and return an empty tickets list.

Break the design into small, independently shippable units of work — one ticket each:
- title: a short imperative summary.
- body: markdown saying what to do and why.
- type: TICKET_TYPE_TASK, unless the unit is net-new user-facing capability (TICKET_TYPE_FEATURE) or fixing a defect (TICKET_TYPE_BUG).
- priority: PRIORITY_MEDIUM, unless the unit is foundational or blocks other work (PRIORITY_HIGH) or is a nice-to-have (PRIORITY_LOW).`

// planSchema is the Ollama `format` constraint for the planning call.
var planSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "project_uuid": { "type": "string" },
    "tickets": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "title": { "type": "string" },
          "body": { "type": "string" },
          "type": { "type": "string", "enum": ["TICKET_TYPE_FEATURE", "TICKET_TYPE_BUG", "TICKET_TYPE_TASK"] },
          "priority": { "type": "string", "enum": ["PRIORITY_LOW", "PRIORITY_MEDIUM", "PRIORITY_HIGH"] }
        },
        "required": ["title", "body", "type", "priority"]
      }
    }
  },
  "required": ["project_uuid", "tickets"]
}`)

// fixedProjectPrompt is used when the caller pinned the project — the model
// only breaks the design into tickets and never picks a project.
const fixedProjectPrompt = `You decompose a software design into DRAFT tickets in a planning system called Rabbit.

The Rabbit project is already chosen — do NOT pick one. Output ONLY JSON matching the schema (a "tickets" array) — no prose, no code fences.

Break the design into small, independently shippable units of work — one ticket each:
- title: a short imperative summary.
- body: markdown saying what to do and why.
- type: TICKET_TYPE_TASK, unless the unit is net-new user-facing capability (TICKET_TYPE_FEATURE) or fixing a defect (TICKET_TYPE_BUG).
- priority: PRIORITY_MEDIUM, unless the unit is foundational or blocks other work (PRIORITY_HIGH) or is a nice-to-have (PRIORITY_LOW).`

// ticketsSchema is the Ollama `format` constraint for a pinned-project run:
// planSchema without the project_uuid field.
var ticketsSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "tickets": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "title": { "type": "string" },
          "body": { "type": "string" },
          "type": { "type": "string", "enum": ["TICKET_TYPE_FEATURE", "TICKET_TYPE_BUG", "TICKET_TYPE_TASK"] },
          "priority": { "type": "string", "enum": ["PRIORITY_LOW", "PRIORITY_MEDIUM", "PRIORITY_HIGH"] }
        },
        "required": ["title", "body", "type", "priority"]
      }
    }
  },
  "required": ["tickets"]
}`)

// SessionRunner drives ollama:<model> agent runs.
type SessionRunner struct {
	tools       agentsvc.ToolProvider
	chat        *ollamatools.Client
	model       string
	logger      *zap.Logger
	runTimeout  time.Duration
	turnTimeout time.Duration

	mu       sync.Mutex
	sessions map[string]*session
}

type session struct {
	status  string // "running" | "terminated"
	outcome string // "" | "satisfied" | "failed"
	events  []agentsvc.AgentRunEvent
	done    chan struct{}
}

// Option tweaks a SessionRunner (mainly for tests).
type Option func(*SessionRunner)

// WithTimeouts overrides the per-call and whole-run timeouts.
func WithTimeouts(turn, run time.Duration) Option {
	return func(r *SessionRunner) { r.turnTimeout, r.runTimeout = turn, run }
}

// NewSessionRunner builds a runner. model is the Ollama model name (the part
// after "ollama:" in the provider string).
func NewSessionRunner(tools agentsvc.ToolProvider, chat *ollamatools.Client, model string, logger *zap.Logger, opts ...Option) *SessionRunner {
	r := &SessionRunner{
		tools:       tools,
		chat:        chat,
		model:       model,
		logger:      logger,
		runTimeout:  defaultRunTimeout,
		turnTimeout: defaultTurnTimeout,
		sessions:    make(map[string]*session),
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

// StartSession kicks off the agent run in a goroutine and returns a synthetic
// session ID immediately.
func (r *SessionRunner) StartSession(_ context.Context, req agentsvc.RunAgentTaskRequest) (string, error) {
	if strings.TrimSpace(req.TaskDescription) == "" {
		return "", fmt.Errorf("ollamarunner: task_description is required")
	}
	id := uuid.New().String()
	s := &session{status: "running", done: make(chan struct{})}
	r.mu.Lock()
	r.sessions[id] = s
	r.mu.Unlock()

	go r.run(id, req.TaskDescription, strings.TrimSpace(req.ProjectUUID))
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

type planTicket struct {
	Title    string `json:"title"`
	Body     string `json:"body"`
	Type     string `json:"type"`
	Priority string `json:"priority"`
}

type plan struct {
	ProjectUUID string       `json:"project_uuid"`
	Tickets     []planTicket `json:"tickets"`
}

func (r *SessionRunner) run(sessionID, design, fixedProject string) {
	ctx, cancel := context.WithTimeout(context.Background(), r.runTimeout)
	defer cancel()

	say := func(msg string) {
		r.emit(sessionID, agentsvc.AgentRunEvent{Type: "agent.message", Message: msg})
	}
	fail := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		r.logger.Warn("ollama agent run failed", zap.String("session_id", sessionID), zap.String("reason", msg))
		say(msg)
		r.finish(sessionID, "failed")
	}

	say("starting — connecting to the ticket tools")
	ts, err := r.tools.Session(ctx)
	if err != nil {
		fail("could not open tool session: %v", err)
		return
	}
	defer ts.Close() //nolint:errcheck

	// Resolve the project list ourselves — the deterministic path needs a
	// uuid up front, and even with a caller-pinned project we still validate
	// it against Rabbit.
	say("resolving the project list…")
	projectList, byUUID, byName := r.resolveProjects(ctx, ts)
	if projectList == "" {
		fail("could not load the project list — cannot draft tickets")
		return
	}

	// Two modes: the caller pins the project (the UI knows which one the
	// design belongs to), or the model matches one by name from the design.
	var sysPrompt, userMsg string
	var schema json.RawMessage
	var parsePlan func(content string) (plan, string) // -> (plan, reason-if-unusable)

	if fixedProject != "" {
		name, ok := byUUID[fixedProject]
		if !ok {
			fail("the pinned project is not in Rabbit (%s)", fixedProject)
			return
		}
		say(fmt.Sprintf("drafting tickets for project %q", name))
		sysPrompt = fixedProjectPrompt
		schema = ticketsSchema
		userMsg = design + fmt.Sprintf("\n\nEvery ticket belongs to project %q.", name)
		parsePlan = func(content string) (plan, string) {
			var got struct {
				Tickets []planTicket `json:"tickets"`
			}
			if json.Unmarshal([]byte(stripThink(content)), &got) != nil {
				return plan{}, "the model did not return valid JSON"
			}
			if len(got.Tickets) == 0 {
				return plan{}, "the plan contained no tickets"
			}
			return plan{ProjectUUID: fixedProject, Tickets: got.Tickets}, ""
		}
	} else {
		say(fmt.Sprintf("using %d project(s)", len(byUUID)))
		sysPrompt = planPrompt
		schema = planSchema
		userMsg = design + "\n\nProjects (name  <uuid>):\n" + projectList
		parsePlan = func(content string) (plan, string) {
			var cand plan
			if json.Unmarshal([]byte(stripThink(content)), &cand) != nil {
				return plan{}, "the model did not return valid JSON"
			}
			resolved, matched := matchProject(cand.ProjectUUID, byUUID, byName)
			if !matched {
				return plan{}, "no listed project matched the design"
			}
			if len(cand.Tickets) == 0 {
				return plan{}, "the plan contained no tickets"
			}
			cand.ProjectUUID = resolved
			return cand, ""
		}
	}

	messages := []ollamatools.Message{
		{Role: "system", Content: sysPrompt},
		{Role: "user", Content: userMsg},
	}

	var p plan
	var lastReason string
	planned := false
	for attempt := 1; attempt <= maxPlanAttempts && !planned; attempt++ {
		hb := fmt.Sprintf("asking %s to work on the design…", r.model)
		if attempt == 1 {
			hb += " — this can take a few minutes on this host"
		}
		say(hb)

		turnCtx, cancelTurn := context.WithTimeout(ctx, r.turnTimeout)
		msg, cerr := r.chat.ChatJSON(turnCtx, r.model, messages, schema)
		cancelTurn()

		if errors.Is(cerr, context.DeadlineExceeded) {
			fail("model call timed out after %s — stopping", r.turnTimeout)
			return
		}
		if cerr != nil {
			lastReason = fmt.Sprintf("model call failed: %v", cerr)
			r.logger.Warn("ollama agent: plan call failed", zap.String("session_id", sessionID), zap.Error(cerr))
		} else {
			var reason string
			p, reason = parsePlan(msg.Content)
			if reason == "" {
				planned = true
			} else {
				lastReason = reason
				if attempt < maxPlanAttempts {
					messages = append(messages, ollamatools.Message{Role: "assistant", Content: msg.Content})
				}
			}
		}

		if !planned && attempt < maxPlanAttempts {
			say(fmt.Sprintf("the plan was unusable (%s) — asking once more", lastReason))
			hint := fmt.Sprintf("That was unusable: %s. Return ONLY JSON matching the schema, with at least one ticket.", lastReason)
			if fixedProject == "" {
				hint = fmt.Sprintf(
					"That was unusable: %s. Return ONLY JSON matching the schema. project_uuid MUST be exactly one of: %s. Include at least one ticket.",
					lastReason, strings.Join(sortedKeys(byUUID), ", "))
			}
			messages = append(messages, ollamatools.Message{Role: "user", Content: hint})
		}
	}

	if !planned {
		if strings.Contains(lastReason, "no listed project") {
			say("no listed project matched the design — nothing created")
		} else {
			say(fmt.Sprintf("could not get a usable plan from the model (%s)", lastReason))
		}
		r.finish(sessionID, "failed")
		return
	}

	say(fmt.Sprintf("planning %d ticket(s)…", len(p.Tickets)))
	created := 0
	for _, t := range p.Tickets {
		args := map[string]any{
			"project_uuid": p.ProjectUUID,
			"title":        t.Title,
			"body":         t.Body,
			"type":         normType(t.Type),
			"priority":     normPriority(t.Priority),
			"draft":        true,
		}
		argsJSON, _ := json.Marshal(args)
		r.emit(sessionID, agentsvc.AgentRunEvent{
			Type:    "agent.tool_use",
			Message: fmt.Sprintf("create_ticket(%s)", truncate(string(argsJSON), 300)),
		})

		result, callErr := ts.CallTool(ctx, "create_ticket", args)
		content := result
		if callErr != nil {
			content = "ERROR: " + callErr.Error()
		} else if ticketCreated(result) {
			created++
		}
		r.emit(sessionID, agentsvc.AgentRunEvent{
			Type:    "agent.tool_use",
			Message: fmt.Sprintf("create_ticket → %s", truncate(content, 300)),
		})
	}

	if created > 0 {
		say(fmt.Sprintf("done — created %d draft ticket(s)", created))
		r.finish(sessionID, "satisfied")
		return
	}
	say("no draft tickets were created")
	r.finish(sessionID, "failed")
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

// resolveProjects calls list_projects (no args) and returns a newline list of
// "name  <uuid>" rows plus name→uuid and uuid→name lookup maps, or ("", nil,
// nil) if the call failed or returned nothing.
func (r *SessionRunner) resolveProjects(ctx context.Context, ts agentsvc.ToolSession) (list string, byUUID, byName map[string]string) {
	rpCtx, cancel := context.WithTimeout(ctx, resolveToolsTimeout)
	defer cancel()
	raw, err := ts.CallTool(rpCtx, "list_projects", map[string]any{})
	if err != nil {
		r.logger.Warn("ollama agent: list_projects failed", zap.Error(err))
		return "", nil, nil
	}
	var resp struct {
		Projects []struct {
			UUID string `json:"uuid"`
			Name string `json:"name"`
		} `json:"projects"`
	}
	if json.Unmarshal([]byte(raw), &resp) != nil || len(resp.Projects) == 0 {
		return "", nil, nil
	}
	byUUID = map[string]string{}
	byName = map[string]string{}
	var b strings.Builder
	for _, pr := range resp.Projects {
		if pr.UUID == "" {
			continue
		}
		fmt.Fprintf(&b, "- %s  %s\n", pr.Name, pr.UUID)
		byUUID[pr.UUID] = pr.Name
		if n := strings.ToLower(strings.TrimSpace(pr.Name)); n != "" {
			byName[n] = pr.UUID
		}
	}
	if len(byUUID) == 0 {
		return "", nil, nil
	}
	return b.String(), byUUID, byName
}

// matchProject resolves what the model put in project_uuid to a real uuid: an
// exact uuid match, or a case-insensitive project-name match (small models
// sometimes return the name). Returns ("", false) for an empty or unknown
// value.
func matchProject(v string, byUUID, byName map[string]string) (string, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", false
	}
	if _, ok := byUUID[v]; ok {
		return v, true
	}
	if u, ok := byName[strings.ToLower(v)]; ok {
		return u, true
	}
	return "", false
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// normType maps a model-supplied ticket type to a valid enum, defaulting to
// TICKET_TYPE_TASK.
func normType(s string) string {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "TICKET_TYPE_FEATURE", "FEATURE":
		return "TICKET_TYPE_FEATURE"
	case "TICKET_TYPE_BUG", "BUG":
		return "TICKET_TYPE_BUG"
	default:
		return "TICKET_TYPE_TASK"
	}
}

// normPriority maps a model-supplied priority to a valid enum, defaulting to
// PRIORITY_MEDIUM.
func normPriority(s string) string {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "PRIORITY_LOW", "LOW":
		return "PRIORITY_LOW"
	case "PRIORITY_HIGH", "HIGH":
		return "PRIORITY_HIGH"
	default:
		return "PRIORITY_MEDIUM"
	}
}

// ticketCreated reports whether a create_ticket result is a real success (a
// data object with a uuid) rather than a Connect error envelope returned as
// text by the backend.
func ticketCreated(result string) bool {
	var r struct {
		Data struct {
			UUID string `json:"uuid"`
		} `json:"data"`
		Code string `json:"code"`
	}
	if json.Unmarshal([]byte(result), &r) != nil {
		return false
	}
	return r.Code == "" && r.Data.UUID != ""
}

var thinkBlock = regexp.MustCompile(`(?is)<think>.*?</think>`)

// stripThink removes <think>…</think> reasoning blocks — qwen3 can emit them
// even with think:false on some Ollama builds — so they stay out of the JSON
// the runner has to parse.
func stripThink(s string) string {
	return strings.TrimSpace(thinkBlock.ReplaceAllString(s, ""))
}
