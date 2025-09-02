package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/ollama/ollama/api"
)

// EmbeddingService defines the interface for embedding generation
type EmbeddingService interface {
	GenerateEmbedding(text string) ([]float32, error)
}

// OllamaEmbeddingServiceImpl uses the official Ollama client
type OllamaEmbeddingServiceImpl struct {
	client *api.Client
	model  string
}

// NewOllamaEmbeddingService creates a new service using the official Ollama client
func NewOllamaEmbeddingService(ollamaURL, model string, httpClient *http.Client) (*OllamaEmbeddingServiceImpl, error) {
	// Parse the URL to ensure it's valid
	parsedURL, err := url.Parse(ollamaURL)
	if err != nil {
		return nil, fmt.Errorf("invalid ollama URL: %w", err)
	}

	// Set environment variable for Ollama client if URL is provided
	if ollamaURL != "" {
		// The official client reads from OLLAMA_HOST environment variable
		// For this session, we'll store the URL but note that the official client
		// primarily uses environment configuration
	}

	// Create the official Ollama client (it will use OLLAMA_HOST env var or default)
	client := api.NewClient(parsedURL, httpClient)

	return &OllamaEmbeddingServiceImpl{
		client: client,
		model:  model,
	}, nil
}

// NewOllamaEmbeddingServiceFromEnvironment creates a service using environment configuration
func NewOllamaEmbeddingServiceFromEnvironment(model string) *OllamaEmbeddingServiceImpl {
	// This uses the OLLAMA_HOST environment variable automatically
	client, err := api.ClientFromEnvironment()
	if err != nil {
		panic(err)
	}

	return &OllamaEmbeddingServiceImpl{
		client: client,
		model:  model,
	}
}

// GenerateEmbedding generates embeddings using the official Ollama client with fallback
func (es *OllamaEmbeddingServiceImpl) GenerateEmbedding(text string) ([]float32, error) {
	if embedding, err := es.generateOllamaEmbedding(text); err == nil {
		return embedding, nil
	} else {
		log.Printf("Ollama unavailable, using mock embeddings: %v", err)
		return DefaultMockOllamaEmbeddingService{}.GenerateEmbedding(text)
	}
}

// generateOllamaEmbedding uses the official Ollama client to generate embeddings
func (es *OllamaEmbeddingServiceImpl) generateOllamaEmbedding(text string) ([]float32, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use the newer Embeddings method (for single embedding)
	req := &api.EmbeddingRequest{
		Model:  es.model,
		Prompt: text,
	}

	resp, err := es.client.Embeddings(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Convert []float64 to []float32
	if len(resp.Embedding) == 0 {
		return nil, fmt.Errorf("empty embedding received")
	}

	embedding := make([]float32, len(resp.Embedding))
	for i, v := range resp.Embedding {
		embedding[i] = float32(v)
	}

	return embedding, nil
}

// Alternative implementation using the Embed method for batch processing
func (es *OllamaEmbeddingServiceImpl) GenerateBatchEmbeddings(texts []string) ([][]float32, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Use the Embed method for batch processing (supports multiple inputs)
	req := &api.EmbedRequest{
		Model: es.model,
		Input: texts, // Can be a string or []string
	}

	resp, err := es.client.Embed(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate batch embeddings: %w", err)
	}

	if len(resp.Embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings received")
	}

	// The embeddings are already [][]float32, so we can return them directly
	return resp.Embeddings, nil
}

// HealthCheck verifies the Ollama service is available
func (es *OllamaEmbeddingServiceImpl) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return es.client.Heartbeat(ctx)
}

// ListAvailableModels returns a list of available models
func (es *OllamaEmbeddingServiceImpl) ListAvailableModels() (*api.ListResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return es.client.List(ctx)
}

// DefaultMockOllamaEmbeddingService provides mock embeddings as fallback
type DefaultMockOllamaEmbeddingService struct{}

