// Package ollamatools is an Ollama /api/chat client that supports
// function/tool calling. It is deliberately separate from lib/repo/ollama
// (the streaming chat-only adapter used by the conversation service): the
// agent tool-loop needs tool definitions in and tool calls out, does not
// stream tokens to a user, and reads whole NDJSON lines that can exceed a
// bufio.Scanner token.
package ollamatools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// Client calls Ollama's /api/chat with tools enabled.
type Client struct {
	host   string
	client *http.Client
}

// New returns a Client for the given Ollama host (e.g. "http://ollama:11434").
func New(host string) *Client {
	if host == "" {
		host = "http://localhost:11434"
	}
	return &Client{host: host, client: &http.Client{}}
}

// Message is one chat message. Role is "system" | "user" | "assistant" |
// "tool". ToolCalls is set on assistant turns that request tools; ToolName
// identifies which call a "tool" message answers.
type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	ToolName  string     `json:"tool_name,omitempty"`
}

// ToolCall is a model request to invoke a tool. Ollama encodes Arguments as
// a JSON object (not an OpenAI-style JSON string).
type ToolCall struct {
	Function struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	} `json:"function"`
}

// Tool is a function definition offered to the model. Parameters is a JSON
// Schema object.
type Tool struct {
	Type     string `json:"type"` // always "function"
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
	} `json:"function"`
}

// NewTool builds a function Tool from a name, description and JSON-Schema
// parameters object.
func NewTool(name, description string, parameters map[string]any) Tool {
	t := Tool{Type: "function"}
	t.Function.Name = name
	t.Function.Description = description
	if parameters == nil {
		parameters = map[string]any{"type": "object", "properties": map[string]any{}}
	}
	t.Function.Parameters = parameters
	return t
}

type chatRequest struct {
	Model    string         `json:"model"`
	Messages []Message      `json:"messages"`
	Tools    []Tool         `json:"tools,omitempty"`
	Stream   bool           `json:"stream"`
	Think    bool           `json:"think"`
	Options  map[string]any `json:"options,omitempty"`
}

// lowTemp keeps the tool-decision loop deterministic — a wandering sampler
// makes small local models skip tool calls.
var lowTemp = map[string]any{"temperature": 0.2}

type chatResponse struct {
	Message    Message `json:"message"`
	Done       bool    `json:"done"`
	DoneReason string  `json:"done_reason"`
}

// Chat sends one non-streaming chat turn. It returns the assistant message
// (Content and/or ToolCalls populated).
func (c *Client) Chat(ctx context.Context, model string, messages []Message, tools []Tool) (Message, error) {
	body, err := json.Marshal(chatRequest{
		Model:    model,
		Messages: messages,
		Tools:    tools,
		Stream:   false,
		Think:    false,
		Options:  lowTemp,
	})
	if err != nil {
		return Message{}, fmt.Errorf("marshal chat request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.host+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return Message{}, fmt.Errorf("build chat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return Message{}, fmt.Errorf("ollama chat request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return Message{}, fmt.Errorf("ollama chat status %d: %s", resp.StatusCode, bytes.TrimSpace(snippet))
	}

	// With stream:false Ollama returns a single JSON object, but read it as
	// NDJSON anyway (and with bufio.Reader, not Scanner — a tool call with a
	// large argument can exceed Scanner's 64KB token).
	r := bufio.NewReader(resp.Body)
	var last chatResponse
	for {
		line, readErr := r.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			var cr chatResponse
			if uErr := json.Unmarshal(line, &cr); uErr != nil {
				return Message{}, fmt.Errorf("decode ollama chat chunk: %w", uErr)
			}
			last = cr
			if cr.Done {
				break
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return Message{}, fmt.Errorf("read ollama chat response: %w", readErr)
		}
	}

	last.Message.Role = "assistant"
	return last.Message, nil
}
