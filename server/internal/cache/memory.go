package cache

import (
	"context"
	"sync"
	"time"
)

const budgetWindow = time.Hour

type memEntry struct {
	e       Entry
	expires time.Time
}

type memLock struct {
	token   uint64
	expires time.Time
}

// Memory is an in-process Store and Budget. Expiry and the budget window use
// the injected clock.
type Memory struct {
	perHour int
	now     func() time.Time

	mu      sync.Mutex
	entries map[string]memEntry
	locks   map[string]memLock
	nextTok uint64
	takes   []time.Time // successful Take times, ascending
}

// NewMemory returns a Memory limited to perHour Take calls per sliding hour.
func NewMemory(perHour int, now func() time.Time) *Memory {
	return &Memory{
		perHour: perHour,
		now:     now,
		entries: map[string]memEntry{},
		locks:   map[string]memLock{},
	}
}

// Get implements Store.
func (m *Memory) Get(_ context.Context, key string) (Entry, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	me, ok := m.entries[key]
	if !ok {
		return Entry{}, false, nil
	}
	if !m.now().Before(me.expires) {
		delete(m.entries, key)
		return Entry{}, false, nil
	}
	return me.e, true, nil
}

// Set implements Store.
func (m *Memory) Set(_ context.Context, key string, e Entry, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[key] = memEntry{e: e, expires: m.now().Add(ttl)}
	return nil
}

// Lock implements Store.
func (m *Memory) Lock(_ context.Context, name string, ttl time.Duration) (func(), bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	if l, held := m.locks[name]; held && now.Before(l.expires) {
		return nil, false, nil
	}
	m.nextTok++
	tok := m.nextTok
	m.locks[name] = memLock{token: tok, expires: now.Add(ttl)}
	return func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if l, ok := m.locks[name]; ok && l.token == tok {
			delete(m.locks, name)
		}
	}, true, nil
}

// Take implements Budget.
func (m *Memory) Take(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	cutoff := now.Add(-budgetWindow)
	i := 0
	for i < len(m.takes) && !m.takes[i].After(cutoff) {
		i++
	}
	m.takes = m.takes[i:]
	if len(m.takes) >= m.perHour {
		return ErrBudget
	}
	m.takes = append(m.takes, now)
	return nil
}
