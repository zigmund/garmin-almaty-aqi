// Package service serves ready responses from the cache and refreshes them
// from the upstream API in fill groups, so that the number of upstream
// requests does not depend on the number of keys, callers or replicas.
//
// Get algorithm: a fresh cached entry is returned as is. Otherwise callers of
// one group are collapsed into a single in-process fill (singleflight), which
// takes the group lock in the store (shared between replicas). The lock holder
// re-reads the group (a neighbour may have refreshed it), and only if it is
// still not fresh fetches the sources, builds every key of the group and
// writes them all with StaleTTL. A fill that does not get the lock polls the
// store until the group is fresh or LockWait elapses.
//
// When the refresh fails (upstream error, exhausted budget, bad upstream
// data, lock wait timeout, caller gone) the existing stale entry is served, or
// ErrUnavailable if there is none. A failed fetch also leaves a failure marker
// `fail:<group>` in the store for FailCooldown: while it is present no replica
// locks the group or calls the upstream for it, so one failing source cannot
// drain the shared upstream budget. Any store error is ErrInternal, and the
// upstream is never called after a store error, so a broken shared store
// cannot multiply the upstream request rate by the number of replicas.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/zigmund/garmin-almaty-aqi/server/internal/aai"
	"github.com/zigmund/garmin-almaty-aqi/server/internal/cache"
	"github.com/zigmund/garmin-almaty-aqi/server/internal/city"
	"github.com/zigmund/garmin-almaty-aqi/server/internal/respond"
)

// Kind is the response kind.
type Kind string

const (
	Glance  Kind = "glance"
	Heatmap Kind = "heatmap"
)

// Upstream is the AAI API (aai.Client).
type Upstream interface {
	Districts(ctx context.Context) (aai.Districts, error)
	Map(ctx context.Context, cc city.Code) (aai.MapSnapshot, error)
	IQAir(ctx context.Context, cc city.Code) ([]aai.Station, error)
}

// Options configures a Service.
type Options struct {
	TTL      time.Duration // an entry is fresh while now - Fetched < TTL
	StaleTTL time.Duration // store TTL of every written entry (stale copy lifetime)
	LockWait time.Duration // how long to wait for another updater of the group
	LockTTL  time.Duration // group lock TTL; also the hard limit of one fill
	// FailCooldown is how long a failed fetch blocks new fetches of its
	// group on all replicas; zero means DefaultFailCooldown.
	FailCooldown time.Duration
	Now          func() time.Time
	Log          *slog.Logger // nil means slog.Default()
}

var (
	// ErrUnavailable means there is neither a fresh nor a stale entry.
	ErrUnavailable = errors.New("unavailable")
	// ErrInternal means a store failure or a programming error.
	ErrInternal = errors.New("internal")
)

// DefaultFailCooldown is used when Options.FailCooldown is zero.
const DefaultFailCooldown = 5 * time.Minute

// markerWriteTimeout bounds the failure marker write, which is done outside
// the (possibly expired) fill context.
const markerWriteTimeout = 2 * time.Second

// pollInterval is how often a waiter re-reads a group locked by another
// updater. It runs on real time, not on Options.Now.
const pollInterval = 200 * time.Millisecond

// errRefresh marks a failed refresh that falls back to the stale copy.
var errRefresh = errors.New("refresh failed")

// group is a fill group: keys that are built from the same upstream requests
// and always written together.
type group struct {
	name string
	keys []string
	// fetch calls the upstream and builds an entry for every key.
	fetch func(ctx context.Context, now time.Time) (map[string]cache.Entry, error)
}

// flight is one in-process fill of a group.
type flight struct {
	done    chan struct{}
	entries map[string]cache.Entry // fresh entries of the group, on success
	err     error                  // ErrInternal or errRefresh
}

// Service serves cached responses and refreshes them.
type Service struct {
	store cache.Store
	up    Upstream
	o     Options
	log   *slog.Logger

	mu      sync.Mutex
	flights map[string]*flight
}

// New creates a Service.
func New(store cache.Store, up Upstream, o Options) *Service {
	log := o.Log
	if log == nil {
		log = slog.Default()
	}
	if o.FailCooldown == 0 {
		o.FailCooldown = DefaultFailCooldown
	}
	return &Service{store: store, up: up, o: o, log: log, flights: map[string]*flight{}}
}

// Get returns the response of kind k for city cc: fresh if possible, else the
// stale copy. Errors are ErrUnavailable or ErrInternal.
func (s *Service) Get(ctx context.Context, k Kind, cc city.Code) (cache.Entry, error) {
	g, ok := s.groupFor(k, cc)
	if !ok {
		s.log.Error("service: unknown response", "kind", string(k), "city", string(cc))
		return cache.Entry{}, ErrInternal
	}
	key := string(k) + ":" + string(cc)
	now := s.o.Now()
	cur, have, err := s.store.Get(ctx, key)
	if err != nil {
		s.storeError("get", g.name, err)
		return cache.Entry{}, ErrInternal
	}
	if have && s.fresh(cur, now) {
		return cur, nil
	}
	fallback := func() (cache.Entry, error) {
		// Guard against a store clock that disagrees with Options.Now.
		if have && s.o.Now().Sub(cur.Fetched) < s.o.StaleTTL {
			return cur, nil
		}
		return cache.Entry{}, ErrUnavailable
	}

	f := s.start(g)
	select {
	case <-f.done:
	case <-ctx.Done():
		// The fill keeps running for the other callers and the cache.
		return fallback()
	}
	if e, ok := f.entries[key]; ok {
		return e, nil
	}
	if errors.Is(f.err, ErrInternal) {
		return cache.Entry{}, ErrInternal
	}
	return fallback()
}

