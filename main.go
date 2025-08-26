package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/marcboeker/go-duckdb"
)

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

// VectorDB wraps DuckDB operations for vector search
type VectorDB struct {
	db *sql.DB
}

// NewVectorDB initializes DuckDB with vector search extension
func NewVectorDB(dbPath string) (*VectorDB, error) {
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open DuckDB: %w", err)
	}

	vdb := &VectorDB{db: db}
	if err := vdb.setupTables(); err != nil {
		return nil, fmt.Errorf("failed to setup tables: %w", err)
	}

	return vdb, nil
}

// setupTables creates the necessary tables and installs vector extension
func (vdb *VectorDB) setupTables() error {
	// Install and load the vector search extension
	queries := []string{
		"INSTALL vss;",
		"LOAD vss;",
		`CREATE TABLE IF NOT EXISTS documents (
			id VARCHAR PRIMARY KEY,
			content TEXT NOT NULL,
			file_path VARCHAR NOT NULL,
			chunk_id INTEGER NOT NULL,
			embedding FLOAT[384] -- Assuming 384-dim embeddings from nomic-embed-text
		);`,
		`CREATE INDEX IF NOT EXISTS idx_documents_embedding 
		 ON documents USING HNSW (embedding) 
		 WITH (metric = 'cosine');`,
	}

	for _, query := range queries {
		if _, err := vdb.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query '%s': %w", query, err)
		}
	}

	return nil
}

// StoreDocument stores a document chunk with its embedding
func (vdb *VectorDB) StoreDocument(doc DocumentChunk) error {
	// Convert float32 slice to string array for DuckDB
	vectorStr := vectorToString(doc.Vector)

	query := `INSERT OR REPLACE INTO documents (id, content, file_path, chunk_id, embedding) 
			  VALUES (?, ?, ?, ?, ?::FLOAT[])`

	_, err := vdb.db.Exec(query, doc.ID, doc.Content, doc.FilePath, doc.ChunkID, vectorStr)
	return err
}

// SearchSimilar finds documents similar to the query vector
func (vdb *VectorDB) SearchSimilar(queryVector []float32, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 5
	}

	vectorStr := vectorToString(queryVector)

	query := `SELECT id, content, file_path, chunk_id, 
			         cosine_similarity(embedding, ?::FLOAT[]) as similarity
			  FROM documents 
			  ORDER BY similarity DESC 
			  LIMIT ?`

	rows, err := vdb.db.Query(query, vectorStr, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		err := rows.Scan(
			&result.ID,
			&result.Content,
			&result.FilePath,
			&result.ChunkID,
			&result.Similarity,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}
		results = append(results, result)
	}

	return results, nil
}

// EmbeddingService handles text-to-vector conversion
type EmbeddingService struct {
	ollamaURL string
	model     string
	client    *http.Client
}

// NewEmbeddingService creates a new embedding service
func NewEmbeddingService() *EmbeddingService {
	return &EmbeddingService{
		ollamaURL: getEnvDefault("OLLAMA_URL", "http://localhost:11434"),
		model:     getEnvDefault("EMBEDDING_MODEL", "nomic-embed-text"),
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

// GenerateEmbedding converts text to vector using Ollama
func (es *EmbeddingService) GenerateEmbedding(text string) ([]float32, error) {
	// Try Ollama first, fallback to mock if unavailable
	if embedding, err := es.generateOllamaEmbedding(text); err == nil {
		return embedding, nil
	} else {
		log.Printf("Ollama unavailable, using mock embeddings: %v", err)
		return es.generateMockEmbedding(text), nil
	}
}

// generateOllamaEmbedding calls Ollama API for real embeddings
func (es *EmbeddingService) generateOllamaEmbedding(text string) ([]float32, error) {
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

// generateMockEmbedding creates mock embeddings for development/fallback
func (es *EmbeddingService) generateMockEmbedding(text string) []float32 {
	// Generate a simple hash-based mock vector for demonstration
	vector := make([]float32, 384)
	hash := simpleHash(text)
	for i := range vector {
		vector[i] = float32((hash >> (i % 32)) & 1)
	}
	return vector
}

// DocumentProcessor handles document ingestion
type DocumentProcessor struct {
	vectorDB   *VectorDB
	embeddings *EmbeddingService
}

// NewDocumentProcessor creates a new document processor
func NewDocumentProcessor(vdb *VectorDB, es *EmbeddingService) *DocumentProcessor {
	return &DocumentProcessor{
		vectorDB:   vdb,
		embeddings: es,
	}
}

// ProcessDirectory recursively processes all text files in a directory
func (dp *DocumentProcessor) ProcessDirectory(dirPath string) error {
	return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".txt") {
			log.Printf("Processing file: %s", path)
			return dp.ProcessFile(path)
		}

		return nil
	})
}

// ProcessFile processes a single file into chunks and stores them
func (dp *DocumentProcessor) ProcessFile(filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Split into chunks (simple word-based chunking)
	chunks := chunkText(string(content), 500) // 500 words per chunk

	for i, chunk := range chunks {
		// Generate embedding
		vector, err := dp.embeddings.GenerateEmbedding(chunk)
		if err != nil {
			return fmt.Errorf("failed to generate embedding: %w", err)
		}

		// Create document chunk
		doc := DocumentChunk{
			ID:       fmt.Sprintf("%s_chunk_%d", filepath.Base(filePath), i),
			Content:  chunk,
			FilePath: filePath,
			ChunkID:  i,
			Vector:   vector,
		}

		// Store in vector database
		if err := dp.vectorDB.StoreDocument(doc); err != nil {
			return fmt.Errorf("failed to store document: %w", err)
		}
	}

	log.Printf("Processed %s into %d chunks", filePath, len(chunks))
	return nil
}

