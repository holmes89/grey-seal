# Grey Seal

A Go-based Retrieval-Augmented Generation (RAG) system using DuckDB for vector storage. This project demonstrates core AI/LLM infrastructure concepts using familiar backend patterns.

## Architecture Overview

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   HTTP Client   │───▶│   RAG Server    │───▶│   DuckDB VSS    │
│                 │    │                 │    │                 │
│ • Query docs    │    │ • REST API      │    │ • Vector store  │
│ • Ingest docs   │    │ • Embeddings    │    │ • Similarity    │
│ • Search        │    │ • Chunking      │    │   search        │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## Key Concepts Demonstrated

- **Vector Database**: DuckDB with `vss` extension for similarity search
- **Embeddings**: Text-to-vector conversion (mock implementation, easily replaceable with Ollama/OpenAI)
- **Document Chunking**: Breaking large documents into searchable pieces
- **RAG Pattern**: Retrieve relevant context + Generate responses
- **Microservice Architecture**: REST API with clean separation of concerns

## Quick Start

### Prerequisites

- Go 1.21+
- Docker (optional)
- DuckDB CLI (optional, for debugging)

### Local Development

1. **Clone and setup**:
```bash
git clone <your-repo>
cd document-rag-system
go mod tidy
```

2. **Create sample documents**:
```bash
mkdir sample-docs
echo "This is a document about Go programming. Go is a statically typed language." > sample-docs/go-intro.txt
echo "Kubernetes is a container orchestration platform. It manages containerized applications." > sample-docs/kubernetes.txt
echo "Vector databases store high-dimensional vectors for similarity search." > sample-docs/vectors.txt
```

3. **Run the server**:
```bash
go run main.go
```

4. **Ingest documents**:
```bash
curl -X POST http://localhost:8080/ingest \
  -H "Content-Type: application/json" \
  -d '{"directory_path": "./sample-docs"}'
```

5. **Query the system**:
```bash
curl -X POST http://localhost:8080/query \
  -H "Content-Type: application/json" \
  -d '{"query": "What is Go programming?", "limit": 3}'
```

### Using Docker

```bash
docker-compose up -d
```

## API Endpoints

### POST /ingest
Processes all `.txt` files in a directory and stores them as searchable chunks.

**Request**:
```json
{
  "directory_path": "./sample-docs"
}
```

### POST /query
Performs RAG query: finds relevant context and generates an answer.

**Request**:
```json
{
  "query": "What is Kubernetes?",
  "limit": 5
}
```

**Response**:
```json
{
  "answer": "Based on the retrieved context...",
  "context": [
    {
      "id": "kubernetes.txt_chunk_0",
      "content": "Kubernetes is a container orchestration platform...",
      "similarity": 0.95
    }
  ]
}
```

### POST /search
Semantic search without LLM generation.

**Request**:
```json
{
  "query": "container orchestration",
  "limit": 3
}
```

## Project Structure

```
document-rag-system/
├── main.go              # Main application with all components
├── go.mod              # Dependencies
├── Dockerfile          # Container build
├── docker-compose.yml  # Multi-service setup
├── sample-docs/        # Sample documents
└── data/               # DuckDB database files
```

## Understanding the Code

### Vector Database (`VectorDB`)
- Wraps DuckDB with vector search extension
- Stores document chunks with 384-dimensional embeddings
- Uses HNSW index for efficient similarity search

### Embedding Service (`EmbeddingService`)
- Currently uses mock embeddings (hash-based)
- Easy to replace with real embeddings from Ollama/OpenAI
- Interface designed for dependency injection

### Document Processor (`DocumentProcessor`)
- Reads text files and splits into chunks
- Generates embeddings for each chunk
- Stores in vector database with metadata

### RAG Service (`RAGService`)
- Orchestrates the RAG workflow
- Retrieval: finds similar document chunks
- Generation: combines context (mock LLM response)

## Extending the System

### Adding Real Embeddings (Ollama)

1. **Install Ollama**:
```bash
curl -fsSL https://ollama.ai/install.sh | sh
ollama serve
ollama pull nomic-embed-text
```

2. **Replace `GenerateEmbedding` method**:
```go
func (es *EmbeddingService) GenerateEmbedding(text string) ([]float32, error) {
    // Call Ollama API
    req := map[string]interface{}{
        "model": "nomic-embed-text",
        "prompt": text,
    }
    
    // HTTP POST to http://localhost:11434/api/embeddings
    // Parse response and return embedding vector
}
```

### Adding Real LLM (Ollama)

Replace the mock answer generation in `RAGService.Query`:

```go
// Build context-aware prompt
prompt := fmt.Sprintf(`Context:\n%s\n\nQuestion: %s\nAnswer:`, 
    strings.Join(contextTexts, "\n"), query)

// Call Ollama generate API
// Return actual LLM response
```

### Adding More Document Types

Extend `ProcessFile` to handle PDFs, Word docs, etc.:

```go
switch filepath.Ext(filePath) {
case ".pdf":
    return dp.processPDF(filePath)
case ".docx":
    return dp.processWord(filePath)
case ".txt":
    return dp.processText(filePath)
}
```

## MCP Integration

To make this compatible with MCP (Model Context Protocol), add an MCP server that exposes document search as a tool:

```go
// MCP tool definition
type SearchTool struct {
    name: "search_documents"
    description: "Search through ingested documents"
    parameters: {
        query: "string"
        limit: "integer"
    }
}
```

## Production Considerations

- **Authentication**: Add API keys/JWT tokens
- **Rate Limiting**: Implement request throttling
- **Monitoring**: Add metrics and health checks
- **Scaling**: Use multiple instances with shared DuckDB
- **Caching**: Cache frequent embeddings/queries
- **Error Handling**: More robust error responses

## Learning Resources

- [DuckDB Vector Search Extension](https://duckdb.org/docs/extensions/vss.html)
- [Ollama API Documentation](https://github.com/ollama/ollama/blob/main/docs/api.md)
- [Vector Database Concepts](https://www.pinecone.io/learn/vector-database/)
- [RAG Architecture Patterns](https://docs.llamaindex.ai/en/stable/getting_started/concepts.html)

## Next Steps

1. Replace mock embeddings with Ollama
2. Add real LLM integration for answer generation
3. Implement MCP server wrapper
4. Add more document format support
5. Deploy to Kubernetes cluster
6. Add monitoring and observability

This system demonstrates the core patterns you'll see in AI infrastructure while using familiar backend development concepts.