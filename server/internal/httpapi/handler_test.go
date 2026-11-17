package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zigmund/garmin-airmon/server/internal/cache"
	"github.com/zigmund/garmin-airmon/server/internal/city"
	"github.com/zigmund/garmin-airmon/server/internal/service"
)

var t0 = time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)

type call struct {
	k  service.Kind
	cc city.Code
}

type fakeGetter struct {
	calls []call
	e     cache.Entry
	err   error
}

func (f *fakeGetter) Get(_ context.Context, k service.Kind, cc city.Code) (cache.Entry, error) {
	f.calls = append(f.calls, call{k, cc})
	return f.e, f.err
}

func newFake() *fakeGetter {
	return &fakeGetter{e: cache.Entry{Body: []byte(`{"m":"city"}`), T: t0.Add(-412 * time.Second)}}
}

func do(t *testing.T, g Getter, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	h := New(g, func() time.Time { return t0 }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func TestGlanceByPosition(t *testing.T) {
	g := newFake()
	rec := do(t, g, "GET", "/v1/glance?lat=43.24&lon=76.92")
	if rec.Code != 200 || rec.Body.String() != `{"m":"city","a":412}` {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
	if len(g.calls) != 1 || g.calls[0] != (call{service.Glance, city.Almaty}) {
		t.Fatalf("calls %v", g.calls)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type %q", ct)
	}
}

func TestOutByPosition(t *testing.T) {
	g := newFake()
	rec := do(t, g, "GET", "/v1/glance?lat=55.75&lon=37.62")
	if rec.Code != 200 || rec.Body.String() != `{"m":"out"}` {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
	if len(g.calls) != 0 {
		t.Fatalf("Get called: %v", g.calls)
	}
}

func TestNoPosition(t *testing.T) {
	g := newFake()
	rec := do(t, g, "GET", "/v1/glance?city=karaganda")
	if rec.Code != 200 || len(g.calls) != 1 || g.calls[0].cc != city.Karaganda {
		t.Fatalf("karaganda: %d %v", rec.Code, g.calls)
	}
	g = newFake()
	rec = do(t, g, "GET", "/v1/glance?city=out")
	if rec.Body.String() != `{"m":"out"}` || len(g.calls) != 0 {
		t.Fatalf("out: %s %v", rec.Body, g.calls)
	}
	rec = do(t, g, "GET", "/v1/glance")
	if rec.Code != 200 || rec.Body.String() != `{"m":"nopos"}` || len(g.calls) != 0 {
		t.Fatalf("nopos: %d %s", rec.Code, rec.Body)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("cache-control %q", cc)
	}
}

func TestPositionWinsOverCity(t *testing.T) {
	g := newFake()
	rec := do(t, g, "GET", "/v1/glance?lat=55.75&lon=37.62&city=almaty")
	if rec.Body.String() != `{"m":"out"}` || len(g.calls) != 0 {
		t.Fatalf("%s %v", rec.Body, g.calls)
	}
}

func TestUnknownCityIgnoredWithPosition(t *testing.T) {
	g := newFake()
	rec := do(t, g, "GET", "/v1/glance?lat=43.25&lon=76.9&city=astana")
	if rec.Code != 200 || len(g.calls) != 1 || g.calls[0].cc != city.Almaty {
		t.Fatalf("%d %s %v", rec.Code, rec.Body, g.calls)
	}
}

func TestBadRequests(t *testing.T) {
	for _, q := range []string{
		"lat=1", "lon=1", "lat=NaN&lon=1", "lat=1&lon=Inf", "lat=91&lon=1", "lat=1&lon=181",
		"lat=x&lon=1", "city=astana", "city=Almaty",
	} {
		for _, p := range []string{"/v1/glance", "/v1/heatmap"} {
			g := newFake()
			rec := do(t, g, "GET", p+"?"+q)
			if rec.Code != 400 || rec.Body.String() != `{"err":"bad_request"}` || len(g.calls) != 0 {
				t.Errorf("%s?%s: %d %s", p, q, rec.Code, rec.Body)
			}
		}
	}
}

func TestErrors(t *testing.T) {
	g := newFake()
	g.err = service.ErrUnavailable
	rec := do(t, g, "GET", "/v1/glance?city=almaty")
	if rec.Code != 502 || rec.Body.String() != `{"err":"upstream"}` {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	g.err = service.ErrInternal
	rec = do(t, g, "GET", "/v1/glance?city=almaty")
	if rec.Code != 500 || rec.Body.String() != `{"err":"internal"}` {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	g.err = io.ErrUnexpectedEOF
	rec = do(t, g, "GET", "/v1/glance?city=almaty")
	if rec.Code != 500 {
		t.Fatalf("unknown error: %d", rec.Code)
	}
}

func TestWithAge(t *testing.T) {
	got := WithAge([]byte(`{"m":"city"}`), t0.Add(-412*time.Second), t0)
	if string(got) != `{"m":"city","a":412}` {
		t.Fatal(string(got))
	}
	got = WithAge([]byte(`{"m":"city"}`), t0.Add(time.Hour), t0)
	if string(got) != `{"m":"city","a":0}` {
		t.Fatal(string(got))
	}
	if got = WithAge([]byte("{}"), t0, t0); string(got) != `{"a":0}` {
		t.Fatal(string(got))
	}
}

func TestWithAgeKeepsInput(t *testing.T) {
	// Spare capacity makes an in-place append possible; it must not happen.
	buf := make([]byte, 0, 64)
	buf = append(buf, `{"m":"city"}`...)
	orig := string(buf)
	_ = WithAge(buf, t0.Add(-5*time.Second), t0)
	if string(buf) != orig || string(buf[:cap(buf)][:len(orig)+8]) != orig+"\x00\x00\x00\x00\x00\x00\x00\x00" {
		t.Fatalf("input modified: %q", buf[:cap(buf)][:len(orig)+8])
	}
}

func TestHealthz(t *testing.T) {
	g := newFake()
	rec := do(t, g, "GET", "/healthz")
	if rec.Code != 200 || rec.Body.String() != "ok" || len(g.calls) != 0 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestCacheControl(t *testing.T) {
	g := newFake()
	if cc := do(t, g, "GET", "/v1/glance?city=almaty").Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("glance %q", cc)
	}
	rec := do(t, g, "GET", "/v1/heatmap?city=almaty")
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=60" {
		t.Fatalf("heatmap %q", cc)
	}
	if g.calls[1].k != service.Heatmap {
		t.Fatalf("kind %v", g.calls[1])
	}
	rec = do(t, g, "GET", "/v1/heatmap?city=out")
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("heatmap out %q", cc)
	}
	rec = do(t, g, "GET", "/v1/glance?city=almaty&lat=1")
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("error %q", cc)
	}
}

func TestRoutes(t *testing.T) {
	g := newFake()
	rec := do(t, g, "GET", "/nope")
	if rec.Code != 404 || rec.Body.String() != `{"err":"not_found"}` {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	for _, p := range []string{"/v1/glance", "/v1/heatmap", "/healthz", "/nope"} {
		rec = do(t, g, "POST", p)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("POST %s: %d", p, rec.Code)
		}
	}
	if len(g.calls) != 0 {
		t.Fatal("Get called")
	}
}

type blockingGetter struct{}

func (blockingGetter) Get(ctx context.Context, _ service.Kind, _ city.Code) (cache.Entry, error) {
	<-ctx.Done()
	return cache.Entry{}, ctx.Err()
}

func TestRequestDeadline(t *testing.T) {
	h := New(blockingGetter{}, func() time.Time { return t0 },
		slog.New(slog.NewTextHandler(io.Discard, nil)), WithRequestTimeout(50*time.Millisecond))
	rec := httptest.NewRecorder()
	start := time.Now()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/glance?city=almaty", nil))
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("handler blocked %v", d)
	}
	if rec.Code != 502 || rec.Body.String() != `{"err":"upstream"}` {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}
