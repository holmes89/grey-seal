package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"html/template"

	greyseal "github.com/holmes89/grey-seal/lib"
	"github.com/holmes89/grey-seal/lib/embedding"
	"github.com/holmes89/grey-seal/lib/rag"
	"github.com/holmes89/grey-seal/lib/repo/vectordb"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	dbLocation := flag.String("db", "./grey-seal.duckdb", "location of db")
	flag.Parse()
	vdb, err := vectordb.NewVectorDBReadOnly(*dbLocation)
	if err != nil {
		log.Fatal("Failed to initialize vector database:", err)
	}
	defer vdb.Close()
	embeddings := embedding.NewOllamaEmbeddingServiceFromEnvironment("nomic-embed-text")
	ragService := rag.NewRAGService(vdb, embeddings)

	// Create a new MCP server
	s := server.NewMCPServer(
		"Recipe Server",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// Define a simple tool
	tool := mcp.NewTool("recipes",
		mcp.WithDescription("get a recipe back"),
		mcp.WithArray("ingredients",
			mcp.Required(),
			mcp.Items(map[string]any{"type": "string"}),
			mcp.Description("Pass a set of ingredients and get a recipe"),
		),
	)

	recipeHandler := &RecipeHandler{
		ragService: ragService,
	}

	// Add tool handler
	s.AddTool(tool, recipeHandler.Handle)

	// Start the stdio server
	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}

	// errs := make(chan error, 2)
	// go func() {
	// 	log.Println("Listening...")
	// 	errs <- server.ServeStdio(s)
	// }()
	// go func() {
	// 	c := make(chan os.Signal, 1)
	// 	signal.Notify(c, syscall.SIGINT)
	// 	errs <- fmt.Errorf("%s", <-c)
	// }()
	// log.Println("terminated %w", <-errs)
}

type RecipeHandler struct {
	ragService greyseal.RAGService
}

func (rh *RecipeHandler) Handle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.GetArguments()
	log.Printf("type of arg: %T", arguments["ingredients"])
	ingredients, ok := arguments["ingredients"].([]any)
	if !ok {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "Error: ingredients parameter is required and must be a string array",
				},
			},
			IsError: true,
		}, nil
	}
	log.Print(ingredients)
	result, err := rh.ragService.Query(context.Background(), fmt.Sprintf("Create a recipe from these ingredients %s", ingredients), 1)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error: %s", err),
				},
			},
			IsError: true,
		}, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: template.HTMLEscapeString(result.Answer),
			},
		},
	}, nil
}
