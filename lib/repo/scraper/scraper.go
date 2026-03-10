package scraper

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/net/html"
)

// Scraper extracts plain text from a URL by fetching HTML and stripping tags.
type Scraper struct {
	client *http.Client
}

// NewScraper creates a new Scraper with a default HTTP client.
func NewScraper() *Scraper {
	return &Scraper{client: &http.Client{}}
}

// ScrapeText fetches the given URL and extracts visible text, skipping script/style elements.
func (s *Scraper) ScrapeText(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "grey-seal/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("unexpected status %d fetching %s", resp.StatusCode, url)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	var sb strings.Builder
	extractText(doc, &sb)
	return strings.TrimSpace(sb.String()), nil
}

// extractText recursively walks the HTML node tree and writes text node content to sb.
// It skips script and style elements entirely.
func extractText(n *html.Node, sb *strings.Builder) {
	if n.Type == html.ElementNode {
		tag := strings.ToLower(n.Data)
		if tag == "script" || tag == "style" {
			return
		}
	}
	if n.Type == html.TextNode {
		text := strings.TrimSpace(n.Data)
		if text != "" {
			sb.WriteString(text)
			sb.WriteString(" ")
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		extractText(c, sb)
	}
}
