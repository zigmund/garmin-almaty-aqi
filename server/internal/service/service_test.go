package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/zigmund/garmin-almaty-aqi/server/internal/aai"
	"github.com/zigmund/garmin-almaty-aqi/server/internal/cache"
	"github.com/zigmund/garmin-almaty-aqi/server/internal/city"
)

var t0 = time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC)

// clock is a fake, goroutine-safe clock.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func newClock() *clock { return &clock{t: t0.Add(5 * time.Minute)} }

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// fakeUp is an Upstream with call counters and overridable results.
type fakeUp struct {
	mu        sync.Mutex
	districts func() (aai.Districts, error)
	mapFn     func(cc city.Code) (aai.MapSnapshot, error)
	iqair     func(cc city.Code) ([]aai.Station, error)
	delay     time.Duration // real-time delay of Map

	nDistricts, nMap, nIQAir atomic.Int32
}

func newFakeUp() *fakeUp {
	return &fakeUp{
		districts: func() (aai.Districts, error) {
			return aai.Districts{PMs: []float64{30, 40}, Time: t0}, nil
		},
		mapFn: func(cc city.Code) (aai.MapSnapshot, error) {
			b := cc.BBox()
			return aai.MapSnapshot{
				Stations:  []aai.Station{{Lat: (b[1] + b[3]) / 2, Lon: (b[0] + b[2]) / 2, PM: 20}},
				WindowEnd: t0,
			}, nil
		},
		iqair: func(cc city.Code) ([]aai.Station, error) {
			b := cc.BBox()
			return []aai.Station{{Lat: b[1] + 0.01, Lon: b[0] + 0.01, PM: 100}}, nil
		},
	}
}

func (f *fakeUp) set(fn func(f *fakeUp)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(f)
}

func (f *fakeUp) Districts(context.Context) (aai.Districts, error) {
	f.nDistricts.Add(1)
	f.mu.Lock()
	fn := f.districts
	f.mu.Unlock()
	return fn()
}

func (f *fakeUp) Map(_ context.Context, cc city.Code) (aai.MapSnapshot, error) {
	f.nMap.Add(1)
	f.mu.Lock()
	fn, d := f.mapFn, f.delay
	f.mu.Unlock()
	if d > 0 {
		time.Sleep(d)
	}
	return fn(cc)
}

func (f *fakeUp) IQAir(_ context.Context, cc city.Code) ([]aai.Station, error) {
	f.nIQAir.Add(1)
	f.mu.Lock()
	fn := f.iqair
	f.mu.Unlock()
	return fn(cc)
}

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func opts(c *clock) Options {
	return Options{
		TTL:      15 * time.Minute,
		StaleTTL: 2 * time.Hour,
		LockWait: 5 * time.Second,
		LockTTL:  21 * time.Second,
		Now:      c.Now,
		Log:      quiet,
		// FailCooldown left zero: the default (5m) applies.
	}
}

func newSvc(t *testing.T) (*Service, *fakeUp, *clock, *cache.Memory) {
	t.Helper()
	c := newClock()
	mem := cache.NewMemory(80, c.Now)
	up := newFakeUp()
	return New(mem, up, opts(c)), up, c, mem
}

func body(t *testing.T, e cache.Entry) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(e.Body, &m); err != nil {
		t.Fatalf("body %q: %v", e.Body, err)
	}
	return m
}

var ctx = context.Background()

func TestFreshHitNoUpstream(t *testing.T) {
	s, up, _, _ := newSvc(t)
	for i := 0; i < 2; i++ {
		e, err := s.Get(ctx, Glance, city.Almaty)
		if err != nil {
			t.Fatal(err)
		}
		if body(t, e)["c"] != "almaty" {
			t.Fatalf("body %s", e.Body)
		}
	}
	if n := up.nDistricts.Load(); n != 1 {
		t.Fatalf("Districts calls = %d, want 1", n)
	}
}

