package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/holmes89/grey-seal/lib/greyseal/conversation"
	"github.com/redis/go-redis/v9"
)

const cacheTTL = 24 * time.Hour

var _ conversation.ResourceCache = (*RedisResourceCache)(nil)

// RedisResourceCache stores per-conversation resource context in Redis.
// Each conversation has one key holding a JSON map of entity_uuid → CachedResource.
type RedisResourceCache struct {
	client *redis.Client
}

// NewRedisResourceCache creates a cache backed by the given Redis client.
func NewRedisResourceCache(client *redis.Client) *RedisResourceCache {
	return &RedisResourceCache{client: client}
}

func cacheKey(conversationUUID string) string {
	return fmt.Sprintf("greyseal:conv:%s:resources", conversationUUID)
}

// Merge upserts resources into the cache. For each incoming resource, if no
// entry exists for that entity_uuid or the incoming Score is higher, the entry
// is overwritten. The TTL is reset on every successful write.
func (c *RedisResourceCache) Merge(ctx context.Context, conversationUUID string, resources []conversation.CachedResource) error {
	if len(resources) == 0 {
		return nil
	}
	key := cacheKey(conversationUUID)

	// Load existing entries.
	existing := map[string]conversation.CachedResource{}
	raw, err := c.client.Get(ctx, key).Bytes()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("cache get: %w", err)
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &existing); err != nil {
			// Corrupt entry — reset rather than fail.
			existing = map[string]conversation.CachedResource{}
		}
	}

	// Merge: keep the higher-scored snippet for each entity.
	for _, r := range resources {
		if prev, ok := existing[r.EntityUUID]; !ok || r.Score > prev.Score {
			existing[r.EntityUUID] = r
		}
	}

	data, err := json.Marshal(existing)
	if err != nil {
		return fmt.Errorf("cache marshal: %w", err)
	}
	return c.client.Set(ctx, key, data, cacheTTL).Err()
}

// List returns all cached resources for the conversation sorted by Score descending.
func (c *RedisResourceCache) List(ctx context.Context, conversationUUID string) ([]conversation.CachedResource, error) {
	raw, err := c.client.Get(ctx, cacheKey(conversationUUID)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cache get: %w", err)
	}

	m := map[string]conversation.CachedResource{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("cache unmarshal: %w", err)
	}

	result := make([]conversation.CachedResource, 0, len(m))
	for _, r := range m {
		result = append(result, r)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Score > result[j].Score
	})
	return result, nil
}
