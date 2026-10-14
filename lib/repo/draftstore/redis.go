package draftstore

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/holmes89/grey-seal/lib/greyseal/conversation"
	"github.com/redis/go-redis/v9"
)

var _ conversation.DraftStore = (*Redis)(nil)

// Redis is a DraftStore backed by Redis. Every key carries the TTL, set on
// write, so nothing needs cleaning up.
type Redis struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedis returns a Redis-backed store whose keys expire after ttl.
func NewRedis(client *redis.Client, ttl time.Duration) *Redis {
	return &Redis{client: client, ttl: ttl}
}

func draftKey(messageUUID string) string { return "greyseal:draft:" + messageUUID }
func offerKey(conversationUUID string) string {
	return "greyseal:conv:" + conversationUUID + ":chain"
}

func (r *Redis) PutDraft(ctx context.Context, messageUUID, body string) error {
	if err := r.client.Set(ctx, draftKey(messageUUID), body, r.ttl).Err(); err != nil {
		return fmt.Errorf("store draft: %w", err)
	}
	return nil
}

func (r *Redis) GetDraft(ctx context.Context, messageUUID string) (string, bool, error) {
	body, err := r.client.Get(ctx, draftKey(messageUUID)).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("load draft: %w", err)
	}
	return body, true, nil
}

func (r *Redis) DeleteDraft(ctx context.Context, messageUUID string) error {
	if err := r.client.Del(ctx, draftKey(messageUUID)).Err(); err != nil {
		return fmt.Errorf("delete draft: %w", err)
	}
	return nil
}

type offerJSON struct {
	WorkType string `json:"work_type"`
	Source   string `json:"source"`
}

func (r *Redis) PutOffer(ctx context.Context, conversationUUID string, offer conversation.ChainOffer) error {
	raw, err := json.Marshal(offerJSON{WorkType: offer.WorkType, Source: offer.Source})
	if err != nil {
		return err
	}
	if err := r.client.Set(ctx, offerKey(conversationUUID), raw, r.ttl).Err(); err != nil {
		return fmt.Errorf("store chain offer: %w", err)
	}
	return nil
}

func (r *Redis) GetOffer(ctx context.Context, conversationUUID string) (conversation.ChainOffer, bool, error) {
	raw, err := r.client.Get(ctx, offerKey(conversationUUID)).Bytes()
	if err == redis.Nil {
		return conversation.ChainOffer{}, false, nil
	}
	if err != nil {
		return conversation.ChainOffer{}, false, fmt.Errorf("load chain offer: %w", err)
	}
	var o offerJSON
	if err := json.Unmarshal(raw, &o); err != nil || o.WorkType == "" {
		// Unreadable entry: treat as no offer rather than blocking chat.
		return conversation.ChainOffer{}, false, nil
	}
	return conversation.ChainOffer{WorkType: o.WorkType, Source: o.Source}, true, nil
}

func (r *Redis) DeleteOffer(ctx context.Context, conversationUUID string) error {
	if err := r.client.Del(ctx, offerKey(conversationUUID)).Err(); err != nil {
		return fmt.Errorf("delete chain offer: %w", err)
	}
	return nil
}
