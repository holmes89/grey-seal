//go:build ignore

package config

import (
	"os"
)

var (
	// ApiEndpoint is the base URL for the API
	ApiEndpoint = getEnv("API_URL", "http://localhost:9000")
)

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
