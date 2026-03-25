package pages

import (
	"regexp"
)

// IDExtractor is a function type for extracting IDs from URL paths
type IDExtractor func(path string) (string, string)

// ExtractPathIDs extracts one or two UUIDs from a URL path
// Returns (firstID, secondID) where secondID may be empty
func ExtractPathIDs(path string) (string, string) {
	uuidRegex := regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	matches := uuidRegex.FindAllString(path, -1)
	
	if len(matches) == 0 {
		return "", ""
	}
	if len(matches) == 1 {
		return matches[0], ""
	}
	return matches[0], matches[1]
}
