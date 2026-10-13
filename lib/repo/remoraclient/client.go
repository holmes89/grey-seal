// Package remoraclient writes kept chat drafts to Remora through its MCP
// tools (create_design, create_doc, create_ticket). It sits on an
// agent.ToolProvider so the MCP SDK stays inside lib/repo/mcpclient.
package remoraclient

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/holmes89/grey-seal/lib/greyseal/agent"
	"github.com/holmes89/grey-seal/lib/greyseal/conversation"
)

var _ conversation.RemoraWriter = (*Client)(nil)

// Client calls Remora's write tools.
type Client struct {
	tools agent.ToolProvider
}

// New returns a Client that opens a tool session per write.
func New(tools agent.ToolProvider) *Client {
	return &Client{tools: tools}
}

func (c *Client) CreateDesign(ctx context.Context, projectUUID, title, description, body string) (string, error) {
	return c.call(ctx, "create_design", map[string]any{
		"project_uuid": projectUUID,
		"title":        title,
		"description":  description,
		"body":         body,
	})
}

func (c *Client) CreateDiscoveryDoc(ctx context.Context, productUUID, title, slug, description, body string) (string, error) {
	return c.call(ctx, "create_doc", map[string]any{
		"product_uuid": productUUID,
		"type":         "DOC_TYPE_DISCOVERY",
		"title":        title,
		"slug":         slug,
		"description":  description,
		"body":         body,
	})
}

// CreateTicket creates the ticket in DRAFT status: kept tickets are proposals
// for a human to approve, not active work.
func (c *Client) CreateTicket(ctx context.Context, projectUUID, designUUID, title, body string) (string, error) {
	args := map[string]any{
		"project_uuid": projectUUID,
		"title":        title,
		"body":         body,
		"type":         "TICKET_TYPE_FEATURE",
		"draft":        true,
	}
	if designUUID != "" {
		args["design_uuid"] = designUUID
	}
	return c.call(ctx, "create_ticket", args)
}

// call invokes one tool on a fresh session and returns the created entity's
// uuid from the {"data": {"uuid": ...}} response.
func (c *Client) call(ctx context.Context, tool string, args map[string]any) (string, error) {
	sess, err := c.tools.Session(ctx)
	if err != nil {
		return "", fmt.Errorf("connect to remora: %w", err)
	}
	defer sess.Close() //nolint:errcheck // nothing useful to do with a close failure

	text, err := sess.CallTool(ctx, tool, args)
	if err != nil {
		return "", err
	}
	var res struct {
		Data struct {
			UUID string `json:"uuid"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &res); err != nil || res.Data.UUID == "" {
		return "", fmt.Errorf("%s returned no uuid: %.200s", tool, text)
	}
	return res.Data.UUID, nil
}
