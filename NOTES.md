document-search/

# Core Concepts

Large Language Models (LLMs) are essentially stateless API services that process text input and return text output. Think of them like a REST API where you POST a prompt and GET back a response. The key difference is they're computationally expensive and often require specialized hardware (GPUs).

**RAG (Retrieval Augmented Generation) Server** is a pattern you'll recognize—it's essentially a microservice that sits between your application and the LLM. When a query comes in, it:

- Searches a vector database for relevant context (like Elasticsearch, but for semantic similarity)
- Injects that context into the LLM prompt
- Returns the enhanced response

Think of it as a smart caching/context injection layer.

**MCP (Model Context Protocol) Server** is more like a plugin system or API gateway specifically for LLMs. It standardizes how external tools and data sources connect to AI applications. If you've worked with webhook systems or service mesh patterns, MCP servers provide a standardized interface for LLMs to call external services.

---

## Infrastructure Patterns

The AI space follows similar patterns to what you're used to:

- **Vector Databases** (Pinecone, Weaviate, Chroma): like your Postgres, but optimized for similarity search instead of exact matches
- **Embedding Services**: convert text to numerical vectors (similar to how you might hash or encode data)
- **Model Serving platforms** (Ollama, vLLM): like your Kubernetes deployments, but optimized for GPU workloads
- **Orchestration tools** (LangChain, LangGraph): workflow engines, similar to Argo workflows you've used

---

## Go-Specific Resources

For Go development in AI:

- **Ollama** – Excellent for local LLM serving, has a clean REST API
- **go-langchaingo** – Go port of LangChain for building AI applications
- **Weaviate Go client** – For vector database operations
- **OpenAI Go SDK** – For integrating with OpenAI's APIs

---

## Project Suggestion

Based on your background, I'd suggest building a simple RAG system in Go that demonstrates these concepts:

- **Document Ingestion Service** – Reads documents, creates embeddings, stores in vector DB
- **Query Service** – Takes user queries, retrieves relevant context, calls LLM
- **MCP-style Plugin System** – Allows the LLM to call external APIs (weather, database queries, etc.)

This would use your existing skills with microservices, databases, and APIs while introducing AI-specific concepts gradually.

---

## Learning Resources

- "Hands-On Large Language Models" by Jay Alammar – Great for understanding the technical foundations
- Ollama documentation – Start here for local development
- LangChain Go docs – For understanding AI application patterns
- Pinecone's learning center – Excellent vector database concepts

Given your experience with distributed systems and event-driven architectures (Kafka, CQRS), you'll find that many AI applications follow similar patterns—they're just processing different types of data (vectors instead of traditional records) and calling different types of services (LLMs instead of traditional APIs).

---

## Q&A

> **Would an MCP server call a RAG server if it implements the right protocol? How do you build a vector DB? Let’s say I want to make all of my documents searchable. How would that work?**

Great questions! Let me break this down:

### MCP Server + RAG Server Interaction

Yes, absolutely! An MCP server can call a RAG server. Think of it this way:

- **MCP Server** = API Gateway/Plugin Registry that exposes tools to the LLM
- **RAG Server** = Microservice that handles document search and retrieval

The MCP server would expose a `search_documents` tool that internally calls your RAG server's REST API. So the flow would be:

```
LLM → MCP Server → RAG Server → Vector DB
                ↓
User ← Response ← Enhanced Context
```

---

### Building a Vector Database

You don't typically build a vector database from scratch (like you wouldn't build Postgres from scratch). Instead, you either:

- Use a managed service (Pinecone, Weaviate Cloud)
- Self-host an existing solution (Weaviate, ChromaDB, Qdrant)
- Use a hybrid approach (PostgreSQL with pgvector extension)

Given your Kubernetes experience, I'd recommend starting with Weaviate or Qdrant deployed in your cluster.

---

## Document Search System Architecture

Here's how you'd make all your documents searchable:

### 1. Document Ingestion Pipeline

```go
// Pseudocode for the flow
func IngestDocument(filePath string) {
    // Extract text from document (PDF, Word, etc.)
    text := extractText(filePath)

    // Split into chunks (typically 500-1000 tokens)
    chunks := splitIntoChunks(text)

    // Generate embeddings for each chunk
    for i, chunk := range chunks {
        embedding := generateEmbedding(chunk) // Call to OpenAI/Ollama

        // Store in vector DB with metadata
        vectorDB.Store(VectorDocument{
            ID: generateID(),
            Content: chunk,
            Vector: embedding,
            Metadata: DocumentMetadata{
                FilePath: filePath,
                ChunkIndex: i,
                // Add your own metadata
            },
        })
    }
}
```

### 2. Search/Query Service

```go
func SearchDocuments(query string) []SearchResult {
    // Convert query to vector
    queryVector := generateEmbedding(query)

    // Search vector DB for similar chunks
    results := vectorDB.SimilaritySearch(queryVector, limit: 5)

    // Return results with original content + metadata
    return results
}
```

### 3. RAG Integration

```go
func AnswerQuestion(question string) string {
    // Get relevant context
    relevantChunks := SearchDocuments(question)

    // Build prompt with context
    prompt := buildRAGPrompt(question, relevantChunks)

    // Call LLM
    return callLLM(prompt)
}
```

---

## Practical Implementation Stack

For your use case, I'd suggest:

- **Vector DB:** PostgreSQL + pgvector (leverages your existing Postgres knowledge)
- **Embeddings:** Ollama with a local embedding model (nomic-embed-text)
- **Document Processing:** Go libraries like unidoc for PDFs, go-docx for Word docs

---

## Simple Project Structure

```text
document-search/
├── cmd/
│   ├── ingester/     # Batch job to process documents
│   ├── rag-server/   # RAG API service
│   └── mcp-server/   # MCP protocol server
├── internal/
│   ├── vectordb/     # Vector DB interface
│   ├── embeddings/   # Embedding generation
│   └── documents/    # Document processing
└── deployments/      # Your K8s manifests
```

---

## Getting Started Steps

1. **Start simple:** Use pgvector with PostgreSQL (you can deploy this easily in K8s)
2. **Local embeddings:** Run Ollama locally with nomic-embed-text model
3. **Process a few PDFs:** Build the ingestion pipeline first
4. **Build search API:** Simple REST endpoint that takes a query, returns relevant chunks
5. **Add RAG:** Integrate with an LLM to answer questions based on retrieved context

The beauty is this follows your existing patterns—you're just dealing with vectors instead of traditional relational data, and calling embedding/LLM APIs instead of traditional REST services.

---

Want me to show you a concrete example of the vector DB setup with pgvector, or dive deeper into the embedding generation process?