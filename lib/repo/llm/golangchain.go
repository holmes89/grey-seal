package llm

import (
	"context"
	"os"
	"strings"

	"github.com/holmes89/grey-seal/lib/greyseal/conversation"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
)

// LangchainLLM wraps a golangchain model to implement conversation.LLM.
type LangchainLLM struct {
	model llms.Model
}

var _ conversation.LLM = (*LangchainLLM)(nil)

// New creates a LangchainLLM backed by Ollama. Falls back to env vars
// OLLAMA_HOST and OLLAMA_CHAT_MODEL if arguments are empty.
func New(ollamaHost, modelName string) (*LangchainLLM, error) {
	if ollamaHost == "" {
		ollamaHost = os.Getenv("OLLAMA_HOST")
	}
	if ollamaHost == "" {
		ollamaHost = "http://localhost:11434"
	}
	if modelName == "" {
		modelName = os.Getenv("OLLAMA_CHAT_MODEL")
	}
	if modelName == "" {
		modelName = "deepseek-r1"
	}
	m, err := ollama.New(
		ollama.WithModel(modelName),
		ollama.WithServerURL(ollamaHost),
	)
	if err != nil {
		return nil, err
	}
	return &LangchainLLM{model: m}, nil
}

// Chat sends messages to Ollama via golangchain and streams tokens via the
// provided callback. Returns the full assembled response when streaming completes.
func (l *LangchainLLM) Chat(ctx context.Context, messages []conversation.LLMMessage, stream func(token string) error) (string, error) {
	content := make([]llms.MessageContent, 0, len(messages))
	for _, m := range messages {
		var role llms.ChatMessageType
		switch m.Role {
		case "system":
			role = llms.ChatMessageTypeSystem
		case "assistant":
			role = llms.ChatMessageTypeAI
		default:
			role = llms.ChatMessageTypeHuman
		}
		content = append(content, llms.TextParts(role, m.Content))
	}
	var sb strings.Builder
	_, err := l.model.GenerateContent(ctx, content,
		llms.WithStreamingFunc(func(_ context.Context, chunk []byte) error {
			token := string(chunk)
			sb.WriteString(token)
			return stream(token)
		}),
	)
	if err != nil {
		return "", err
	}
	return sb.String(), nil
}
