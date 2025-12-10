package scraper

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// ScrapedContent represents the extracted website content
type ScrapedContent struct {
	Title       string
	Description string
	Body        string // Main content, cleaned of navigation/ads
	RawHTML     string // Full HTML if needed
	Links       []string
	Headings    []Heading
	Metadata    map[string]string
}

// Heading represents a page heading with its level
type Heading struct {
	Level int // 1-6 for h1-h6
	Text  string
}

// Scraper handles website scraping with configurable options
type Scraper struct {
	client    *http.Client
	timeout   time.Duration
	userAgent string
}

// NewScraper creates a new scraper instance
func NewScraper() *Scraper {
	return &Scraper{
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		timeout:   30 * time.Second,
		userAgent: "Mozilla/5.0 (compatible; VectorBot/1.0)",
	}
}

func (s *Scraper) Scrape(ctx context.Context, url string) (*http.Response, error) {
	// Create request with context
	if !strings.HasPrefix(url, "http") {
		url = "http://" + url
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	// Execute request
	return s.client.Do(req)
}

// Scrape fetches and parses a website
func (s *Scraper) ScrapeContent(ctx context.Context, url string) (io.Reader, error) {
	resp, err := s.Scrape(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	// Parse HTML
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("parsing HTML: %w", err)
	}

	content := &ScrapedContent{
		RawHTML:  string(body),
		Metadata: make(map[string]string),
	}

	// Extract title
	content.Title = doc.Find("title").First().Text()
	content.Title = strings.TrimSpace(content.Title)

	// Extract meta description
	doc.Find("meta[name='description']").Each(func(i int, sel *goquery.Selection) {
		if desc, exists := sel.Attr("content"); exists {
			content.Description = strings.TrimSpace(desc)
		}
	})

	// Extract other useful metadata
	doc.Find("meta[property^='og:']").Each(func(i int, sel *goquery.Selection) {
		if prop, exists := sel.Attr("property"); exists {
			if cont, exists := sel.Attr("content"); exists {
				content.Metadata[prop] = cont
			}
		}
	})

	// Remove unwanted elements (scripts, styles, nav, footer, ads, etc.)
	doc.Find("script, style, nav, footer, header, aside, iframe, noscript").Remove()
	doc.Find("[class*='ad-'], [class*='advertisement'], [id*='ad-']").Remove()
	doc.Find(".sidebar, .navigation, .menu, .comments").Remove()

	// Extract main content - prioritize main/article tags
	var mainContent string
	if main := doc.Find("main, article, [role='main']").First(); main.Length() > 0 {
		mainContent = s.extractText(main)
	} else {
		// Fallback to body
		mainContent = s.extractText(doc.Find("body"))
	}

	content.Body = s.cleanText(mainContent)

	// Extract headings for structure
	doc.Find("h1, h2, h3, h4, h5, h6").Each(func(i int, sel *goquery.Selection) {
		text := strings.TrimSpace(sel.Text())
		if text != "" {
			level := int(sel.Nodes[0].Data[1] - '0') // h1 -> 1, h2 -> 2, etc.
			content.Headings = append(content.Headings, Heading{
				Level: level,
				Text:  text,
			})
		}
	})

	// Extract internal links
	doc.Find("a[href]").Each(func(i int, sel *goquery.Selection) {
		if href, exists := sel.Attr("href"); exists {
			// Basic link filtering
			if strings.HasPrefix(href, "http") || strings.HasPrefix(href, "/") {
				content.Links = append(content.Links, href)
			}
		}
	})

	return content.Embedding(), nil
}

// extractText recursively extracts text from HTML nodes
func (s *Scraper) extractText(sel *goquery.Selection) string {
	var builder strings.Builder

	sel.Contents().Each(func(i int, child *goquery.Selection) {
		node := child.Nodes[0]

		if node.Type == html.TextNode {
			text := strings.TrimSpace(node.Data)
			if text != "" {
				builder.WriteString(text)
				builder.WriteString(" ")
			}
		} else if node.Type == html.ElementNode {
			// Add spacing for block elements
			if s.isBlockElement(node.Data) {
				builder.WriteString("\n\n")
			}
			builder.WriteString(s.extractText(child))
			if s.isBlockElement(node.Data) {
				builder.WriteString("\n\n")
			}
		}
	})

	return builder.String()
}

// isBlockElement checks if an HTML element is a block-level element
func (s *Scraper) isBlockElement(tag string) bool {
	blockTags := map[string]bool{
		"p": true, "div": true, "h1": true, "h2": true, "h3": true,
		"h4": true, "h5": true, "h6": true, "li": true, "ul": true,
		"ol": true, "blockquote": true, "pre": true, "table": true,
		"tr": true, "section": true, "article": true,
	}
	return blockTags[tag]
}

// cleanText normalizes whitespace and removes excessive newlines
func (s *Scraper) cleanText(text string) string {
	// Replace multiple spaces with single space
	text = strings.Join(strings.Fields(text), " ")

	// Replace multiple newlines with double newline
	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}

	return strings.Join(cleaned, "\n\n")
}

// Embedding returns formatted content suitable for embedding
func (c *ScrapedContent) Embedding() io.Reader {
	var builder strings.Builder

	// Include title and description for context
	if c.Title != "" {
		builder.WriteString("Title: ")
		builder.WriteString(c.Title)
		builder.WriteString("\n\n")
	}

	if c.Description != "" {
		builder.WriteString("Description: ")
		builder.WriteString(c.Description)
		builder.WriteString("\n\n")
	}

	// Add main content
	builder.WriteString(c.Body)

	return bytes.NewBufferString(builder.String())
}
