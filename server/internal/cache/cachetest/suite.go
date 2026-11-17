// Package cachetest holds the contract test suite shared by all cache
// implementations. The budget limit under test is 80 per hour.
package cachetest

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zigmund/garmin-airmon/server/internal/cache"
)

// PerHour is the budget limit every implementation must be built with.
const PerHour = 80

// Impl is a full cache implementation.
type Impl interface {
	cache.Store
	cache.Budget
}

// RunSuite runs the contract tests. newImpl must return a fresh instance
// limited to PerHour requests per hour, and advance, which moves the
// implementation's notion of time forward (never sleeps).
func RunSuite(t *testing.T, newImpl func(t *testing.T) (impl Impl, advance func(d time.Duration))) {
	ctx := context.Background()
	t0 := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	entry := cache.Entry{Body: []byte(`{"x":1}`), T: t0, Fetched: t0}

	t.Run("GetMissing", func(t *testing.T) {
		impl, _ := newImpl(t)
		_, ok, err := impl.Get(ctx, "nope")
		if err != nil || ok {
			t.Fatalf("Get missing = ok %v err %v", ok, err)
		}
	})

	t.Run("SetGetRoundTrip", func(t *testing.T) {
		impl, _ := newImpl(t)
		if err := impl.Set(ctx, "k", entry, time.Minute); err != nil {
			t.Fatal(err)
		}
		got, ok, err := impl.Get(ctx, "k")
		if err != nil || !ok {
			t.Fatalf("Get = ok %v err %v", ok, err)
		}
		if string(got.Body) != string(entry.Body) || !got.T.Equal(entry.T) || !got.Fetched.Equal(entry.Fetched) {
			t.Fatalf("got %+v, want %+v", got, entry)
		}
	})

	t.Run("Expiry", func(t *testing.T) {
		impl, advance := newImpl(t)
		if err := impl.Set(ctx, "k", entry, time.Minute); err != nil {
			t.Fatal(err)
		}
		advance(59 * time.Second)
		if _, ok, _ := impl.Get(ctx, "k"); !ok {
			t.Fatal("entry expired too early")
		}
		advance(2 * time.Second)
		if _, ok, _ := impl.Get(ctx, "k"); ok {
			t.Fatal("entry still present after ttl")
		}
	})

	t.Run("SetOverwrites", func(t *testing.T) {
		impl, _ := newImpl(t)
		_ = impl.Set(ctx, "k", entry, time.Minute)
		e2 := entry
		e2.Body = []byte(`{"x":2}`)
		_ = impl.Set(ctx, "k", e2, time.Minute)
		got, _, _ := impl.Get(ctx, "k")
		if string(got.Body) != `{"x":2}` {
			t.Fatalf("got %s", got.Body)
		}
	})

	t.Run("LockExclusiveAndRelease", func(t *testing.T) {
		impl, _ := newImpl(t)
		unlock, ok, err := impl.Lock(ctx, "l", time.Minute)
		if err != nil || !ok {
			t.Fatalf("first Lock = ok %v err %v", ok, err)
		}
		_, ok, err = impl.Lock(ctx, "l", time.Minute)
		if err != nil || ok {
			t.Fatalf("second Lock = ok %v err %v, want ok=false", ok, err)
		}
		if _, ok, _ := impl.Lock(ctx, "other", time.Minute); !ok {
			t.Fatal("independent lock name must be acquirable")
		}
		unlock()
		unlock2, ok, err := impl.Lock(ctx, "l", time.Minute)
		if err != nil || !ok {
			t.Fatalf("Lock after unlock = ok %v err %v", ok, err)
		}
		unlock2()
	})

	t.Run("LockExpires", func(t *testing.T) {
		impl, advance := newImpl(t)
		if _, ok, _ := impl.Lock(ctx, "l", 10*time.Second); !ok {
			t.Fatal("Lock failed")
		}
		advance(11 * time.Second)
		if _, ok, _ := impl.Lock(ctx, "l", 10*time.Second); !ok {
			t.Fatal("expired lock must be acquirable")
		}
	})

	t.Run("StaleUnlockKeepsNewLock", func(t *testing.T) {
		impl, advance := newImpl(t)
		oldUnlock, ok, _ := impl.Lock(ctx, "l", 10*time.Second)
		if !ok {
			t.Fatal("Lock failed")
		}
		advance(11 * time.Second)
		newUnlock, ok, _ := impl.Lock(ctx, "l", time.Minute)
		if !ok {
			t.Fatal("second owner Lock failed")
		}
		oldUnlock() // must not release the new owner's lock
		if _, ok, _ := impl.Lock(ctx, "l", time.Minute); ok {
			t.Fatal("stale unlock released a lock it does not own")
		}
		newUnlock()
		if _, ok, _ := impl.Lock(ctx, "l", time.Minute); !ok {
			t.Fatal("own unlock did not release")
		}
	})

	t.Run("UnlockTwiceIsSafe", func(t *testing.T) {
		impl, _ := newImpl(t)
		u1, _, _ := impl.Lock(ctx, "l", time.Minute)
		u1()
		_, ok, _ := impl.Lock(ctx, "l", time.Minute)
		if !ok {
			t.Fatal("Lock after unlock failed")
		}
		u1() // second call of the old unlock must not release the new lock
		if _, ok, _ := impl.Lock(ctx, "l", time.Minute); ok {
			t.Fatal("repeated unlock released a foreign lock")
		}
	})

	t.Run("BudgetLimit", func(t *testing.T) {
		impl, _ := newImpl(t)
		for i := 0; i < PerHour; i++ {
			if err := impl.Take(ctx); err != nil {
				t.Fatalf("Take #%d: %v", i+1, err)
			}
		}
		if err := impl.Take(ctx); !errors.Is(err, cache.ErrBudget) {
			t.Fatalf("Take #%d = %v, want ErrBudget", PerHour+1, err)
		}
	})

	t.Run("BudgetWindowSlides", func(t *testing.T) {
		impl, advance := newImpl(t)
		for i := 0; i < PerHour; i++ {
			if err := impl.Take(ctx); err != nil {
				t.Fatal(err)
			}
		}
		advance(30 * time.Minute)
		if err := impl.Take(ctx); !errors.Is(err, cache.ErrBudget) {
			t.Fatalf("Take inside window = %v, want ErrBudget", err)
		}
		advance(30*time.Minute + time.Second) // 1h+1s after the first Takes
		if err := impl.Take(ctx); err != nil {
			t.Fatalf("Take after window: %v", err)
		}
	})

	t.Run("BudgetPartialSlide", func(t *testing.T) {
		impl, advance := newImpl(t)
		for i := 0; i < 40; i++ {
			_ = impl.Take(ctx)
		}
		advance(40 * time.Minute)
		for i := 0; i < 40; i++ {
			if err := impl.Take(ctx); err != nil {
				t.Fatal(err)
			}
		}
		if err := impl.Take(ctx); !errors.Is(err, cache.ErrBudget) {
			t.Fatalf("got %v, want ErrBudget", err)
		}
		advance(21*time.Minute + time.Second) // first batch is 1h+1s old, second 21m
		for i := 0; i < 40; i++ {
			if err := impl.Take(ctx); err != nil {
				t.Fatalf("Take #%d after first batch left window: %v", i+1, err)
			}
		}
		if err := impl.Take(ctx); !errors.Is(err, cache.ErrBudget) {
			t.Fatalf("got %v, want ErrBudget", err)
		}
	})

	t.Run("BudgetConcurrent", func(t *testing.T) {
		impl, _ := newImpl(t)
		var ok, denied, other atomic.Int64
		var wg sync.WaitGroup
		for i := 0; i < 200; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				switch err := impl.Take(ctx); {
				case err == nil:
					ok.Add(1)
				case errors.Is(err, cache.ErrBudget):
					denied.Add(1)
				default:
					other.Add(1)
				}
			}()
		}
		wg.Wait()
		if ok.Load() != PerHour || denied.Load() != 200-PerHour || other.Load() != 0 {
			t.Fatalf("ok=%d denied=%d other=%d, want %d/%d/0", ok.Load(), denied.Load(), other.Load(), PerHour, 200-PerHour)
		}
	})
}
