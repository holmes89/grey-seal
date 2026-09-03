package agent

import (
	"context"

	"github.com/holmes89/archaea/base"
	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
)

// AgentService orchestrates agentic coding-task runs against external
// providers (Aider, backed by a self-hosted LiteLLM+Ollama stack).
type AgentService interface {
	// RunAgentTask starts a new agent run and returns immediately with its
	// initial state; the run continues asynchronously on the provider's side.
	RunAgentTask(ctx context.Context, req RunAgentTaskRequest) (*greysealv1.AgentRun, error)

	// GetAgentRun returns the current state of a previously started run,
	// refreshing it from the live provider session first.
	GetAgentRun(ctx context.Context, runUUID string) (*greysealv1.AgentRun, error)

	// ListAgentRuns returns runs newest-first, optionally restricted to the
	// given status ("" for no filter). Reads persisted state directly —
	// every in-flight run already has its own watchForCompletion goroutine
	// keeping status current every pollInterval, so a live per-run provider
	// refresh (as GetAgentRun does) isn't needed and would be wasteful
	// across a whole list.
	ListAgentRuns(ctx context.Context, status string, limit uint) ([]*greysealv1.AgentRun, error)

	// StreamAgentRun streams events for a run as they occur. The stream
	// callback is invoked once per event; returning an error aborts
	// streaming. Returns once the run reaches a terminal status or ctx is
	// canceled.
	StreamAgentRun(ctx context.Context, runUUID string, stream func(event AgentRunEvent) error) error
}

// RunAgentTaskRequest describes a new agent run.
type RunAgentTaskRequest struct {
	// Provider: "aider" (only implemented value).
	Provider string
	RepoURL  string
	// GithubToken authorizes cloning and pushing to RepoURL, and opening the
	// PR once the run is satisfied. Never persisted — used only in memory
	// for the run's lifetime.
	GithubToken string
	Branch      string // optional; base branch to check out from — defaults to the repo's default branch
	// PushBranch is the branch the runner must create from Branch and push
	// finished work to. Set by RunAgentTask (deterministic, "agent/<uuid>")
	// before calling SessionRunner.StartSession — passed as a discrete field
	// rather than only embedded in TaskDescription's prose, so a
	// non-agentic runner (e.g. a shell script driving Aider) can act on it
	// directly instead of parsing free text.
	PushBranch      string
	TaskDescription string
	Rubric          string
}

// AgentRunEvent is one relayed event from a running agent session.
type AgentRunEvent struct {
	// Type is the provider event type, e.g. "agent.message", "agent.tool_use",
	// "session.status_idle", "session.status_terminated".
	Type string
	// Message is human-readable text for message/tool-use events.
	Message string
	// Status is populated on status-change events: "running" | "idle" |
	// "terminated" | "error".
	Status string
}

// SessionRunner starts and observes agent sessions against a specific
// provider. Implemented by an adapter outside this package (e.g.
// lib/repo/aiderrunner) so this domain package stays free of any provider
// SDK import.
type SessionRunner interface {
	// StartSession creates a new session for the given task and returns its
	// provider-assigned session ID immediately; the session runs
	// asynchronously on the provider's side.
	StartSession(ctx context.Context, req RunAgentTaskRequest) (sessionID string, err error)

	// GetSessionStatus returns the live status ("running"|"idle"|"terminated")
	// of an existing session, plus its most recent outcome-evaluation result
	// ("" if no outcome has been defined yet; otherwise one of
	// "pending"/"running"/"evaluating"/"satisfied"/"max_iterations_reached"/
	// "failed"/"interrupted").
	GetSessionStatus(ctx context.Context, sessionID string) (status string, outcomeResult string, err error)

	// StreamSession relays events for an existing session. The callback is
	// invoked once per event; returning an error aborts streaming. Returns
	// once the session reaches a terminal status or ctx is canceled.
	StreamSession(ctx context.Context, sessionID string, stream func(event AgentRunEvent) error) error
}

// OpenPullRequestRequest describes a PR to open once an agent run's outcome
// is satisfied.
type OpenPullRequestRequest struct {
	RepoURL string
	Branch  string
	Base    string
	Title   string
	Body    string
	// Token authorizes the PR creation call. Never persisted — held only in
	// memory for the run's lifetime by whoever calls OpenPullRequest.
	Token string
}

// PullRequestOpener opens a pull request directly via the hosting
// provider's API (GitHub REST) — deliberately not via an agent-invoked MCP
// tool, so no MCP server/vault wiring is required.
type PullRequestOpener interface {
	OpenPullRequest(ctx context.Context, req OpenPullRequestRequest) (prURL string, err error)
}

var _ base.Entity = (*greysealv1.AgentRun)(nil)

// AgentRunRepository persists AgentRun state.
type AgentRunRepository interface {
	Create(ctx context.Context, run *greysealv1.AgentRun) error
	Update(ctx context.Context, id string, run *greysealv1.AgentRun) error
	Get(ctx context.Context, id string) (*greysealv1.AgentRun, error)
	List(ctx context.Context, cursor string, limit uint, filter map[string][]any) ([]*greysealv1.AgentRun, error)
}
