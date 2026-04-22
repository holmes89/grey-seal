package transcript

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/holmes89/grey-seal/lib/greyseal/conversation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTurn(convUUID string, idx int, user, response string) conversation.TranscriptTurn {
	return conversation.TranscriptTurn{
		ConversationUUID: convUUID,
		TurnIndex:        idx,
		Timestamp:        time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC),
		UserMessage:      user,
		Response:         response,
	}
}

func TestWriter_CreatesFileOnFirstWrite(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	require.NoError(t, err)
	defer w.Close()

	err = w.WriteTurn(context.Background(), newTurn("conv-1", 1, "hello", "world"))
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, "conv-1.md"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "## Turn 1")
	assert.Contains(t, string(content), "hello")
	assert.Contains(t, string(content), "world")
}

func TestWriter_TwoCallsAppendTwoSections(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	require.NoError(t, err)
	defer w.Close()

	ctx := context.Background()
	require.NoError(t, w.WriteTurn(ctx, newTurn("conv-2", 1, "first", "answer one")))
	require.NoError(t, w.WriteTurn(ctx, newTurn("conv-2", 2, "second", "answer two")))

	content, err := os.ReadFile(filepath.Join(dir, "conv-2.md"))
	require.NoError(t, err)
	assert.Equal(t, 2, strings.Count(string(content), "## Turn"))
	assert.Contains(t, string(content), "first")
	assert.Contains(t, string(content), "second")
}

func TestWriter_DifferentUUIDsCreateSeparateFiles(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	require.NoError(t, err)
	defer w.Close()

	ctx := context.Background()
	require.NoError(t, w.WriteTurn(ctx, newTurn("conv-a", 1, "q1", "r1")))
	require.NoError(t, w.WriteTurn(ctx, newTurn("conv-b", 1, "q2", "r2")))

	_, errA := os.Stat(filepath.Join(dir, "conv-a.md"))
	_, errB := os.Stat(filepath.Join(dir, "conv-b.md"))
	assert.NoError(t, errA)
	assert.NoError(t, errB)
}

func TestWriter_EmptySummaryOmitsSection(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	require.NoError(t, err)
	defer w.Close()

	turn := newTurn("conv-3", 1, "hi", "bye")
	turn.ConversationSummary = ""
	require.NoError(t, w.WriteTurn(context.Background(), turn))

	content, err := os.ReadFile(filepath.Join(dir, "conv-3.md"))
	require.NoError(t, err)
	assert.NotContains(t, string(content), "**Conversation summary**")
}

func TestWriter_EmptySearchResultsOmitsTable(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	require.NoError(t, err)
	defer w.Close()

	turn := newTurn("conv-4", 1, "hi", "bye")
	turn.SearchResults = nil
	require.NoError(t, w.WriteTurn(context.Background(), turn))

	content, err := os.ReadFile(filepath.Join(dir, "conv-4.md"))
	require.NoError(t, err)
	assert.NotContains(t, string(content), "**Shrike search**")
}

func TestWriter_EmptyResourceUUIDsOmitsCitedLine(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	require.NoError(t, err)
	defer w.Close()

	turn := newTurn("conv-5", 1, "hi", "bye")
	turn.ResourceUUIDs = nil
	require.NoError(t, w.WriteTurn(context.Background(), turn))

	content, err := os.ReadFile(filepath.Join(dir, "conv-5.md"))
	require.NoError(t, err)
	assert.NotContains(t, string(content), "**Resources cited**")
}
