package greyseal

import (
	"fmt"
	"strings"
)

// VectorToString converts a float32 slice to a string representation for DuckDB
func VectorToString(vector []float32) string {
	strValues := make([]string, len(vector))
	for i, v := range vector {
		strValues[i] = fmt.Sprintf("%f", v)
	}
	return "[" + strings.Join(strValues, ",") + "]"
}

// ChunkText splits text into chunks of maxWords words each
func ChunkText(text string, maxWords int) []string {
	words := strings.Fields(text)
	var chunks []string
	for i := 0; i < len(words); i += maxWords {
		end := i + maxWords
		if end > len(words) {
			end = len(words)
		}
		chunks = append(chunks, strings.Join(words[i:end], " "))
	}
	return chunks
}
