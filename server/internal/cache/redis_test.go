package cache_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/zigmund/garmin-almaty-aqi/server/internal/cache"
	"github.com/zigmund/garmin-almaty-aqi/server/internal/cache/cachetest"
)

func newTestRedis(t *testing.T, prefix string) (*cache.Redis, *miniredis.Miniredis, func(time.Duration)) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	var mu sync.Mutex
	clock := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	now := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return clock
	}
	advance := func(d time.Duration) {
		mu.Lock()
		clock = clock.Add(d)
		mu.Unlock()
		mr.FastForward(d)
	}
	return cache.NewRedis(rdb, prefix, cachetest.PerHour, now), mr, advance
}

func TestRedisSuite(t *testing.T) {
	cachetest.RunSuite(t, func(t *testing.T) (cachetest.Impl, func(time.Duration)) {
		r, _, advance := newTestRedis(t, "almatyaqi:")
		return r, advance
	})
}

func TestRedisDownReturnsError(t *testing.T) {
	ctx := context.Background()
	r, mr, _ := newTestRedis(t, "almatyaqi:")
	mr.Close()

	if _, _, err := r.Get(ctx, "k"); err == nil || errors.Is(err, cache.ErrBudget) {
		t.Fatalf("Get err = %v", err)
	}
	if err := r.Set(ctx, "k", cache.Entry{}, time.Minute); err == nil || errors.Is(err, cache.ErrBudget) {
		t.Fatalf("Set err = %v", err)
	}
	if _, ok, err := r.Lock(ctx, "l", time.Minute); err == nil || ok || errors.Is(err, cache.ErrBudget) {
		t.Fatalf("Lock ok=%v err = %v", ok, err)
	}
	if err := r.Take(ctx); err == nil || errors.Is(err, cache.ErrBudget) {
		t.Fatalf("Take err = %v", err)
	}
}

func TestRedisPrefix(t *testing.T) {
	ctx := context.Background()
	r, mr, _ := newTestRedis(t, "almatyaqi:")
	if err := r.Set(ctx, "glance:almaty", cache.Entry{Body: []byte(`{}`)}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if !mr.Exists("almatyaqi:v1:glance:almaty") {
		t.Fatalf("keys = %v", mr.Keys())
	}
	if _, ok, _ := r.Lock(ctx, "g", time.Minute); !ok {
		t.Fatal("lock")
	}
	if !mr.Exists("almatyaqi:lock:g") {
		t.Fatalf("keys = %v", mr.Keys())
	}
	if err := r.Take(ctx); err != nil {
		t.Fatal(err)
	}
	if !mr.Exists("almatyaqi:budget") {
		t.Fatalf("keys = %v", mr.Keys())
	}
}
