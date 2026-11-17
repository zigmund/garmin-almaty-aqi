// Package cache defines storage interfaces for ready HTTP responses and for
// the upstream request budget, plus an in-memory implementation.
package cache

import (
	"context"
	"errors"
	"time"
)

// ErrBudget is returned by Budget.Take when the hourly limit is reached.
var ErrBudget = errors.New("aai budget exhausted")

// Entry is a cached ready response.
type Entry struct {
	Body    []byte    `json:"b"` // response JSON object without the "a" field
	T       time.Time `json:"t"` // data time
	Fetched time.Time `json:"f"` // when built (fresh = now - Fetched < cache.ttl)
}

// Store keeps entries and short-lived locks.
type Store interface {
	// Get returns the entry for key; ok=false if absent or expired.
	Get(ctx context.Context, key string) (e Entry, ok bool, err error)
	// Set stores e under key for ttl.
	Set(ctx context.Context, key string, e Entry, ttl time.Duration) error
	// Lock acquires a named lock for ttl; ok=false if it is held.
	// unlock releases only the lock acquired by this call.
	Lock(ctx context.Context, name string, ttl time.Duration) (unlock func(), ok bool, err error)
}

// Budget caps upstream requests (sliding one hour window).
type Budget interface {
	// Take consumes one request; ErrBudget if the limit is already reached.
	Take(ctx context.Context) error
}
