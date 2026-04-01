package resource

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/net/html"

	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
)

// FetchContent retrieves the full text of a resource based on its source type.
//
//   - SOURCE_TEXT: returns Resource.Path directly (no network call).
//   - SOURCE_WEBSITE: HTTP GET + visible text extraction from HTML.
//   - SOURCE_PDF: not yet implemented; returns a descriptive error.
func FetchContent(ctx context.Context, r *greysealv1.Resource) (string, error) {
	switch r.Source {
	case greysealv1.Source_SOURCE_TEXT:
		return r.Path, nil
	case greysealv1.Source_SOURCE_WEBSITE:
		return fetchWebsite(ctx, r.Path)
	case greysealv1.Source_SOURCE_PDF:
		return "", fmt.Errorf("PDF content extraction not yet implemented (resource %s)", r.Uuid)
	default:
		return "", fmt.Errorf("unknown resource source type %v for resource %s", r.Source, r.Uuid)
	}
}

func fetchWebsite(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("building request for %s: %w", url, err)
	}
	req.Header.Set("User-Agent", "grey-seal/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetching %s: HTTP %d", url, resp.StatusCode)
	}

	return extractText(resp.Body)
}

// extractText walks an HTML parse tree and collects visible text nodes,
// skipping <script> and <style> subtrees.
func extractText(r io.Reader) (string, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return "", fmt.Errorf("parsing HTML: %w", err)
	}

	var buf strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style") {
			return
		}
		if n.Type == html.TextNode {
			if trimmed := strings.TrimSpace(n.Data); trimmed != "" {
				buf.WriteString(trimmed)
				buf.WriteByte(' ')
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return strings.TrimSpace(buf.String()), nil
}