func (s *Service) fresh(e cache.Entry, now time.Time) bool {
	return now.Sub(e.Fetched) < s.o.TTL
}

// start joins the running fill of g or starts a new one. The fill runs
// detached from any caller's cancellation, so a caller that gives up does
// not fail the others; its duration is bounded by LockTTL (and by the
// upstream client timeouts within it).
func (s *Service) start(g group) *flight {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f, ok := s.flights[g.name]; ok {
		return f
	}
	f := &flight{done: make(chan struct{})}
	s.flights[g.name] = f
	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.log.Error("service: fill panicked", "group", g.name, "panic", fmt.Sprint(r))
				f.entries, f.err = nil, ErrInternal
			}
			s.mu.Lock()
			delete(s.flights, g.name)
			s.mu.Unlock()
			close(f.done)
		}()
		// A fill never outlives its lock.
		ctx, cancel := context.WithTimeout(context.Background(), s.o.LockTTL)
		defer cancel()
		f.entries, f.err = s.fill(ctx, g)
	}()
	return f
}

// fill refreshes group g across replicas. It returns fresh entries for every
// key of g, or ErrInternal / errRefresh.
func (s *Service) fill(ctx context.Context, g group) (map[string]cache.Entry, error) {
	if cooling, err := s.coolingDown(ctx, g); err != nil || cooling {
		return nil, coolErr(err)
	}
	unlock, ok, err := s.store.Lock(ctx, g.name, s.o.LockTTL)
	if err != nil {
		s.storeError("lock", g.name, err)
		return nil, ErrInternal
	}
	if !ok {
		return s.wait(ctx, g)
	}
	defer unlock()

	// A neighbour may have refreshed the group, or failed to, before we got
	// the lock.
	if es, ok, err := s.readFresh(ctx, g); err != nil || ok {
		return es, err
	}
	if cooling, err := s.coolingDown(ctx, g); err != nil || cooling {
		return nil, coolErr(err)
	}

	now := s.o.Now()
	es, err := g.fetch(ctx, now)
	if err != nil {
		err = s.classify(g.name, err)
		if errors.Is(err, errRefresh) {
			return nil, s.markFailed(ctx, g, now)
		}
		return nil, err
	}
	// No marker can exist here (checked under the lock above), so a
	// successful fill has nothing to clear.
	for _, k := range g.keys {
		if err := s.store.Set(ctx, k, es[k], s.o.StaleTTL); err != nil {
			s.storeError("set", g.name, err)
			return nil, ErrInternal
		}
	}
	return es, nil
}

// wait polls the store until another updater has refreshed g or LockWait
// elapses (real time).
func (s *Service) wait(ctx context.Context, g group) (map[string]cache.Entry, error) {
	deadline := time.NewTimer(s.o.LockWait)
	defer deadline.Stop()
	tick := time.NewTicker(pollInterval)
	defer tick.Stop()
	for {
		select {
		case <-deadline.C:
			s.log.Warn("service: lock wait timed out", "group", g.name)
			return nil, errRefresh
		case <-ctx.Done():
			return nil, errRefresh
		case <-tick.C:
		}
		if es, ok, err := s.readFresh(ctx, g); err != nil || ok {
			return es, err
		}
		// The updater failed: stop waiting, serve stale.
		if cooling, err := s.coolingDown(ctx, g); err != nil || cooling {
			return nil, coolErr(err)
		}
	}
}

func failKey(g group) string { return "fail:" + g.name }

// coolingDown reports whether a recent failed fetch of g blocks fetching.
// The marker age is also checked against Options.Now, so a store clock that
// runs behind cannot extend the cooldown.
func (s *Service) coolingDown(ctx context.Context, g group) (bool, error) {
	m, ok, err := s.store.Get(ctx, failKey(g))
	if err != nil {
		s.storeError("get", g.name, err)
		return false, ErrInternal
	}
	return ok && s.o.Now().Sub(m.Fetched) < s.o.FailCooldown, nil
}

func coolErr(err error) error {
	if err != nil {
		return err
	}
	return errRefresh
}

// markFailed records a failed fetch of g for FailCooldown and returns
// errRefresh, or ErrInternal if the marker cannot be written. The write
// uses its own short context because the fill context may have expired.
func (s *Service) markFailed(ctx context.Context, g group, now time.Time) error {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), markerWriteTimeout)
	defer cancel()
	if err := s.store.Set(wctx, failKey(g), cache.Entry{T: now, Fetched: now}, s.o.FailCooldown); err != nil {
		s.storeError("set", g.name, err)
		return ErrInternal
	}
	return errRefresh
}

