package aai

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zigmund/garmin-almaty-aqi/server/internal/cache"
	"github.com/zigmund/garmin-almaty-aqi/server/internal/city"
)

type budgetFunc func(context.Context) error

func (f budgetFunc) Take(ctx context.Context) error { return f(ctx) }

var allow = budgetFunc(func(context.Context) error { return nil })

func newClient(url string, timeout time.Duration, b Budget) *Client {
	return newClientLog(url, timeout, b, nil)
}

func newClientLog(url string, timeout time.Duration, b Budget, log *slog.Logger) *Client {
	return New(url, "sekret-123", timeout, nil, b, func() time.Time { return time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC) }, log)
}

func TestClientSendsKeyHeader(t *testing.T) {
	var gotKey, gotURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey, gotURI = r.Header.Get("x-api-key"), r.URL.RequestURI()
		w.Write(fixture(t, "map.json"))
	}))
	defer srv.Close()
	m, err := newClient(srv.URL, time.Second, allow).Map(context.Background(), city.Karaganda)
	if err != nil || len(m.Stations) != 2 {
		t.Fatalf("m=%v err=%v", m, err)
	}
	if gotKey != "sekret-123" || gotURI != "/measurements/map?city=karaganda" {
		t.Errorf("key=%q uri=%q", gotKey, gotURI)
	}
}

func TestClientEndpoints(t *testing.T) {
	var uris []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uris = append(uris, r.URL.RequestURI())
		if strings.HasPrefix(r.URL.Path, "/iqair") {
			w.Write(fixture(t, "iqair.json"))
			return
		}
		w.Write(fixture(t, "districts.json"))
	}))
	defer srv.Close()
	c := newClient(srv.URL, time.Second, allow)
	if d, err := c.Districts(context.Background()); err != nil || len(d.PMs) != 8 {
		t.Fatalf("d=%v err=%v", d, err)
	}
	// now = 12:00, station "a" ts 09:00 is exactly 3h old: kept (not older than 3h)
	if s, err := c.IQAir(context.Background(), city.Karaganda); err != nil || len(s) != 1 {
		t.Fatalf("s=%v err=%v", s, err)
	}
	if len(uris) != 2 || uris[0] != "/measurements/districts?lang=ru" || uris[1] != "/iqair/current?city=karaganda" {
		t.Errorf("uris = %v", uris)
	}
}

func TestClientBudgetDenied(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	defer srv.Close()
	denied := errors.New("denied")
	_, err := newClient(srv.URL, time.Second, budgetFunc(func(context.Context) error { return denied })).Districts(context.Background())
	if !errors.Is(err, denied) || hits.Load() != 0 {
		t.Errorf("err=%v hits=%d", err, hits.Load())
	}
}

func TestClientNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }))
	defer srv.Close()
	_, err := newClient(srv.URL, time.Second, allow).Map(context.Background(), city.Almaty)
	if !errors.Is(err, ErrUpstream) {
		t.Errorf("err = %v", err)
	}
}

func TestClientTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	_, err := newClient(srv.URL, 50*time.Millisecond, allow).Map(context.Background(), city.Almaty)
	if !errors.Is(err, ErrUpstream) {
		t.Errorf("err = %v", err)
	}
}

func TestClientNeverLogsKey(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("city") {
		case "karaganda":
			time.Sleep(300 * time.Millisecond)
		case "almaty":
			w.Write(fixture(t, "map.json"))
			return
		}
		w.WriteHeader(500)
	}))
	defer srv.Close()
	var errs []error
	_, e0 := newClientLog(srv.URL, time.Second, allow, log).Map(context.Background(), city.Almaty) // ok
	_, e1 := newClientLog(srv.URL, time.Second, allow, log).Districts(context.Background())        // 500
	_, e2 := newClientLog(srv.URL, 50*time.Millisecond, allow, log).Map(context.Background(), city.Karaganda)
	if e0 != nil {
		t.Fatal(e0)
	}
	errs = append(errs, e1, e2)
	out := buf.String()
	if n := strings.Count(out, `msg="request out" dir=out upstream=aai`); n != 3 {
		t.Errorf("want 3 'request out' lines, got %d: %s", n, out)
	}
	for _, want := range []string{"path=/measurements/map", "status=200", "outcome=ok", "outcome=bad_status", "outcome=error:timeout_or_canceled", "duration="} {
		if !strings.Contains(out, want) {
			t.Errorf("log lacks %q: %s", want, out)
		}
	}
	for _, bad := range []string{"sekret-123", "city=", "lang=", srv.URL, "x-api-key"} {
		if strings.Contains(out, bad) {
			t.Errorf("log leaks %q: %s", bad, out)
		}
	}
	for _, e := range errs {
		if e == nil || strings.Contains(e.Error(), "sekret-123") || strings.Contains(e.Error(), srv.URL) {
			t.Errorf("bad error: %v", e)
		}
	}
}

func TestClientLogsBudgetDenial(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	b := budgetFunc(func(context.Context) error { return cache.ErrBudget })
	_, err := newClientLog("http://127.0.0.1:1", time.Second, b, log).Districts(context.Background())
	if !errors.Is(err, cache.ErrBudget) {
		t.Fatalf("err = %v", err)
	}
	if out := buf.String(); !strings.Contains(out, "level=WARN") || !strings.Contains(out, "budget denied") || strings.Contains(out, "sekret-123") {
		t.Errorf("log = %s", out)
	}
}
