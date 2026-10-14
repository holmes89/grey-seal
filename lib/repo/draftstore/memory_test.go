package draftstore_test

import (
	"context"
	"testing"
	"time"

	"github.com/holmes89/grey-seal/lib/greyseal/conversation"
	"github.com/holmes89/grey-seal/lib/repo/draftstore"
	"github.com/stretchr/testify/require"
)

// runStoreContract is shared with the Redis integration test.
func runStoreContract(t *testing.T, store conversation.DraftStore, advance func(time.Duration)) {
	ctx := context.Background()

	t.Run("draft round trip and delete", func(t *testing.T) {
		require.NoError(t, store.PutDraft(ctx, "m1", "# Design: X"))
		body, ok, err := store.GetDraft(ctx, "m1")
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, "# Design: X", body)

		require.NoError(t, store.DeleteDraft(ctx, "m1"))
		_, ok, err = store.GetDraft(ctx, "m1")
		require.NoError(t, err)
		require.False(t, ok)
	})

	t.Run("missing entries are not errors", func(t *testing.T) {
		_, ok, err := store.GetDraft(ctx, "nope")
		require.NoError(t, err)
		require.False(t, ok)
		_, ok, err = store.GetOffer(ctx, "nope")
		require.NoError(t, err)
		require.False(t, ok)
		require.NoError(t, store.DeleteDraft(ctx, "nope"))
		require.NoError(t, store.DeleteOffer(ctx, "nope"))
	})

	t.Run("offer round trip, replace, delete", func(t *testing.T) {
		require.NoError(t, store.PutOffer(ctx, "c1", conversation.ChainOffer{WorkType: "design", Source: "disc"}))
		require.NoError(t, store.PutOffer(ctx, "c1", conversation.ChainOffer{WorkType: "tickets", Source: "des"}))
		offer, ok, err := store.GetOffer(ctx, "c1")
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, conversation.ChainOffer{WorkType: "tickets", Source: "des"}, offer)

		require.NoError(t, store.DeleteOffer(ctx, "c1"))
		_, ok, err = store.GetOffer(ctx, "c1")
		require.NoError(t, err)
		require.False(t, ok)
	})

	if advance == nil {
		return
	}

	t.Run("entries expire after the ttl", func(t *testing.T) {
		require.NoError(t, store.PutDraft(ctx, "m2", "body"))
		require.NoError(t, store.PutOffer(ctx, "c2", conversation.ChainOffer{WorkType: "design", Source: "s"}))

		advance(30 * time.Minute)
		_, ok, _ := store.GetDraft(ctx, "m2")
		require.True(t, ok, "still alive before the ttl")

		advance(31 * time.Minute)
		_, ok, err := store.GetDraft(ctx, "m2")
		require.NoError(t, err)
		require.False(t, ok, "draft expired")
		_, ok, err = store.GetOffer(ctx, "c2")
		require.NoError(t, err)
		require.False(t, ok, "offer expired")
	})

	t.Run("writing refreshes the ttl", func(t *testing.T) {
		require.NoError(t, store.PutDraft(ctx, "m3", "v1"))
		advance(45 * time.Minute)
		require.NoError(t, store.PutDraft(ctx, "m3", "v2"))
		advance(45 * time.Minute)
		body, ok, err := store.GetDraft(ctx, "m3")
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, "v2", body)
	})
}

func TestMemoryStore(t *testing.T) {
	now := time.Now()
	m := draftstore.NewMemory(time.Hour)
	m.SetClock(func() time.Time { return now })
	runStoreContract(t, m, func(d time.Duration) { now = now.Add(d) })
}