func TestExpiresAfterTTL(t *testing.T) {
	s, up, c, _ := newSvc(t)
	if _, err := s.Get(ctx, Glance, city.Almaty); err != nil {
		t.Fatal(err)
	}
	c.Advance(15*time.Minute - time.Second)
	if _, err := s.Get(ctx, Glance, city.Almaty); err != nil {
		t.Fatal(err)
	}
	if n := up.nDistricts.Load(); n != 1 {
		t.Fatalf("Districts calls before TTL = %d, want 1", n)
	}
	c.Advance(2 * time.Second) // 15m+1s after the fill
	e, err := s.Get(ctx, Glance, city.Almaty)
	if err != nil {
		t.Fatal(err)
	}
	if n := up.nDistricts.Load(); n != 2 {
		t.Fatalf("Districts calls = %d, want 2", n)
	}
	if !e.Fetched.Equal(c.Now()) {
		t.Fatalf("Fetched = %v, want %v", e.Fetched, c.Now())
	}
}

func TestKaragandaGroupFillsBoth(t *testing.T) {
	s, up, _, _ := newSvc(t)
	g, err := s.Get(ctx, Glance, city.Karaganda)
	if err != nil {
		t.Fatal(err)
	}
	h, err := s.Get(ctx, Heatmap, city.Karaganda)
	if err != nil {
		t.Fatal(err)
	}
	if up.nMap.Load() != 1 || up.nIQAir.Load() != 1 {
		t.Fatalf("Map %d IQAir %d, want 1 1", up.nMap.Load(), up.nIQAir.Load())
	}
	gb, hb := body(t, g), body(t, h)
	if gb["c"] != "karaganda" || hb["c"] != "karaganda" {
		t.Fatalf("bodies %s %s", g.Body, h.Body)
	}
	// Stations = map (PM 20) + IQAir (PM 100): mean 60.
	if gb["pm"] != float64(60) {
		t.Fatalf("glance pm = %v, want 60", gb["pm"])
	}
	if _, ok := hb["g"]; !ok {
		t.Fatalf("heatmap body %s", h.Body)
	}
	if s, _ := hb["s"].([]any); len(s) != 2 {
		t.Fatalf("heatmap stations = %v, want 2", hb["s"])
	}
	if !g.T.Equal(t0) || !h.T.Equal(t0) {
		t.Fatalf("T = %v %v, want WindowEnd", g.T, h.T)
	}
}

func TestKaragandaEmptyMapUsesIQAir(t *testing.T) {
	s, up, _, _ := newSvc(t)
	up.set(func(f *fakeUp) {
		f.mapFn = func(city.Code) (aai.MapSnapshot, error) { return aai.MapSnapshot{WindowEnd: t0}, nil }
	})
	g, err := s.Get(ctx, Glance, city.Karaganda)
	if err != nil {
		t.Fatal(err)
	}
	if pm := body(t, g)["pm"]; pm != float64(100) {
		t.Fatalf("pm = %v, want 100 (IQAir only)", pm)
	}
	h, err := s.Get(ctx, Heatmap, city.Karaganda)
	if err != nil {
		t.Fatal(err)
	}
	if st, _ := body(t, h)["s"].([]any); len(st) != 1 {
		t.Fatalf("heatmap stations = %v, want 1", body(t, h)["s"])
	}
	if !g.T.Equal(t0) || up.nIQAir.Load() != 1 {
		t.Fatalf("T = %v, IQAir calls %d", g.T, up.nIQAir.Load())
	}
}

func TestKaragandaEmptyUnionFails(t *testing.T) {
	s, up, _, _ := newSvc(t)
	up.set(func(f *fakeUp) {
		f.mapFn = func(city.Code) (aai.MapSnapshot, error) { return aai.MapSnapshot{WindowEnd: t0}, nil }
		f.iqair = func(city.Code) ([]aai.Station, error) { return nil, nil }
	})
	if _, err := s.Get(ctx, Glance, city.Karaganda); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
	// Failure marker: no second upstream round within the cooldown.
	if _, err := s.Get(ctx, Heatmap, city.Karaganda); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
	if up.nMap.Load() != 1 {
		t.Fatalf("Map calls = %d, want 1", up.nMap.Load())
	}
}

