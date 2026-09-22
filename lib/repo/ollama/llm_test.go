package ollama_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/holmes89/grey-seal/lib/repo/ollama"
	"github.com/stretchr/testify/require"
)

func TestDraftLLM_GenerateSendsModelPromptsAndStreams(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/chat", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		for _, tok := range []string{"# Design", ": X"} {
			fmt.Fprintf(w, `{"message":{"content":%q},"done":false}`+"\n", tok)
		}
		fmt.Fprintln(w, `{"message":{"content":""},"done":true}`)
	}))
	defer srv.Close()

	var tokens []string
	body, err := ollama.NewDraftLLM(srv.URL, "qwen3:8b").Generate(context.Background(), "sys", "user", func(tok string) error {
		tokens = append(tokens, tok)
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, "# Design: X", body)
	require.Equal(t, []string{"# Design", ": X"}, tokens)
	require.Equal(t, "qwen3:8b", got["model"])
	require.Equal(t, false, got["think"])
	require.Equal(t, 0.3, got["options"].(map[string]any)["temperature"])
	msgs := got["messages"].([]any)
	require.Equal(t, "system", msgs[0].(map[string]any)["role"])
	require.Equal(t, "user", msgs[1].(map[string]any)["content"])
}
