package cmd

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	services "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
	"github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services/servicesconnect"
	"github.com/spf13/cobra"
)

var (
	ingestURL    string
	ingestText   string
	ingestName   string
	ingestServer string
)

var ingestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Ingest a resource into grey-seal",
	Long:  "Ingest a website URL or literal text as a resource into the grey-seal knowledge base.",
	RunE:  runIngest,
}

func runIngest(cmd *cobra.Command, args []string) error {
	if ingestName == "" {
		return fmt.Errorf("--name is required")
	}
	if ingestURL == "" && ingestText == "" {
		return fmt.Errorf("one of --url or --text is required")
	}
	if ingestURL != "" && ingestText != "" {
		return fmt.Errorf("only one of --url or --text may be specified")
	}

	r := &greysealv1.Resource{
		Name: ingestName,
	}
	if ingestURL != "" {
		r.Source = greysealv1.Source_SOURCE_WEBSITE
		r.Path = ingestURL
	} else {
		r.Source = greysealv1.Source_SOURCE_TEXT
		r.Path = ingestText
	}

	baseURL := "http://" + ingestServer
	client := servicesconnect.NewResourceServiceClient(http.DefaultClient, baseURL, connect.WithGRPCWeb())

	req := connect.NewRequest(&services.IngestResourceRequest{Data: r})
	resp, err := client.IngestResource(context.Background(), req)
	if err != nil {
		return fmt.Errorf("ingest failed: %w", err)
	}

	fmt.Printf("Ingested resource UUID: %s\n", resp.Msg.GetData().GetUuid())
	return nil
}

func init() {
	ingestCmd.Flags().StringVar(&ingestURL, "url", "", "URL of a website to ingest")
	ingestCmd.Flags().StringVar(&ingestText, "text", "", "Literal text content to ingest")
	ingestCmd.Flags().StringVar(&ingestName, "name", "", "Human-readable name for the resource (required)")
	ingestCmd.Flags().StringVar(&ingestServer, "server", "localhost:9000", "API server address")

	rootCmd.AddCommand(ingestCmd)
}