// RAGService handles retrieval-augmented generation
type RAGService struct {
	vectorDB   *VectorDB
	embeddings *EmbeddingService
	llmURL     string
	llmModel   string
	client     *http.Client
}

// NewRAGService creates a new RAG service
func NewRAGService(vdb *VectorDB, es *EmbeddingService) *RAGService {
	return &RAGService{
		vectorDB:   vdb,
		embeddings: es,
		llmURL:     getEnvDefault("OLLAMA_URL", "http://localhost:11434"),
		llmModel:   getEnvDefault("LLM_MODEL", "llama3.2"),
		client:     &http.Client{Timeout: 60 * time.Second},
	}
}

// Query performs RAG query: retrieve relevant context and generate answer
func (rs *RAGService) Query(ctx context.Context, query string, limit int) (*RAGResponse, error) {
	// Generate embedding for the query
	queryVector, err := rs.embeddings.GenerateEmbedding(query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Search for similar documents
	results, err := rs.vectorDB.SearchSimilar(queryVector, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search documents: %w", err)
	}

	// Build context from retrieved documents
	var contextTexts []string
	for _, result := range results {
		contextTexts = append(contextTexts, fmt.Sprintf("From %s: %s",
			filepath.Base(result.FilePath), result.Content))
	}
	context := strings.Join(contextTexts, "\n\n")

	// Generate answer using LLM with context
	answer, err := rs.generateAnswer(ctx, query, context)
	if err != nil {
		// Fallback to simple context summary if LLM fails
		answer = fmt.Sprintf("Based on the retrieved context, here are the most relevant passages for '%s':\n\n%s",
			query, strings.Join(contextTexts[:min(2, len(contextTexts))], "\n\n"))
		log.Printf("LLM generation failed, using fallback: %v", err)
	}

	return &RAGResponse{
		Answer:  answer,
		Context: results,
	}, nil
}

// generateAnswer calls Ollama to generate an answer with context
func (rs *RAGService) generateAnswer(ctx context.Context, query, context string) (string, error) {
	prompt := fmt.Sprintf(`You are a helpful assistant. Use the following context to answer the question accurately and concisely.

Context:
%s

Question: %s

Answer:`, context, query)

	reqBody := map[string]interface{}{
		"model":  rs.llmModel,
		"prompt": prompt,
		"stream": false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", rs.llmURL+"/api/generate", strings.NewReader(string(jsonData)))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := rs.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Ollama returned status %d", resp.StatusCode)
	}

	var response struct {
		Response string `json:"response"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return response.Response, nil
}

// HTTP Handlers

func setupRoutes(ragService *RAGService, docProcessor *DocumentProcessor) *gin.Engine {
	r := gin.Default()

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// Ingest documents from a directory
	r.POST("/ingest", func(c *gin.Context) {
		var req struct {
			DirectoryPath string `json:"directory_path"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := docProcessor.ProcessDirectory(req.DirectoryPath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Documents processed successfully"})
	})

	// RAG query endpoint
	r.POST("/query", func(c *gin.Context) {
		var req RAGRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if req.Limit == 0 {
			req.Limit = 5
		}

		response, err := ragService.Query(c.Request.Context(), req.Query, req.Limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, response)
	})

	// Search endpoint (just retrieval, no generation)
	r.POST("/search", func(c *gin.Context) {
		var req RAGRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		embeddings := NewEmbeddingService()
		queryVector, err := embeddings.GenerateEmbedding(req.Query)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Access the vectorDB through ragService
		results, err := ragService.vectorDB.SearchSimilar(queryVector, req.Limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"results": results})
	})

	return r
}

func main() {
	// Initialize DuckDB with vector search
	vdb, err := NewVectorDB("./documents.duckdb")
	if err != nil {
		log.Fatal("Failed to initialize vector database:", err)
	}

	// Initialize services
	embeddings := NewEmbeddingService()
	docProcessor := NewDocumentProcessor(vdb, embeddings)
	ragService := NewRAGService(vdb, embeddings)

	// Setup HTTP routes
	router := setupRoutes(ragService, docProcessor)

	// Start server
	log.Println("Starting RAG server on :8080")
	log.Println("Endpoints:")
	log.Println("  POST /ingest - Process documents from directory")
	log.Println("  POST /query - RAG query with context")
	log.Println("  POST /search - Semantic search only")
	log.Println("  GET /health - Health check")

	if err := router.Run(":8080"); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

// Utility functions

func vectorToString(vector []float32) string {
	strValues := make([]string, len(vector))
	for i, v := range vector {
		strValues[i] = fmt.Sprintf("%f", v)
	}
	return "[" + strings.Join(strValues, ",") + "]"
}

func chunkText(text string, maxWords int) []string {
	words := strings.Fields(text)
	var chunks []string

	for i := 0; i < len(words); i += maxWords {
		end := i + maxWords
		if end > len(words) {
			end = len(words)
		}
		chunks = append(chunks, strings.Join(words[i:end], " "))
	}

	return chunks
}

func simpleHash(text string) uint32 {
	hash := uint32(2166136261)
	for _, c := range text {
		hash ^= uint32(c)
		hash *= 16777619
	}
	return hash
}

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
