// Package managedagents adapts Anthropic's Managed Agents API (Claude
// running its own hosted agent loop and tool-execution sandbox) to the
// agent.SessionRunner interface. It is the only package in this repo that
// imports the Anthropic SDK — domain packages stay free of provider SDK
// imports per this repo's conventions (see the shrikeSearcher adapter in
// cmd/api/main.go for the same pattern applied to shrike).
package managedagents

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/param"

	agentsvc "github.com/holmes89/grey-seal/lib/greyseal/agent"
)

// SessionRunner drives Claude coding-agent runs via Managed Agents.
//
// The Agent and Environment it references must already exist — see
// cmd/setup-agent for the one-time provisioning script. This adapter only
// ever calls Sessions.*, never Agents.New/Environments.New, per the SDK's
// own guidance against creating those in the request path.
type SessionRunner struct {
	client        anthropic.Client
	agentID       string
	environmentID string
}

// NewSessionRunner builds a SessionRunner. API credentials are resolved by
// the SDK's normal precedence (ANTHROPIC_API_KEY, an `ant auth login`
// profile, etc.) — nothing is passed explicitly here.
func NewSessionRunner(agentID, environmentID string) *SessionRunner {
	return &SessionRunner{
		client:        anthropic.NewClient(),
		agentID:       agentID,
		environmentID: environmentID,
	}
}

var _ agentsvc.SessionRunner = (*SessionRunner)(nil)

func (r *SessionRunner) StartSession(ctx context.Context, req agentsvc.RunAgentTaskRequest) (string, error) {
	repoResource := anthropic.BetaManagedAgentsGitHubRepositoryResourceParams{
		Type:               anthropic.BetaManagedAgentsGitHubRepositoryResourceParamsTypeGitHubRepository,
		URL:                req.RepoURL,
		AuthorizationToken: req.GithubToken,
	}
	if req.Branch != "" {
		repoResource.Checkout = anthropic.BetaManagedAgentsGitHubRepositoryResourceParamsCheckoutUnion{
			OfBranch: &anthropic.BetaManagedAgentsBranchCheckoutParam{
				Type: anthropic.BetaManagedAgentsBranchCheckoutTypeBranch,
				Name: req.Branch,
			},
		}
	}

	session, err := r.client.Beta.Sessions.New(ctx, anthropic.BetaSessionNewParams{
		Agent:         anthropic.BetaSessionNewParamsAgentUnion{OfString: param.NewOpt(r.agentID)},
		EnvironmentID: r.environmentID,
		Resources: []anthropic.BetaSessionNewParamsResourceUnion{
			{OfGitHubRepository: &repoResource},
		},
		InitialEvents: []anthropic.BetaSessionNewParamsInitialEventUnion{
			{
				OfUserDefineOutcome: &anthropic.BetaManagedAgentsUserDefineOutcomeEventParams{
					Type:        anthropic.BetaManagedAgentsUserDefineOutcomeEventParamsTypeUserDefineOutcome,
					Description: req.TaskDescription,
					Rubric: anthropic.BetaManagedAgentsUserDefineOutcomeEventParamsRubricUnion{
						OfText: &anthropic.BetaManagedAgentsTextRubricParams{
							Type:    anthropic.BetaManagedAgentsTextRubricParamsTypeText,
							Content: req.Rubric,
						},
					},
				},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("managed agents: create session: %w", err)
	}
	return session.ID, nil
}

// GetSessionStatus returns the live session status, plus the most recent
// outcome-evaluation result ("" if no outcome has been defined yet, else one
// of "pending"/"running"/"evaluating"/"satisfied"/"max_iterations_reached"/
// "failed"/"interrupted" — see BetaManagedAgentsOutcomeEvaluationResource).
// PR creation is driven off this result, not off scanning session output
// files — the SDK itself documents Files-API output scanning as still
// finicky (indexing lag, retry needed), while OutcomeEvaluations is a plain
// field on the session object.
func (r *SessionRunner) GetSessionStatus(ctx context.Context, sessionID string) (status string, outcomeResult string, err error) {
	session, err := r.client.Beta.Sessions.Get(ctx, sessionID, anthropic.BetaSessionGetParams{})
	if err != nil {
		return "", "", fmt.Errorf("managed agents: get session: %w", err)
	}
	if len(session.OutcomeEvaluations) > 0 {
		outcomeResult = session.OutcomeEvaluations[len(session.OutcomeEvaluations)-1].Result
	}
	return string(session.Status), outcomeResult, nil
}

func (r *SessionRunner) StreamSession(ctx context.Context, sessionID string, stream func(event agentsvc.AgentRunEvent) error) error {
	sseStream := r.client.Beta.Sessions.Events.StreamEvents(ctx, sessionID, anthropic.BetaSessionEventStreamParams{})
	defer sseStream.Close() //nolint:errcheck

	for sseStream.Next() {
		event := sseStream.Current()
		out := agentsvc.AgentRunEvent{Type: event.Type}

		switch event.Type {
		case "agent.message":
			for _, block := range event.Content.OfBetaManagedAgentsAgentMessageEventContentArray {
				out.Message += block.Text
			}
		case "agent.tool_use", "agent.mcp_tool_use":
			out.Message = event.Name
		case "session.status_idle":
			out.Status = "idle"
		case "session.status_terminated":
			out.Status = "terminated"
		case "session.error":
			out.Status = "error"
		}

		if err := stream(out); err != nil {
			return err
		}

		if event.Type == "session.status_terminated" {
			return sseStream.Err()
		}
		if event.Type == "session.status_idle" && event.StopReason.Type != "requires_action" {
			return sseStream.Err()
		}
	}
	return sseStream.Err()
}
