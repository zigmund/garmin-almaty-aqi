package cache_test

import (
	"sync"
	"testing"
	"time"

	"github.com/zigmund/garmin-almaty-aqi/server/internal/cache"
	"github.com/zigmund/garmin-almaty-aqi/server/internal/cache/cachetest"
)

func TestMemoryContract(t *testing.T) {
	cachetest.RunSuite(t, func(t *testing.T) (cachetest.Impl, func(time.Duration)) {
		var mu sync.Mutex
		now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		clock := func() time.Time {
			mu.Lock()
			defer mu.Unlock()
			return now
		}
		advance := func(d time.Duration) {
			mu.Lock()
			defer mu.Unlock()
			now = now.Add(d)
		}
		return cache.NewMemory(cachetest.PerHour, clock), advance
	})
}