func TestIQAirFailureNonFatal(t *testing.T) {
	for _, iqErr := range []error{
		fmt.Errorf("%w: status 500", aai.ErrUpstream),
		fmt.Errorf("aai budget: %w", cache.ErrBudget),
	} {
		t.Run(iqErr.Error(), func(t *testing.T) {
			s, up, _, _ := newSvc(t)
			up.set(func(f *fakeUp) {
				f.iqair = func(city.Code) ([]aai.Station, error) { return nil, iqErr }
			})
			e, err := s.Get(ctx, Glance, city.Karaganda)
			if err != nil {
				t.Fatal(err)
			}
			if pm := body(t, e)["pm"]; pm != float64(20) {
				t.Fatalf("pm = %v, want 20 (map stations only)", pm)
			}
			if _, err := s.Get(ctx, Heatmap, city.Karaganda); err != nil {
				t.Fatal(err)
			}
			if up.nMap.Load() != 1 {
				t.Fatalf("Map calls = %d", up.nMap.Load())
			}
		})
	}
}

func staleCase(t *testing.T, fail error) {
	t.Helper()
	s, up, c, _ := newSvc(t)
	first, err := s.Get(ctx, Glance, city.Almaty)
	if err != nil {
		t.Fatal(err)
	}
	c.Advance(16 * time.Minute)
	up.set(func(f *fakeUp) {
		f.districts = func() (aai.Districts, error) { return aai.Districts{}, fail }
	})
	e, err := s.Get(ctx, Glance, city.Almaty)
	if err != nil {
		t.Fatalf("err = %v, want stale copy", err)
	}
	if !e.T.Equal(first.T) || !e.Fetched.Equal(first.Fetched) || string(e.Body) != string(first.Body) {
		t.Fatalf("got %+v, want stale %+v", e, first)
	}
	if n := up.nDistricts.Load(); n != 2 {
		t.Fatalf("Districts calls = %d, want 2", n)
	}
}

func TestStaleOnUpstreamError(t *testing.T) {
	staleCase(t, fmt.Errorf("%w: status 502", aai.ErrUpstream))
}

func TestStaleOnBudget(t *testing.T) {
	staleCase(t, fmt.Errorf("x: %w", cache.ErrBudget))
}

func TestNoStaleUnavailable(t *testing.T) {
	s, up, c, _ := newSvc(t)
	fail := func() (aai.Districts, error) { return aai.Districts{}, fmt.Errorf("%w: boom", aai.ErrUpstream) }
	ok := up.districts
	up.set(func(f *fakeUp) { f.districts = fail })
	if _, err := s.Get(ctx, Glance, city.Almaty); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("empty cache: err = %v, want ErrUnavailable", err)
	}
	up.set(func(f *fakeUp) { f.districts = ok })
	c.Advance(5*time.Minute + time.Second) // past the failure cooldown
	if _, err := s.Get(ctx, Glance, city.Almaty); err != nil {
		t.Fatal(err)
	}
	c.Advance(2*time.Hour + time.Second)
	up.set(func(f *fakeUp) { f.districts = fail })
	if _, err := s.Get(ctx, Glance, city.Almaty); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("after stale_ttl: err = %v, want ErrUnavailable", err)
	}
}

func TestBadDataDoesNotOverwrite(t *testing.T) {
	s, up, c, mem := newSvc(t)
	first, err := s.Get(ctx, Glance, city.Almaty)
	if err != nil {
		t.Fatal(err)
	}
	c.Advance(16 * time.Minute)
	up.set(func(f *fakeUp) {
		f.districts = func() (aai.Districts, error) { return aai.Districts{Time: t0.Add(time.Hour)}, nil }
	})
	e, err := s.Get(ctx, Glance, city.Almaty)
	if err != nil {
		t.Fatalf("err = %v, want stale copy", err)
	}
	if !e.Fetched.Equal(first.Fetched) {
		t.Fatalf("Fetched = %v, want stale %v", e.Fetched, first.Fetched)
	}
	got, ok, err := mem.Get(ctx, "glance:almaty")
	if err != nil || !ok || !got.Fetched.Equal(first.Fetched) || string(got.Body) != string(first.Body) {
		t.Fatalf("stored = %+v ok %v err %v, want untouched stale copy", got, ok, err)
	}
}

