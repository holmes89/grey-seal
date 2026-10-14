// Package draftstore implements conversation.DraftStore: draft bodies and
// chain offers kept in Redis (or, with no Redis configured, in process
// memory) with a TTL.
package draftstore

import (
	"context"
	"sync"
	"time"

	"github.com/holmes89/grey-seal/lib/greyseal/conversation"
)

var _ conversation.DraftStore = (*Memory)(nil)

type entry struct {
	draft   string
	offer   conversation.ChainOffer
	expires time.Time
}

// Memory is an in-process DraftStore with a TTL. Its contents are lost on
// restart; it is the fallback when REDIS_URL is not set.
type Memory struct {
	ttl time.Duration
	now func() time.Time

	mu     sync.Mutex
	drafts map[string]entry
	offers map[string]entry
}

// SetClock replaces the clock used for expiry; for tests.
func (m *Memory) SetClock(now func() time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = now
}

// NewMemory returns a Memory store whose entries live for ttl.
func NewMemory(ttl time.Duration) *Memory {
	return &Memory{ttl: ttl, now: time.Now, drafts: map[string]entry{}, offers: map[string]entry{}}
}

// sweep drops expired entries. Callers hold m.mu.
func (m *Memory) sweep() {
	now := m.now()
	for k, e := range m.drafts {
		if !now.Before(e.expires) {
			delete(m.drafts, k)
		}
	}
	for k, e := range m.offers {
		if !now.Before(e.expires) {
			delete(m.offers, k)
		}
	}
}

func (m *Memory) PutDraft(_ context.Context, messageUUID, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sweep()
	m.drafts[messageUUID] = entry{draft: body, expires: m.now().Add(m.ttl)}
	return nil
}

func (m *Memory) GetDraft(_ context.Context, messageUUID string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sweep()
	e, ok := m.drafts[messageUUID]
	return e.draft, ok, nil
}

func (m *Memory) DeleteDraft(_ context.Context, messageUUID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.drafts, messageUUID)
	return nil
}

func (m *Memory) PutOffer(_ context.Context, conversationUUID string, offer conversation.ChainOffer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sweep()
	m.offers[conversationUUID] = entry{offer: offer, expires: m.now().Add(m.ttl)}
	return nil
}

func (m *Memory) GetOffer(_ context.Context, conversationUUID string) (conversation.ChainOffer, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sweep()
	e, ok := m.offers[conversationUUID]
	return e.offer, ok, nil
}

func (m *Memory) DeleteOffer(_ context.Context, conversationUUID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.offers, conversationUUID)
	return nil
}
