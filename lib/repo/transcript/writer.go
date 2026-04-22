package transcript

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/holmes89/grey-seal/lib/greyseal/conversation"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/fileblob"
)

// Writer appends markdown transcript turns to per-conversation files in a
// gocloud.dev/blob bucket. Supports file:// buckets locally and can be
// switched to s3:// or gcs:// by changing the bucket URL.
type Writer struct {
	bucket *blob.Bucket
}

// NewWriter opens (or creates) the given directory as a local blob bucket.
func NewWriter(dir string) (*Writer, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve dir %s: %w", dir, err)
	}
	if err := os.MkdirAll(absDir, 0o700); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", absDir, err)
	}
	bucket, err := blob.OpenBucket(context.Background(), "file://"+absDir)
	if err != nil {
		return nil, fmt.Errorf("open bucket: %w", err)
	}
	return &Writer{bucket: bucket}, nil
}

// NewWriterFromURL opens a bucket directly from a gocloud.dev URL
// (e.g. "file:///tmp/transcripts", "s3://my-bucket?region=us-east-1").
func NewWriterFromURL(ctx context.Context, bucketURL string) (*Writer, error) {
	bucket, err := blob.OpenBucket(ctx, bucketURL)
	if err != nil {
		return nil, fmt.Errorf("open bucket %s: %w", bucketURL, err)
	}
	return &Writer{bucket: bucket}, nil
}

// Close releases the underlying bucket.
func (w *Writer) Close() error {
	return w.bucket.Close()
}

// WriteTurn appends a markdown turn section to {conversationUUID}.md.
// Implements conversation.TranscriptWriter.
func (w *Writer) WriteTurn(ctx context.Context, turn conversation.TranscriptTurn) error {
	key := turn.ConversationUUID + ".md"

	var existing []byte
	rc, err := w.bucket.NewReader(ctx, key, nil)
	if err == nil {
		existing, _ = io.ReadAll(rc)
		_ = rc.Close()
	}

	var sb strings.Builder
	renderTurn(&sb, turn)

	wc, err := w.bucket.NewWriter(ctx, key, &blob.WriterOptions{ContentType: "text/markdown; charset=utf-8"})
	if err != nil {
		return fmt.Errorf("open writer for %s: %w", key, err)
	}
	if len(existing) > 0 {
		if _, err := wc.Write(existing); err != nil {
			_ = wc.Close()
			return err
		}
	}
	if _, err := wc.Write([]byte(sb.String())); err != nil {
		_ = wc.Close()
		return err
	}
	return wc.Close()
}

func renderTurn(sb *strings.Builder, t conversation.TranscriptTurn) {
	fmt.Fprintf(sb, "## Turn %d — %s\n\n", t.TurnIndex, t.Timestamp.UTC().Format("2006-01-02T15:04:05Z"))
	fmt.Fprintf(sb, "**User message**: %s\n\n", t.UserMessage)

	if t.SystemPrompt != "" {
		fmt.Fprintf(sb, "**System prompt**: %s\n\n", t.SystemPrompt)
	}
	if t.ConversationSummary != "" {
		fmt.Fprintf(sb, "**Conversation summary**: %s\n\n", t.ConversationSummary)
	}
	if len(t.SearchResults) > 0 {
		sb.WriteString("**Shrike search**:\n\n")
		sb.WriteString("| # | Title | Score | Snippet |\n")
		sb.WriteString("|---|-------|-------|---------|\n")
		for i, r := range t.SearchResults {
			fmt.Fprintf(sb, "| %d | %s | %.3f | %s |\n", i+1, r.Title, r.Score, r.Snippet)
		}
		sb.WriteString("\n")
	}
	if len(t.AssembledMessages) > 0 {
		sb.WriteString("**Assembled prompt**:\n\n")
		for _, m := range t.AssembledMessages {
			fmt.Fprintf(sb, "```%s\n%s\n```\n\n", m.Role, m.Content)
		}
	}
	fmt.Fprintf(sb, "**Response**: %s\n\n", t.Response)
	if len(t.ResourceUUIDs) > 0 {
		fmt.Fprintf(sb, "**Resources cited**: %s\n\n", strings.Join(t.ResourceUUIDs, ", "))
	}
	sb.WriteString("---\n\n")
}