func TestBadDataNoStaleUnavailable(t *testing.T) {
	s, up, _, _ := newSvc(t)
	up.set(func(f *fakeUp) {
		f.mapFn = func(city.Code) (aai.MapSnapshot, error) { return aai.MapSnapshot{WindowEnd: t0}, nil }
	})
	if _, err := s.Get(ctx, Heatmap, city.Almaty); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
}

func TestConcurrentSingleFetch(t *testing.T) {
	s, up, _, _ := newSvc(t)
	up.delay = 100 * time.Millisecond
	var wg sync.WaitGroup
	errs := make(chan error, 50)
	for i := 0; i < 50; i++ {
		wg.Go(func() {
			if _, err := s.Get(ctx, Heatmap, city.Almaty); err != nil {
				errs <- err
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if n := up.nMap.Load(); n != 1 {
		t.Fatalf("Map calls = %d, want 1", n)
	}
}

func TestTwoReplicasOneFetch(t *testing.T) {
	mr := miniredis.RunT(t)
	c := newClock()
	up := newFakeUp()
	up.delay = 100 * time.Millisecond
	var svcs []*Service
	for i := 0; i < 2; i++ {
		rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		t.Cleanup(func() { _ = rdb.Close() })
		svcs = append(svcs, New(cache.NewRedis(rdb, "almatyaqi:", 80, c.Now), up, opts(c)))
	}
	var wg sync.WaitGroup
	errs := make(chan error, 40)
	for _, s := range svcs {
		for i := 0; i < 20; i++ {
			wg.Go(func() {
				e, err := s.Get(ctx, Heatmap, city.Almaty)
				if err == nil && e.Fetched.IsZero() {
					err = errors.New("empty entry")
				}
				if err != nil {
					errs <- err
				}
			})
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if n := up.nMap.Load(); n != 1 {
		t.Fatalf("Map calls = %d, want 1", n)
	}
	if mr.Exists("almatyaqi:lock:almaty-map") {
		t.Fatal("lock not released")
	}
}

func TestLockHeldServesStale(t *testing.T) {
	c := newClock()
	mem := cache.NewMemory(80, c.Now)
	up := newFakeUp()
	o := opts(c)
	o.LockWait = 300 * time.Millisecond
	s := New(mem, up, o)
	first, err := s.Get(ctx, Glance, city.Almaty)
	if err != nil {
		t.Fatal(err)
	}
	c.Advance(16 * time.Minute)
	// Another replica holds the group lock and never finishes.
	unlock, ok, err := mem.Lock(ctx, "almaty-glance", time.Hour)
	if err != nil || !ok {
		t.Fatalf("lock: ok %v err %v", ok, err)
	}
	defer unlock()
	start := time.Now()
	e, err := s.Get(ctx, Glance, city.Almaty)
	if err != nil {
		t.Fatal(err)
	}
	if el := time.Since(start); el < 300*time.Millisecond {
		t.Fatalf("returned after %v, want to wait LockWait", el)
	}
	if !e.Fetched.Equal(first.Fetched) {
		t.Fatalf("want stale copy, got %+v", e)
	}
	if n := up.nDistricts.Load(); n != 1 {
		t.Fatalf("Districts calls = %d, want 1 (lock held elsewhere)", n)
	}
	// Without a stale copy the wait ends in ErrUnavailable.
	if _, err := s.Get(ctx, Heatmap, city.Almaty); err != nil {
		t.Fatal(err) // different group, not locked
	}
	unlock2, ok, _ := mem.Lock(ctx, "karaganda", time.Hour)
	if !ok {
		t.Fatal("karaganda lock")
	}
	defer unlock2()
	if _, err := s.Get(ctx, Glance, city.Karaganda); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
}

func TestWaiterSeesNeighbourFill(t *testing.T) {
	c := newClock()
	mem := cache.NewMemory(80, c.Now)
	up := newFakeUp()
	s := New(mem, up, opts(c))
	unlock, ok, _ := mem.Lock(ctx, "almaty-glance", time.Hour)
	if !ok {
		t.Fatal("lock")
	}
	want := cache.Entry{Body: []byte(`{"c":"almaty"}`), T: t0, Fetched: c.Now()}
	go func() {
		time.Sleep(250 * time.Millisecond)
		_ = mem.Set(ctx, "glance:almaty", want, 2*time.Hour)
		unlock()
	}()
	e, err := s.Get(ctx, Glance, city.Almaty)
	if err != nil {
		t.Fatal(err)
	}
	if string(e.Body) != string(want.Body) {
		t.Fatalf("got %s", e.Body)
	}
	if up.nDistricts.Load() != 0 {
		t.Fatal("upstream called while neighbour held the lock")
	}
}

// errStore fails the selected operation.
type errStore struct {
	cache.Store
	getErr, setErr, lockErr bool
}

var errStoreDown = errors.New("store down")

func (s *errStore) Get(ctx context.Context, key string) (cache.Entry, bool, error) {
	if s.getErr {
		return cache.Entry{}, false, errStoreDown
	}
	return s.Store.Get(ctx, key)
}

func (s *errStore) Set(ctx context.Context, key string, e cache.Entry, ttl time.Duration) error {
	if s.setErr {
		return errStoreDown
	}
	return s.Store.Set(ctx, key, e, ttl)
}

func (s *errStore) Lock(ctx context.Context, name string, ttl time.Duration) (func(), bool, error) {
	if s.lockErr {
		return nil, false, errStoreDown
	}
	return s.Store.Lock(ctx, name, ttl)
}

func TestStoreErrorInternal(t *testing.T) {
	cases := map[string]*errStore{
		"get":  {getErr: true},
		"lock": {lockErr: true},
	}
	for name, st := range cases {
		t.Run(name, func(t *testing.T) {
			c := newClock()
			st.Store = cache.NewMemory(80, c.Now)
			up := newFakeUp()
			s := New(st, up, opts(c))
			for _, k := range []Kind{Glance, Heatmap} {
				for _, cc := range []city.Code{city.Almaty, city.Karaganda} {
					if _, err := s.Get(ctx, k, cc); !errors.Is(err, ErrInternal) {
						t.Fatalf("%s/%s: err = %v, want ErrInternal", k, cc, err)
					}
				}
			}
			if n := up.nDistricts.Load() + up.nMap.Load() + up.nIQAir.Load(); n != 0 {
				t.Fatalf("upstream calls = %d, want 0", n)
			}
		})
	}
	t.Run("set", func(t *testing.T) {
		c := newClock()
		s := New(&errStore{Store: cache.NewMemory(80, c.Now), setErr: true}, newFakeUp(), opts(c))
		if _, err := s.Get(ctx, Glance, city.Almaty); !errors.Is(err, ErrInternal) {
			t.Fatalf("err = %v, want ErrInternal", err)
		}
	})
}

func TestBudgetStoreErrorInternal(t *testing.T) {
	// A Redis failure inside Budget.Take arrives wrapped and is neither
	// ErrBudget nor aai.ErrUpstream: it must be internal, not served stale.
	redisDown := fmt.Errorf("aai budget: %w", errors.New("dial tcp: connection refused"))
	t.Run("districts", func(t *testing.T) {
		s, up, c, _ := newSvc(t)
		if _, err := s.Get(ctx, Glance, city.Almaty); err != nil {
			t.Fatal(err)
		}
		c.Advance(16 * time.Minute)
		up.set(func(f *fakeUp) {
			f.districts = func() (aai.Districts, error) { return aai.Districts{}, redisDown }
		})
		if _, err := s.Get(ctx, Glance, city.Almaty); !errors.Is(err, ErrInternal) {
			t.Fatalf("err = %v, want ErrInternal", err)
		}
	})
	t.Run("iqair", func(t *testing.T) {
		s, up, _, _ := newSvc(t)
		up.set(func(f *fakeUp) {
			f.iqair = func(city.Code) ([]aai.Station, error) { return nil, redisDown }
		})
		if _, err := s.Get(ctx, Heatmap, city.Karaganda); !errors.Is(err, ErrInternal) {
			t.Fatalf("err = %v, want ErrInternal", err)
		}
	})
}

func TestMapFailureSkipsIQAir(t *testing.T) {
	s, up, _, _ := newSvc(t)
	up.set(func(f *fakeUp) {
		f.mapFn = func(city.Code) (aai.MapSnapshot, error) {
			return aai.MapSnapshot{}, fmt.Errorf("%w: status 500", aai.ErrUpstream)
		}
	})
	if _, err := s.Get(ctx, Glance, city.Karaganda); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
	if n := up.nIQAir.Load(); n != 0 {
		t.Fatalf("IQAir calls = %d, want 0 after Map failure", n)
	}
}

func TestEntriesWrittenWithStaleTTL(t *testing.T) {
	s, _, c, mem := newSvc(t)
	if _, err := s.Get(ctx, Glance, city.Karaganda); err != nil {
		t.Fatal(err)
	}
	c.Advance(2*time.Hour - time.Second)
	for _, k := range []string{"glance:karaganda", "heatmap:karaganda"} {
		if _, ok, _ := mem.Get(ctx, k); !ok {
			t.Fatalf("%s expired before StaleTTL", k)
		}
	}
	c.Advance(2 * time.Second)
	for _, k := range []string{"glance:karaganda", "heatmap:karaganda"} {
		if _, ok, _ := mem.Get(ctx, k); ok {
			t.Fatalf("%s outlived StaleTTL", k)
		}
	}
}

func TestUnknownKindInternal(t *testing.T) {
	s, _, _, _ := newSvc(t)
	if _, err := s.Get(ctx, Kind("nope"), city.Almaty); !errors.Is(err, ErrInternal) {
		t.Fatalf("err = %v, want ErrInternal", err)
	}
	if _, err := s.Get(ctx, Glance, city.Code("nope")); !errors.Is(err, ErrInternal) {
		t.Fatalf("err = %v, want ErrInternal", err)
	}
}

func TestCallerCancelServesStale(t *testing.T) {
	s, up, c, _ := newSvc(t)
	first, err := s.Get(ctx, Heatmap, city.Almaty)
	if err != nil {
		t.Fatal(err)
	}
	c.Advance(16 * time.Minute)
	up.set(func(f *fakeUp) { f.delay = 300 * time.Millisecond })
	cctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	e, err := s.Get(cctx, Heatmap, city.Almaty)
	if err != nil {
		t.Fatal(err)
	}
	if !e.Fetched.Equal(first.Fetched) {
		t.Fatalf("want stale copy, got %+v", e)
	}
	// The fill continues in the background and lands in the cache.
	deadline := time.Now().Add(2 * time.Second)
	for {
		e, err := s.Get(ctx, Heatmap, city.Almaty)
		if err != nil {
			t.Fatal(err)
		}
		if e.Fetched.Equal(c.Now()) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("background fill did not land")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if n := up.nMap.Load(); n != 2 {
		t.Fatalf("Map calls = %d, want 2", n)
	}
}

func TestCallerDeadlineNoStaleUnavailable(t *testing.T) {
	// The HTTP handler maps context errors to 502, so a done caller context
	// must yield ErrUnavailable (no stale copy), never a context error.
	s, up, _, _ := newSvc(t)
	up.delay = 300 * time.Millisecond
	cctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	_, err := s.Get(cctx, Heatmap, city.Almaty)
	if !errors.Is(err, ErrUnavailable) || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want plain ErrUnavailable", err)
	}
}

func TestFailCooldown(t *testing.T) {
	s, up, c, mem := newSvc(t)
	first, err := s.Get(ctx, Glance, city.Almaty)
	if err != nil {
		t.Fatal(err)
	}
	c.Advance(16 * time.Minute)
	up.set(func(f *fakeUp) {
		f.districts = func() (aai.Districts, error) { return aai.Districts{}, fmt.Errorf("%w: 500", aai.ErrUpstream) }
	})
	for i := 0; i < 5; i++ {
		e, err := s.Get(ctx, Glance, city.Almaty)
		if err != nil || !e.Fetched.Equal(first.Fetched) {
			t.Fatalf("get %d: %+v %v, want stale copy", i, e, err)
		}
		c.Advance(59 * time.Second) // 5 gets span < 5m
	}
	if n := up.nDistricts.Load(); n != 2 {
		t.Fatalf("Districts calls = %d, want 2 (1 fill + 1 failure)", n)
	}
	// Cooldown applies before taking the lock: no lock was left behind
	// and the marker is in the store.
	if _, ok, _ := mem.Get(ctx, "fail:almaty-glance"); !ok {
		t.Fatal("no failure marker in the store")
	}
	c.Advance(time.Minute + time.Second) // > 5m since the failure
	if _, err := s.Get(ctx, Glance, city.Almaty); err != nil {
		t.Fatal(err)
	}
	if n := up.nDistricts.Load(); n != 3 {
		t.Fatalf("Districts calls = %d, want 3 after cooldown", n)
	}
}

func TestFailCooldownBadDataAndBudget(t *testing.T) {
	for name, fn := range map[string]func() (aai.MapSnapshot, error){
		"bad data": func() (aai.MapSnapshot, error) { return aai.MapSnapshot{WindowEnd: t0}, nil },
		"budget":   func() (aai.MapSnapshot, error) { return aai.MapSnapshot{}, fmt.Errorf("x: %w", cache.ErrBudget) },
	} {
		t.Run(name, func(t *testing.T) {
			s, up, c, _ := newSvc(t)
			up.set(func(f *fakeUp) {
				f.mapFn = func(city.Code) (aai.MapSnapshot, error) { return fn() }
				f.iqair = func(city.Code) ([]aai.Station, error) { return nil, nil } // no stations at all
			})
			for i := 0; i < 3; i++ {
				if _, err := s.Get(ctx, Heatmap, city.Karaganda); !errors.Is(err, ErrUnavailable) {
					t.Fatalf("err = %v, want ErrUnavailable", err)
				}
				if _, err := s.Get(ctx, Glance, city.Karaganda); !errors.Is(err, ErrUnavailable) {
					t.Fatalf("err = %v, want ErrUnavailable", err)
				}
				c.Advance(time.Minute)
			}
			if n := up.nMap.Load(); n != 1 {
				t.Fatalf("Map calls = %d, want 1", n)
			}
		})
	}
}

func TestFailCooldownTwoReplicas(t *testing.T) {
	mr := miniredis.RunT(t)
	c := newClock()
	up := newFakeUp()
	up.set(func(f *fakeUp) {
		f.districts = func() (aai.Districts, error) { return aai.Districts{}, fmt.Errorf("%w: 500", aai.ErrUpstream) }
	})
	var svcs []*Service
	for i := 0; i < 2; i++ {
		rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		t.Cleanup(func() { _ = rdb.Close() })
		svcs = append(svcs, New(cache.NewRedis(rdb, "almatyaqi:", 80, c.Now), up, opts(c)))
	}
	for i := 0; i < 6; i++ {
		if _, err := svcs[i%2].Get(ctx, Glance, city.Almaty); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("get %d: err = %v, want ErrUnavailable", i, err)
		}
		c.Advance(30 * time.Second)
	}
	if n := up.nDistricts.Load(); n != 1 {
		t.Fatalf("Districts calls = %d, want 1", n)
	}
	c.Advance(5 * time.Minute)
	_, _ = svcs[1].Get(ctx, Glance, city.Almaty)
	if n := up.nDistricts.Load(); n != 2 {
		t.Fatalf("Districts calls = %d, want 2 after cooldown", n)
	}
}

func TestFailMarkerStoreErrorInternal(t *testing.T) {
	c := newClock()
	st := &errStore{Store: cache.NewMemory(80, c.Now)}
	up := newFakeUp()
	up.set(func(f *fakeUp) {
		f.districts = func() (aai.Districts, error) { return aai.Districts{}, fmt.Errorf("%w: 500", aai.ErrUpstream) }
	})
	s := New(st, up, opts(c))
	// Writing the marker fails.
	st.setErr = true
	if _, err := s.Get(ctx, Glance, city.Almaty); !errors.Is(err, ErrInternal) {
		t.Fatalf("marker write: err = %v, want ErrInternal", err)
	}
}

// neighbourStore simulates another replica finishing a fill right before
// this replica obtains the group lock.
type neighbourStore struct {
	cache.Store
	now func() time.Time
}

func (s *neighbourStore) Lock(ctx context.Context, name string, ttl time.Duration) (func(), bool, error) {
	e := cache.Entry{Body: []byte(`{"c":"almaty","n":1}`), T: t0, Fetched: s.now()}
	if err := s.Store.Set(ctx, "glance:almaty", e, 2*time.Hour); err != nil {
		return nil, false, err
	}
	return s.Store.Lock(ctx, name, ttl)
}

func TestRereadUnderLock(t *testing.T) {
	c := newClock()
	up := newFakeUp()
	s := New(&neighbourStore{Store: cache.NewMemory(80, c.Now), now: c.Now}, up, opts(c))
	e, err := s.Get(ctx, Glance, city.Almaty)
	if err != nil {
		t.Fatal(err)
	}
	if body(t, e)["n"] != float64(1) {
		t.Fatalf("got %s, want the neighbour's entry", e.Body)
	}
	if n := up.nDistricts.Load(); n != 0 {
		t.Fatalf("Districts calls = %d, want 0", n)
	}
}

func TestPanicInFillRecovered(t *testing.T) {
	s, up, _, _ := newSvc(t)
	ok := up.districts
	up.set(func(f *fakeUp) {
		f.districts = func() (aai.Districts, error) { panic("boom") }
	})
	if _, err := s.Get(ctx, Glance, city.Almaty); !errors.Is(err, ErrInternal) {
		t.Fatalf("err = %v, want ErrInternal", err)
	}
	up.set(func(f *fakeUp) { f.districts = ok })
	// The lock was released and no flight is stuck: the next call fills.
	e, err := s.Get(ctx, Glance, city.Almaty)
	if err != nil {
		t.Fatal(err)
	}
	if body(t, e)["c"] != "almaty" {
		t.Fatalf("body %s", e.Body)
	}
}

func TestFillBoundedByLockTTL(t *testing.T) {
	c := newClock()
	up := &ctxUp{fakeUp: newFakeUp()}
	o := opts(c)
	o.LockTTL = 200 * time.Millisecond
	s := New(cache.NewMemory(80, c.Now), up, o)
	start := time.Now()
	if _, err := s.Get(ctx, Glance, city.Almaty); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
	if el := time.Since(start); el > 2*time.Second {
		t.Fatalf("fill ran %v, want it cut at LockTTL", el)
	}
}

// ctxUp blocks Districts until its context is done, like an HTTP request
// to a hung upstream (aai wraps that as ErrUpstream).
type ctxUp struct{ *fakeUp }

func (u *ctxUp) Districts(ctx context.Context) (aai.Districts, error) {
	u.nDistricts.Add(1)
	<-ctx.Done()
	return aai.Districts{}, fmt.Errorf("%w: timeout_or_canceled", aai.ErrUpstream)
}
