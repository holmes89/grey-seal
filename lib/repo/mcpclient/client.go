// Package mcpclient adapts an MCP server reached over Streamable HTTP to
// agent.ToolProvider / agent.ToolSession. It keeps the mcp-go SDK out of
// the domain package (mirrors lib/repo/aiderrunner for agent.SessionRunner).
package mcpclient

import (
	"context"
	"fmt"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"go.uber.org/zap"

	"github.com/holmes89/grey-seal/lib/greyseal/agent"
)

var _ agent.ToolProvider = (*Provider)(nil)

// Provider opens a fresh Streamable HTTP MCP connection per agent run.
// Runs are short, so a per-run connection is simpler and safe.
type Provider struct {
	url    string
	logger *zap.Logger
}

// New returns a Provider for the MCP server's Streamable HTTP endpoint URL
// (e.g. "http://remora:8090/mcp").
func New(url string, logger *zap.Logger) *Provider {
	return &Provider{url: url, logger: logger}
}

// Session dials the MCP server, initializes, and returns a ready session.
func (p *Provider) Session(ctx context.Context) (agent.ToolSession, error) {
	c, err := mcpclient.NewStreamableHttpClient(p.url)
	if err != nil {
		return nil, fmt.Errorf("create MCP client: %w", err)
	}
	if err := c.Start(ctx); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("start MCP client: %w", err)
	}
	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "grey-seal", Version: "1.0.0"}
	if _, err := c.Initialize(ctx, initReq); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("initialize MCP session: %w", err)
	}
	return &session{c: c, logger: p.logger}, nil
}

var _ agent.ToolSession = (*session)(nil)

type session struct {
	c      *mcpclient.Client
	logger *zap.Logger
}

func (s *session) ListTools(ctx context.Context) ([]agent.ToolDef, error) {
	res, err := s.c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, fmt.Errorf("list MCP tools: %w", err)
	}
	defs := make([]agent.ToolDef, 0, len(res.Tools))
	for _, t := range res.Tools {
		defs = append(defs, agent.ToolDef{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: schemaToMap(t.InputSchema),
		})
	}
	return defs, nil
}

func (s *session) CallTool(ctx context.Context, name string, args map[string]any) (string, error) {
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	res, err := s.c.CallTool(ctx, req)
	if err != nil {
		return "", fmt.Errorf("call MCP tool %q: %w", name, err)
	}
	text := textOf(res)
	if res.IsError {
		// Surface the failure to the model rather than aborting the run.
		return "", fmt.Errorf("tool %q failed: %s", name, text)
	}
	return text, nil
}

func (s *session) Close() error {
	return s.c.Close()
}

func schemaToMap(in mcp.ToolInputSchema) map[string]any {
	m := map[string]any{"type": in.Type}
	if in.Properties != nil {
		m["properties"] = in.Properties
	} else {
		m["properties"] = map[string]any{}
	}
	if len(in.Required) > 0 {
		m["required"] = in.Required
	}
	return m
}

func textOf(res *mcp.CallToolResult) string {
	var out string
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			out += tc.Text
		}
	}
	return out
}
