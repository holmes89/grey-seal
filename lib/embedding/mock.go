package embedding

import (
	"os"
)

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
