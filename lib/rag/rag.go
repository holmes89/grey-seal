package rag

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	greyseal "github.com/holmes89/grey-seal/lib"
)

var _ greyseal.RAGService = (*RAGServiceImpl)(nil)

type RAGServiceImpl struct {
	vectorDB   greyseal.VectorDB
	embeddings greyseal.EmbeddingService
	llmURL     string
	llmModel   string
	client     *http.Client
}

func NewRAGService(vdb greyseal.VectorDB, es greyseal.EmbeddingService) *RAGServiceImpl {
	return &RAGServiceImpl{
		vectorDB:   vdb,
		embeddings: es,
		llmURL:     getEnvDefault("OLLAMA_URL", "http://localhost:11434"),
		llmModel:   getEnvDefault("LLM_MODEL", "llama3.2"),
		client:     &http.Client{Timeout: 60 * time.Second},
	}
}

func (rs *RAGServiceImpl) Query(ctx context.Context, query string, limit int) (*greyseal.RAGResponse, error) {
	queryVector, err := rs.embeddings.GenerateEmbedding(query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}
	results, err := rs.vectorDB.SearchSimilar(queryVector, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search documents: %w", err)
	}
	var contextTexts []string
	for _, result := range results {
		contextTexts = append(contextTexts, fmt.Sprintf("From %s: %s", filepath.Base(result.FilePath), result.Content))
	}
	contextStr := strings.Join(contextTexts, "\n\n")
	answer, err := rs.generateAnswer(ctx, query, contextStr)
	if err != nil {
		answer = fmt.Sprintf("Based on the retrieved context, here are the most relevant passages for '%s':\n\n%s", query, strings.Join(contextTexts[:min(2, len(contextTexts))], "\n\n"))
		log.Printf("LLM generation failed, using fallback: %v", err)
	}
	return &greyseal.RAGResponse{
		Answer:  answer,
		Context: results,
	}, nil
}

// func (rs *RAGServiceImpl) generateAnswer(ctx context.Context, query, context string) (string, error) {
// 	prompt := fmt.Sprintf(`You are a helpful assistant. Use the following context to answer the question accurately and concisely.\n\nContext:\n%s\n\nQuestion: %s\n\nAnswer:`, context, query)
// 	reqBody := map[string]interface{}{
// 		"model":  rs.llmModel,
// 		"prompt": prompt,
// 		"stream": false,
// 	}
// 	jsonData, err := json.Marshal(reqBody)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to marshal request: %w", err)
// 	}
// 	req, err := http.NewRequestWithContext(ctx, "POST", rs.llmURL+"/api/generate", strings.NewReader(string(jsonData)))
// 	if err != nil {
// 		return "", fmt.Errorf("failed to create request: %w", err)
// 	}
// 	req.Header.Set("Content-Type", "application/json")
// 	resp, err := rs.client.Do(req)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to call Ollama: %w", err)
// 	}
// 	defer resp.Body.Close()
// 	if resp.StatusCode != http.StatusOK {
// 		return "", fmt.Errorf("Ollama returned status %d", resp.StatusCode)
// 	}
// 	var response struct {
// 		Response string `json:"response"`
// 	}
// 	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
// 		return "", fmt.Errorf("failed to decode response: %w", err)
// 	}
// 	return response.Response, nil
// }

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func getEnvDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