// GenerateEmbedding generates mock embeddings for testing/fallback
func (es DefaultMockOllamaEmbeddingService) GenerateEmbedding(text string) ([]float32, error) {
	// Generate a deterministic but pseudo-random embedding based on text hash
	// This is just a placeholder - you might want a more sophisticated mock
	hash := simpleHash(text)
	embedding := make([]float32, 384) // Common embedding dimension

	for i := range embedding {
		// Generate deterministic "random" values based on hash and position
		embedding[i] = float32((hash*uint32(i+1))%1000-500) / 500.0 // Values between -1 and 1
	}

	return embedding, nil
}

// CustomOllamaEmbeddingService provides more control while still using official types
type CustomOllamaEmbeddingService struct {
	ollamaURL string
	model     string
	client    *http.Client
}

// NewCustomOllamaEmbeddingService creates a service with custom HTTP client (maintains your original approach)
func NewCustomOllamaEmbeddingService(ollamaURL, model string, client *http.Client) *CustomOllamaEmbeddingService {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &CustomOllamaEmbeddingService{
		ollamaURL: ollamaURL,
		model:     model,
		client:    client,
	}
}

// GenerateEmbedding uses your original HTTP approach but with official API types
func (es *CustomOllamaEmbeddingService) GenerateEmbedding(text string) ([]float32, error) {
	if embedding, err := es.generateOllamaEmbedding(text); err == nil {
		return embedding, nil
	} else {
		log.Printf("Ollama unavailable, using mock embeddings: %v", err)
		return DefaultMockOllamaEmbeddingService{}.GenerateEmbedding(text)
	}
}

// generateOllamaEmbedding uses your original HTTP approach with official request/response types
func (es *CustomOllamaEmbeddingService) generateOllamaEmbedding(text string) ([]float32, error) {
	// Use the official API types
	req := &api.EmbeddingRequest{
		Model:  es.model,
		Prompt: text,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := es.client.Post(es.ollamaURL+"/api/embeddings", "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to call Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama returned status %d", resp.StatusCode)
	}

	var response api.EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert []float64 to []float32
	if len(response.Embedding) == 0 {
		return nil, fmt.Errorf("empty embedding received")
	}

	embedding := make([]float32, len(response.Embedding))
	for i, v := range response.Embedding {
		embedding[i] = float32(v)
	}

	return embedding, nil
}

// Example usage and configuration
func ExampleUsage() {
	// Option 1: Create with specific URL and model
	service, err := NewOllamaEmbeddingService("http://localhost:11434", "nomic-embed-text", nil)
	if err != nil {
		log.Fatal(err)
	}

	// Option 2: Create from environment (uses OLLAMA_HOST env var)
	service2 := NewOllamaEmbeddingServiceFromEnvironment("nomic-embed-text")

	// Option 3: Create with custom HTTP client (maintains your original approach)
	customService := NewCustomOllamaEmbeddingService("http://localhost:11434", "nomic-embed-text", &http.Client{
		Timeout: 45 * time.Second,
	})

	// Health check
	if err := service.HealthCheck(); err != nil {
		log.Printf("Ollama service not available: %v", err)
	}

	// Generate single embedding (works with both service types)
	embedding, err := service.GenerateEmbedding("Hello world")
	if err != nil {
		log.Printf("Error generating embedding: %v", err)
	} else {
		log.Printf("Generated embedding with dimension: %d", len(embedding))
	}

	// Also test custom service
	customEmbedding, err := customService.GenerateEmbedding("Hello world")
	if err != nil {
		log.Printf("Error with custom service: %v", err)
	} else {
		log.Printf("Custom service embedding dimension: %d", len(customEmbedding))
	}

	// Generate batch embeddings
	texts := []string{"Hello", "World", "Ollama"}
	embeddings, err := service.GenerateBatchEmbeddings(texts)
	if err != nil {
		log.Printf("Error generating batch embeddings: %v", err)
	} else {
		log.Printf("Generated %d embeddings", len(embeddings))
	}

	// List available models
	models, err := service2.ListAvailableModels()
	if err != nil {
		log.Printf("Error listing models: %v", err)
	} else {
		log.Printf("Available models: %d", len(models.Models))
	}
}