// readFresh returns the entries of g if all of them are present and fresh.
func (s *Service) readFresh(ctx context.Context, g group) (map[string]cache.Entry, bool, error) {
	now := s.o.Now()
	es := make(map[string]cache.Entry, len(g.keys))
	for _, k := range g.keys {
		e, ok, err := s.store.Get(ctx, k)
		if err != nil {
			s.storeError("get", g.name, err)
			return nil, false, ErrInternal
		}
		if !ok || !s.fresh(e, now) {
			return nil, false, nil
		}
		es[k] = e
	}
	return es, true, nil
}

// classify maps a fetch error to errRefresh (serve stale) or ErrInternal.
// Budget denials, upstream failures and bad upstream data are refresh
// failures. Anything else that comes out of the upstream client is neither
// ErrBudget nor aai.ErrUpstream: in practice a failure of the shared budget
// store (Redis) inside Budget.Take, which is treated as a store error.
func (s *Service) classify(name string, err error) error {
	switch {
	case errors.Is(err, cache.ErrBudget):
		s.log.Warn("service: upstream budget exhausted", "group", name)
		return errRefresh
	case errors.Is(err, aai.ErrUpstream):
		s.log.Warn("service: upstream failed", "group", name, "error", err.Error())
		return errRefresh
	case errors.Is(err, errBadData):
		s.log.Warn("service: bad upstream data", "group", name, "error", err.Error())
		return errRefresh
	case errors.Is(err, context.DeadlineExceeded):
		// The fill hit its LockTTL limit (e.g. inside Budget.Take).
		s.log.Warn("service: fill timed out", "group", name)
		return errRefresh
	}
	s.storeError("budget", name, err)
	return ErrInternal
}

// storeError logs a store failure without keys or bodies.
func (s *Service) storeError(op, name string, err error) {
	s.log.Error("service: store failed", "op", op, "group", name, "error", err.Error())
}

// isInternal reports whether an upstream error is a budget store failure
// (see classify).
func isInternal(err error) bool {
	return !errors.Is(err, cache.ErrBudget) && !errors.Is(err, aai.ErrUpstream)
}

// errBadData marks upstream data that respond refused to build from (empty
// station list, zero data time). The stale copy is kept.
var errBadData = errors.New("bad upstream data")

func badData(err error) error { return fmt.Errorf("%w: %v", errBadData, err) }

func (s *Service) groupFor(k Kind, cc city.Code) (group, bool) {
	switch {
	case cc == city.Almaty && k == Glance:
		return group{name: "almaty-glance", keys: []string{"glance:almaty"}, fetch: s.fetchDistricts}, true
	case cc == city.Almaty && k == Heatmap:
		return group{name: "almaty-map", keys: []string{"heatmap:almaty"}, fetch: s.fetchAlmatyMap}, true
	case cc == city.Karaganda && (k == Glance || k == Heatmap):
		return group{name: "karaganda", keys: []string{"glance:karaganda", "heatmap:karaganda"}, fetch: s.fetchKaraganda}, true
	}
	return group{}, false
}

func (s *Service) fetchDistricts(ctx context.Context, now time.Time) (map[string]cache.Entry, error) {
	d, err := s.up.Districts(ctx)
	if err != nil {
		return nil, err
	}
	e, err := respond.GlanceAlmaty(d, now)
	if err != nil {
		return nil, badData(err)
	}
	return map[string]cache.Entry{"glance:almaty": e}, nil
}

func (s *Service) fetchAlmatyMap(ctx context.Context, now time.Time) (map[string]cache.Entry, error) {
	m, err := s.up.Map(ctx, city.Almaty)
	if err != nil {
		return nil, err
	}
	e, err := respond.Heatmap(city.Almaty, m.Stations, m.WindowEnd, now)
	if err != nil {
		return nil, badData(err)
	}
	return map[string]cache.Entry{"heatmap:almaty": e}, nil
}

// fetchKaraganda builds both Karaganda keys from map ∪ IQAir stations, with
// the data time of the map. The map may have no stations (IQAir alone is
// enough); only an empty union is bad data. An IQAir failure (including a budget denial)
// only drops the IQAir stations; a budget store failure is still fatal.
func (s *Service) fetchKaraganda(ctx context.Context, now time.Time) (map[string]cache.Entry, error) {
	m, err := s.up.Map(ctx, city.Karaganda)
	if err != nil {
		return nil, err
	}
	st := m.Stations
	iq, err := s.up.IQAir(ctx, city.Karaganda)
	switch {
	case err == nil:
		st = append(st[:len(st):len(st)], iq...)
	case isInternal(err):
		return nil, err
	default:
		s.log.Warn("service: iqair skipped", "group", "karaganda", "error", err.Error())
	}
	g, err := respond.GlanceStations(city.Karaganda, st, m.WindowEnd, now)
	if err != nil {
		return nil, badData(err)
	}
	h, err := respond.Heatmap(city.Karaganda, st, m.WindowEnd, now)
	if err != nil {
		return nil, badData(err)
	}
	return map[string]cache.Entry{"glance:karaganda": g, "heatmap:karaganda": h}, nil
}
