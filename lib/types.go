package greyseal

import "context"

// DocumentProcessingService defines the interface for document ingestion and processing.
type DocumentProcessingService interface {
	ProcessDirectory(dirPath string) error
	ProcessFile(filePath string) error
}

// EmbeddingService defines the interface for text-to-vector embedding services.
type EmbeddingService interface {
	GenerateEmbedding(text string) ([]float32, error)
}

// RAGService defines the interface for retrieval-augmented generation services.
type RAGService interface {
	Query(ctx context.Context, query string, limit int) (*RAGResponse, error)
}

// VectorDB defines the interface for vector database operations.
type VectorDB interface {
	StoreDocument(doc DocumentChunk) error
	SearchSimilar(queryVector []float32, limit int) ([]SearchResult, error)
	Close() error
}

// DocumentChunk represents a piece of a document with its embedding
type DocumentChunk struct {
	ID       string    `json:"id"`
	Content  string    `json:"content"`
	FilePath string    `json:"file_path"`
	ChunkID  int       `json:"chunk_id"`
	Vector   []float32 `json:"vector,omitempty"`
}

// SearchResult represents a search result with similarity score
type SearchResult struct {
	DocumentChunk
	Similarity float64 `json:"similarity"`
}

// RAGRequest represents an incoming RAG query
type RAGRequest struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

// RAGResponse represents the response with context and answer
type RAGResponse struct {
	Answer  string         `json:"answer"`
	Context []SearchResult `json:"context"`
}
