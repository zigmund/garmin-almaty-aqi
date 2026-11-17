// Package httpapi serves the watch-facing HTTP API.
package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/zigmund/garmin-airmon/server/internal/cache"
	"github.com/zigmund/garmin-airmon/server/internal/city"
	"github.com/zigmund/garmin-airmon/server/internal/service"
)

// Getter returns the ready response for a kind and city (service.Service).
type Getter interface {
	Get(ctx context.Context, k service.Kind, cc city.Code) (cache.Entry, error)
}

const (
	bodyBadRequest = `{"err":"bad_request"}`
	bodyUpstream   = `{"err":"upstream"}`
	bodyInternal   = `{"err":"internal"}`
	bodyNotFound   = `{"err":"not_found"}`
	bodyOut        = `{"m":"out"}`
	bodyNoPos      = `{"m":"nopos"}`
)

// DefaultRequestTimeout bounds Get for one request. It must stay below the
// server WriteTimeout, which closes the connection without any response.
const DefaultRequestTimeout = 12 * time.Second

type handler struct {
	g       Getter
	now     func() time.Time
	log     *slog.Logger
	timeout time.Duration
}

// Option configures the handler.
type Option func(*handler)

// WithRequestTimeout sets the per-request deadline passed to Getter.Get.
func WithRequestTimeout(d time.Duration) Option {
	return func(h *handler) { h.timeout = d }
}

// New returns the API handler. now and log may be nil.
func New(g Getter, now func() time.Time, log *slog.Logger, opts ...Option) http.Handler {
	if now == nil {
		now = time.Now
	}
	if log == nil {
		log = slog.Default()
	}
	h := &handler{g: g, now: now, log: log, timeout: DefaultRequestTimeout}
	for _, o := range opts {
		o(h)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/glance", h.data(service.Glance))
	mux.HandleFunc("/v1/heatmap", h.data(service.Heatmap))
	mux.HandleFunc("/healthz", h.healthz)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			notAllowed(w)
			return
		}
		write(w, http.StatusNotFound, "no-store", bodyNotFound)
	})
	return mux
}

func notAllowed(w http.ResponseWriter) {
	w.Header().Set("Allow", http.MethodGet)
	write(w, http.StatusMethodNotAllowed, "no-store", `{"err":"method_not_allowed"}`)
}

func write(w http.ResponseWriter, code int, cacheControl, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", cacheControl)
	w.WriteHeader(code)
	_, _ = w.Write([]byte(body))
}

func (h *handler) healthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		notAllowed(w)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte("ok"))
}

// sel is the result of request parsing: a city to fetch, or a fixed body.
type sel struct {
	city  city.Code
	fixed string // non-empty: respond with this body without touching the store
}

func (h *handler) data(k service.Kind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			notAllowed(w)
			return
		}
		s, ok := parse(r.URL.Query())
		if !ok {
			write(w, http.StatusBadRequest, "no-store", bodyBadRequest)
			return
		}
		if s.fixed != "" {
			write(w, http.StatusOK, "no-store", s.fixed)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
		defer cancel()
		e, err := h.g.Get(ctx, k, s.city)
		switch {
		case err == nil:
		case errors.Is(err, service.ErrUnavailable),
			errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
			write(w, http.StatusBadGateway, "no-store", bodyUpstream)
			return
		default:
			if !errors.Is(err, service.ErrInternal) {
				h.log.Error("httpapi: unexpected error", "err", err)
			}
			write(w, http.StatusInternalServerError, "no-store", bodyInternal)
			return
		}
		cc := "no-store"
		if k == service.Heatmap {
			cc = "public, max-age=60"
		}
		write(w, http.StatusOK, cc, string(WithAge(e.Body, e.T, h.now())))
	}
}

// parse applies the mode selection: a position wins over the city parameter.
func parse(q url.Values) (sel, bool) {
	cityParam := q.Get("city")
	hasLat, hasLon := q.Has("lat"), q.Has("lon")
	if hasLat != hasLon {
		return sel{}, false
	}
	if hasLat {
		lat, ok1 := coord(q.Get("lat"), 90)
		lon, ok2 := coord(q.Get("lon"), 180)
		if !ok1 || !ok2 {
			return sel{}, false
		}
		if cc, ok := city.Locate(lat, lon); ok {
			return sel{city: cc}, true
		}
		return sel{fixed: bodyOut}, true
	}
	// The city parameter is validated only when no position is sent.
	switch cityParam {
	case "":
		return sel{fixed: bodyNoPos}, true
	case "out":
		return sel{fixed: bodyOut}, true
	}
	cc, ok := city.Parse(cityParam)
	if !ok {
		return sel{}, false
	}
	return sel{city: cc}, true
}

func coord(s string, limit float64) (float64, bool) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > limit {
		return 0, false
	}
	return v, true
}

// WithAge returns a copy of the JSON object body with "a" (seconds between t
// and now, at least 0) appended as its last field. body is never modified:
// cache entries share their backing array.
func WithAge(body []byte, t, now time.Time) []byte {
	age := int64(now.Sub(t) / time.Second)
	if age < 0 {
		age = 0
	}
	n := len(body)
	for n > 0 && body[n-1] != '}' {
		n--
	}
	if n == 0 {
		return append([]byte(nil), body...)
	}
	out := make([]byte, 0, n+24)
	out = append(out, body[:n-1]...)
	if hasField(out) {
		out = append(out, ',')
	}
	out = append(out, `"a":`...)
	out = strconv.AppendInt(out, age, 10)
	return append(out, '}')
}

// hasField reports whether the open object prefix already holds a field.
func hasField(prefix []byte) bool {
	for i := len(prefix) - 1; i >= 0; i-- {
		switch prefix[i] {
		case ' ', '\t', '\n', '\r':
		case '{':
			return false
		default:
			return true
		}
	}
	return false
}
