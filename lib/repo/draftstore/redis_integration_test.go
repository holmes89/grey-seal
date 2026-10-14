//go:build integration

package draftstore_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/holmes89/grey-seal/lib/repo/draftstore"
	"github.com/ory/dockertest/v3"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

var redisAddr string

func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		fmt.Fprintln(os.Stderr, "dockertest:", err)
		os.Exit(1)
	}
	res, err := pool.Run("redis", "7-alpine", nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "start redis:", err)
		os.Exit(1)
	}
	redisAddr = res.GetHostPort("6379/tcp")
	if err := pool.Retry(func() error {
		c := redis.NewClient(&redis.Options{Addr: redisAddr})
		defer c.Close() //nolint:errcheck
		return c.Ping(context.Background()).Err()
	}); err != nil {
		_ = pool.Purge(res)
		fmt.Fprintln(os.Stderr, "redis not ready:", err)
		os.Exit(1)
	}
	code := m.Run()
	_ = pool.Purge(res)
	os.Exit(code)
}

// Redis expires keys on its own clock, so this runs a real (short) TTL
// instead of the shared contract's fake clock.
func TestRedisStore(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: redisAddr})
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()

	t.Run("contract without expiry", func(t *testing.T) {
		store := draftstore.NewRedis(client, time.Hour)
		runStoreContract(t, store, nil) // nil: no fake clock, so the expiry cases are skipped
	})

	t.Run("keys carry the ttl and really expire", func(t *testing.T) {
		store := draftstore.NewRedis(client, 1500*time.Millisecond)
		require.NoError(t, store.PutDraft(ctx, "ttl-m", "body"))
		ttl, err := client.PTTL(ctx, "greyseal:draft:ttl-m").Result()
		require.NoError(t, err)
		require.Greater(t, ttl, time.Duration(0))
		require.LessOrEqual(t, ttl, 1500*time.Millisecond)

		require.Eventually(t, func() bool {
			_, ok, err := store.GetDraft(ctx, "ttl-m")
			return err == nil && !ok
		}, 5*time.Second, 200*time.Millisecond)
	})
}
