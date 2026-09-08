package mcpclient_test

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/holmes89/grey-seal/lib/repo/mcpclient"
)

func TestProvider_ListAndCall(t *testing.T) {
	s := mcpserver.NewMCPServer("test", "1.0.0")
	tool := mcp.NewTool("echo",
		mcp.WithDescription("echo back the message"),
		mcp.WithString("message", mcp.Required(), mcp.Description("text to echo")),
	)
	s.AddTool(tool, func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("echo: " + req.GetArguments()["message"].(string)), nil
	})

	ts := mcpserver.NewTestStreamableHTTPServer(s)
	defer ts.Close()

	p := mcpclient.New(ts.URL+"/mcp", zap.NewNop())
	sess, err := p.Session(context.Background())
	require.NoError(t, err)
	defer sess.Close() //nolint:errcheck

	tools, err := sess.ListTools(context.Background())
	require.NoError(t, err)
	require.Len(t, tools, 1)
	require.Equal(t, "echo", tools[0].Name)
	require.Equal(t, "object", tools[0].InputSchema["type"])
	require.Contains(t, tools[0].InputSchema["required"], "message")

	out, err := sess.CallTool(context.Background(), "echo", map[string]any{"message": "hi"})
	require.NoError(t, err)
	require.Equal(t, "echo: hi", out)
}
