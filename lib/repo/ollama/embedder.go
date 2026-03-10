package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// Embedder calls the Ollama /api/embeddings endpoint to generate text embeddings.
type Embedder struct {
	host  string
	model string
	client *http.Client
}

// NewEmbedder creates an Embedder using OLLAMA_HOST and OLLAMA_EMBED_MODEL env vars.
func NewEmbedder() *Embedder {
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		host = "http://localhost:11434"
	}
	model := os.Getenv("OLLAMA_EMBED_MODEL")
	if model == "" {
		model = "all-minilm"
	}
	return &Embedder{
		host:   host,
		model:  model,
		client: &http.Client{},
	}
}

type embedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type embedResponse struct {
	Embedding []float32 `json:"embedding"`
}

// EmbedDocuments generates embeddings for each text by calling Ollama once per chunk.
func (e *Embedder) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, 0, len(texts))
	for _, text := range texts {
		emb, err := e.embedOne(ctx, text)
		if err != nil {
			return nil, err
		}
		result = append(result, emb)
	}
	return result, nil
}

func (e *Embedder) embedOne(ctx context.Context, text string) ([]float32, error) {
	reqBody := embedRequest{
		Model:  e.model,
		Prompt: text,
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embed request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.host+"/api/embeddings", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama embeddings request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama embeddings returned status %d", resp.StatusCode)
	}

	var embedResp embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("failed to decode embed response: %w", err)
	}
	return embedResp.Embedding, nil
}
