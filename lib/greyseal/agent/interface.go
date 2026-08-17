package agent

import (
	"context"

	"github.com/holmes89/archaea/base"
	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
)

// AgentService orchestrates agentic coding-task runs against external
// providers (Claude via Managed Agents; local models are reserved for a
// later phase).
type AgentService interface {
	// RunAgentTask starts a new agent run and returns immediately with its
	// initial state; the run continues asynchronously on the provider's side.
	RunAgentTask(ctx context.Context, req RunAgentTaskRequest) (*greysealv1.AgentRun, error)

	// GetAgentRun returns the current state of a previously started run,
	// refreshing it from the live provider session first.
	GetAgentRun(ctx context.Context, runUUID string) (*greysealv1.AgentRun, error)

	// StreamAgentRun streams events for a run as they occur. The stream
	// callback is invoked once per event; returning an error aborts
	// streaming. Returns once the run reaches a terminal status or ctx is
	// canceled.
	StreamAgentRun(ctx context.Context, runUUID string, stream func(event AgentRunEvent) error) error
}

// RunAgentTaskRequest describes a new agent run.
type RunAgentTaskRequest struct {
	// Provider: "claude" (only implemented value); "ollama:<model>" reserved
	// for a later phase.
	Provider string
	RepoURL  string
	// GithubToken authorizes cloning (and, for Claude, opening a PR against)
	// RepoURL. Never persisted — used only to start the provider session.
	GithubToken     string
	Branch          string // optional; defaults to the repo's default branch
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
// lib/repo/managedagents) so this domain package stays free of any provider
// SDK import.
type SessionRunner interface {
	// StartSession creates a new session for the given task and returns its
	// provider-assigned session ID immediately; the session runs
	// asynchronously on the provider's side.
	StartSession(ctx context.Context, req RunAgentTaskRequest) (sessionID string, err error)

	// GetSessionStatus returns the live status ("running"|"idle"|"terminated")
	// of an existing session, and its PR URL if the agent has reported one.
	GetSessionStatus(ctx context.Context, sessionID string) (status string, prURL string, err error)

	// StreamSession relays events for an existing session. The callback is
	// invoked once per event; returning an error aborts streaming. Returns
	// once the session reaches a terminal status or ctx is canceled.
	StreamSession(ctx context.Context, sessionID string, stream func(event AgentRunEvent) error) error
}

var _ base.Entity = (*greysealv1.AgentRun)(nil)

// AgentRunRepository persists AgentRun state.
type AgentRunRepository interface {
	Create(ctx context.Context, run *greysealv1.AgentRun) error
	Update(ctx context.Context, id string, run *greysealv1.AgentRun) error
	Get(ctx context.Context, id string) (*greysealv1.AgentRun, error)
}
