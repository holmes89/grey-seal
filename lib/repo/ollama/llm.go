package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/holmes89/grey-seal/lib/greyseal/conversation"
)

// LLM calls the Ollama /api/chat endpoint with streaming support.
type LLM struct {
	host   string
	model  string
	client *http.Client
}

// NewLLM creates an LLM using OLLAMA_HOST and OLLAMA_CHAT_MODEL env vars.
func NewLLM() *LLM {
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		host = "http://localhost:11434"
	}
	model := os.Getenv("OLLAMA_CHAT_MODEL")
	if model == "" {
		model = "deepseek-r1"
	}
	return &LLM{
		host:   host,
		model:  model,
		client: &http.Client{},
	}
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type chatChunk struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

// Chat sends messages to Ollama and streams responses via the stream callback.
// Returns the full assembled response string when done.
func (l *LLM) Chat(ctx context.Context, messages []conversation.LLMMessage, stream func(token string) error) (string, error) {
	ollamaMsgs := make([]ollamaMessage, 0, len(messages))
	for _, m := range messages {
		ollamaMsgs = append(ollamaMsgs, ollamaMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	reqBody := chatRequest{
		Model:    l.model,
		Messages: ollamaMsgs,
		Stream:   true,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal chat request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.host+"/api/chat", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("failed to create chat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama chat request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama chat returned status %d", resp.StatusCode)
	}

	var fullResponse string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var chunk chatChunk
		if err := json.Unmarshal(line, &chunk); err != nil {
			continue
		}
		token := chunk.Message.Content
		if token != "" {
			fullResponse += token
			if stream != nil {
				if err := stream(token); err != nil {
					return fullResponse, err
				}
			}
		}
		if chunk.Done {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return fullResponse, fmt.Errorf("error reading ollama stream: %w", err)
	}

	return fullResponse, nil
}
