
import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// EmbeddingService defines the interface for text-to-vector embedding services.
type EmbeddingService interface {
	GenerateEmbedding(text string) ([]float32, error)
}

// EmbeddingServiceImpl is the real implementation using Ollama.
type EmbeddingServiceImpl struct {
	ollamaURL string
	model     string
	client    *http.Client
}

// NewEmbeddingService creates a new EmbeddingServiceImpl with injected client.
func NewEmbeddingService(ollamaURL, model string, client *http.Client) *EmbeddingServiceImpl {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &EmbeddingServiceImpl{
		ollamaURL: ollamaURL,
		model:     model,
		client:    client,
	}
}

func (es *EmbeddingServiceImpl) GenerateEmbedding(text string) ([]float32, error) {
	if embedding, err := es.generateOllamaEmbedding(text); err == nil {
		return embedding, nil
	} else {
		log.Printf("Ollama unavailable, using mock embeddings: %v", err)
		return DefaultMockEmbeddingService{}.GenerateEmbedding(text)
	}
}

func (es *EmbeddingServiceImpl) generateOllamaEmbedding(text string) ([]float32, error) {
	reqBody := map[string]interface{}{
		"model":  es.model,
		"prompt": text,
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	resp, err := es.client.Post(es.ollamaURL+"/api/embeddings", "application/json", strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, fmt.Errorf("failed to call Ollama: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama returned status %d", resp.StatusCode)
	}
	var response struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return response.Embedding, nil
}

// MockEmbeddingService is a mock implementation for testing or fallback.
type MockEmbeddingService struct{}

func (MockEmbeddingService) GenerateEmbedding(text string) ([]float32, error) {
	vector := make([]float32, 384)
	hash := simpleHash(text)
	for i := range vector {
		vector[i] = float32((hash >> (i % 32)) & 1)
	}
	return vector, nil
}

// DefaultMockEmbeddingService is a value for fallback use.
var DefaultMockEmbeddingService MockEmbeddingService

func getEnvDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func simpleHash(text string) uint32 {
	hash := uint32(2166136261)
	for _, c := range text {
		hash ^= uint32(c)
		hash *= 16777619
	}
	return hash
}
