package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	greyseal "github.com/holmes89/grey-seal/lib"
)

// MCP Protocol Types
type MCPMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method,omitempty"`
	Params  interface{} `json:"params,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

type ToolListResult struct {
	Tools []Tool `json:"tools"`
}

type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type CallToolResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type InitializeParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ClientInfo      ClientInfo             `json:"clientInfo"`
}

type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type InitializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ServerInfo      ServerInfo             `json:"serverInfo"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// MCPServer wraps our RAG service for MCP protocol
type MCPServer struct {
	RagService *greyseal.RAGService
}

func NewMCPServer(ragService *greyseal.RAGService) *MCPServer {
	return &MCPServer{
		RagService: ragService,
	}
}

func (s *MCPServer) handleMessage(message MCPMessage) MCPMessage {
	switch message.Method {
	case "initialize":
		return s.handleInitialize(message)
	case "tools/list":
		return s.handleToolsList(message)
	case "tools/call":
		return s.handleToolCall(message)
	default:
		return MCPMessage{
			JSONRPC: "2.0",
			ID:      message.ID,
			Error: &MCPError{
				Code:    -32601,
				Message: fmt.Sprintf("Method not found: %s", message.Method),
			},
		}
	}
}

func (s *MCPServer) handleInitialize(message MCPMessage) MCPMessage {
	return MCPMessage{
		JSONRPC: "2.0",
		ID:      message.ID,
		Result: InitializeResult{
			ProtocolVersion: "2024-11-05",
			Capabilities: map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			ServerInfo: ServerInfo{
				Name:    "document-rag-server",
				Version: "1.0.0",
			},
		},
	}
}

func (s *MCPServer) handleToolsList(message MCPMessage) MCPMessage {
	tools := []Tool{
		{
			Name:        "search_documents",
			Description: "Search through ingested documents using semantic similarity",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The search query to find relevant documents",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of results to return (default: 5)",
						"default":     5,
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "ask_documents",
			Description: "Ask a question about the ingested documents using RAG (Retrieval-Augmented Generation)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"question": map[string]interface{}{
						"type":        "string",
						"description": "The question to ask about the documents",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of context chunks to use (default: 5)",
						"default":     5,
					},
				},
				"required": []string{"question"},
			},
		},
	}

	return MCPMessage{
		JSONRPC: "2.0",
		ID:      message.ID,
		Result: ToolListResult{
			Tools: tools,
		},
	}
}

func (s *MCPServer) handleToolCall(message MCPMessage) MCPMessage {
	// Parse the parameters
	paramsBytes, err := json.Marshal(message.Params)
	if err != nil {
		return s.errorResponse(message.ID, -32602, "Invalid params", err)
	}

	var params CallToolParams
	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		return s.errorResponse(message.ID, -32602, "Invalid params", err)
	}

	switch params.Name {
	case "search_documents":
		return s.handleSearchDocuments(message.ID, params.Arguments)
	case "ask_documents":
		return s.handleAskDocuments(message.ID, params.Arguments)
	default:
		return s.errorResponse(message.ID, -32602, "Unknown tool", fmt.Errorf("tool %s not found", params.Name))
	}
}

func (s *MCPServer) handleSearchDocuments(id interface{}, args map[string]interface{}) MCPMessage {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return s.errorResponse(id, -32602, "Missing or invalid query parameter", nil)
	}

	limit := 5
	if l, ok := args["limit"]; ok {
		if limitFloat, ok := l.(float64); ok {
			limit = int(limitFloat)
		}
	}

	// Generate embedding for search
	queryVector, err := s.RagService.GenerateEmbedding(query)
	if err != nil {
		return s.errorResponse(id, -32603, "Failed to generate embedding", err)
	}

	// Search documents
	results, err := s.RagService.VectorDB.SearchSimilar(queryVector, limit)
	if err != nil {
		return s.errorResponse(id, -32603, "Search failed", err)
	}

	// Format results for MCP
	var content []string
	content = append(content, fmt.Sprintf("Found %d relevant documents for query: '%s'\n", len(results), query))

	for i, result := range results {
		content = append(content, fmt.Sprintf("%d. **%s** (similarity: %.3f)\n%s\n",
			i+1, result.FilePath, result.Similarity, result.Content))
	}

	return MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result: CallToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: strings.Join(content, "\n"),
				},
			},
		},
	}
}

func (s *MCPServer) handleAskDocuments(id interface{}, args map[string]interface{}) MCPMessage {
	question, ok := args["question"].(string)
	if !ok || question == "" {
		return s.errorResponse(id, -32602, "Missing or invalid question parameter", nil)
	}

	limit := 5
	if l, ok := args["limit"]; ok {
		if limitFloat, ok := l.(float64); ok {
			limit = int(limitFloat)
		}
	}

	// Perform RAG query
	response, err := s.RagService.Query(context.Background(), question, limit)
	if err != nil {
		return s.errorResponse(id, -32603, "RAG query failed", err)
	}

	// Format response for MCP
	var content []string
	content = append(content, fmt.Sprintf("**Answer to: %s**\n", question))
	content = append(content, response.Answer)
	content = append(content, "\n**Sources:**")

	for i, ctx := range response.Context {
		content = append(content, fmt.Sprintf("%d. %s (similarity: %.3f)",
			i+1, ctx.FilePath, ctx.Similarity))
	}

	return MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result: CallToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: strings.Join(content, "\n"),
				},
			},
		},
	}
}

func (s *MCPServer) errorResponse(id interface{}, code int, message string, err error) MCPMessage {
	errorData := message
	if err != nil {
		errorData = fmt.Sprintf("%s: %v", message, err)
	}

	return MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error: &MCPError{
			Code:    code,
			Message: errorData,
		},
	}
}

func (s *MCPServer) Run() {
	scanner := bufio.NewScanner(os.Stdin)

	log.Println("MCP Server started. Waiting for messages...")

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var message MCPMessage
		if err := json.Unmarshal([]byte(line), &message); err != nil {
			log.Printf("Failed to parse message: %v", err)
			continue
		}

		response := s.handleMessage(message)

		responseBytes, err := json.Marshal(response)
		if err != nil {
			log.Printf("Failed to marshal response: %v", err)
			continue
		}

		fmt.Println(string(responseBytes))
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading input: %v", err)
	}
}

// Main function for MCP server mode
func runMCPServer() {
	// Initialize the same components as the HTTP server
	vdb, err := NewVectorDB("./documents.duckdb")
	if err != nil {
		log.Fatal("Failed to initialize vector database:", err)
	}

	embeddings := NewEmbeddingService()
	ragService := NewRAGService(vdb, embeddings)

	mcpServer := NewMCPServer(ragService)
	mcpServer.Run()
}

// Add this to main.go's main function to support both modes
func init() {
	// Check if we should run in MCP mode
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		runMCPServer()
		os.Exit(0)
	}
}
