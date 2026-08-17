// Command setup-agent provisions the Managed Agents Agent and Environment
// that the running grey-seal service references by ID. Per the Anthropic
// SDK's own guidance, agents/environments are versioned, reusable resources
// created once (not per-request) — this is a one-time (or occasional-update)
// operator command, never called from the request path.
//
// Run it manually:
//
//	ANTHROPIC_API_KEY=... go run ./cmd/setup-agent
//
// Then store the printed AGENT_ID/ENVIRONMENT_ID as env vars for the api
// service (see docker-compose.yml).
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/anthropics/anthropic-sdk-go"
)

const systemPrompt = `You are an autonomous coding agent working inside a mounted GitHub repository.

You will be given a task description and a rubric describing what "done" looks like.
Implement the task, run the repository's own build/test commands to verify your work
against the rubric, and iterate until it passes. Match the surrounding code's existing
conventions. Do not add anything beyond what the task asks for.

Once the rubric is satisfied, push your branch and open a pull request describing what
you did and why, including any open questions or low-confidence items a human reviewer
should look at before merging. Do not merge the PR yourself.`

func main() {
	ctx := context.Background()
	client := anthropic.NewClient()

	agent, err := client.Beta.Agents.New(ctx, anthropic.BetaAgentNewParams{
		Name:   "grey-seal-coding-agent",
		Model:  anthropic.BetaManagedAgentsModelConfigParams{ID: anthropic.BetaManagedAgentsModelClaudeOpus5},
		System: anthropic.String(systemPrompt),
		MCPServers: []anthropic.BetaManagedAgentsURLMCPServerParams{
			{
				Type: anthropic.BetaManagedAgentsURLMCPServerParamsTypeURL,
				Name: "github",
				URL:  "https://api.githubcopilot.com/mcp/",
			},
		},
		Tools: []anthropic.BetaAgentNewParamsToolUnion{
			{OfAgentToolset20260401: &anthropic.BetaManagedAgentsAgentToolset20260401Params{
				Type: anthropic.BetaManagedAgentsAgentToolset20260401ParamsTypeAgentToolset20260401,
			}},
			{OfMCPToolset: &anthropic.BetaManagedAgentsMCPToolsetParams{
				Type:          anthropic.BetaManagedAgentsMCPToolsetParamsTypeMCPToolset,
				MCPServerName: "github",
			}},
		},
	})
	if err != nil {
		log.Fatalf("create agent: %v", err)
	}

	environment, err := client.Beta.Environments.New(ctx, anthropic.BetaEnvironmentNewParams{
		Name: "grey-seal-agent-env",
		Config: anthropic.BetaEnvironmentNewParamsConfigUnion{
			OfCloud: &anthropic.BetaCloudConfigParams{
				Networking: anthropic.BetaCloudConfigParamsNetworkingUnion{
					OfUnrestricted: &anthropic.BetaUnrestrictedNetworkParam{},
				},
			},
		},
	})
	if err != nil {
		log.Fatalf("create environment: %v", err)
	}

	fmt.Printf("AGENT_ID=%s\n", agent.ID)
	fmt.Printf("ENVIRONMENT_ID=%s\n", environment.ID)
	fmt.Println("\nAdd these to the api service's environment (docker-compose.yml) and restart it.")
	fmt.Println("\nNote: opening pull requests via the GitHub MCP server requires a vault credential")
	fmt.Println("(GitHub OAuth) attached at session-create time — not yet wired into this Phase 1")
	fmt.Println("script. Until then, sessions can push branches (via the github_repository resource's")
	fmt.Println("own token) but PR creation via the MCP tool will fail until a vault is attached.")
}
