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

Your task instructions will tell you the exact branch name to push your finished,
committed changes to. Push to that branch on the "origin" remote once the rubric is
satisfied. Do not open a pull request yourself — that is handled separately, outside
this session.`

func main() {
	ctx := context.Background()
	client := anthropic.NewClient()

	// No GitHub MCP server/tool here: the agent gets git push access via the
	// github_repository session resource's own token (see StartSession in
	// lib/repo/managedagents/client.go), which is enough to commit and push
	// a branch via bash. Opening the pull request itself is handled outside
	// this session — grey-seal calls the GitHub REST API directly once the
	// run's outcome is satisfied (lib/repo/github), so no MCP server
	// declaration or vault credential is needed.
	agent, err := client.Beta.Agents.New(ctx, anthropic.BetaAgentNewParams{
		Name:   "grey-seal-coding-agent",
		Model:  anthropic.BetaManagedAgentsModelConfigParams{ID: anthropic.BetaManagedAgentsModelClaudeOpus5},
		System: anthropic.String(systemPrompt),
		Tools: []anthropic.BetaAgentNewParamsToolUnion{
			{OfAgentToolset20260401: &anthropic.BetaManagedAgentsAgentToolset20260401Params{
				Type: anthropic.BetaManagedAgentsAgentToolset20260401ParamsTypeAgentToolset20260401,
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
}
