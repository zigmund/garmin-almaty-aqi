package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func logOne(t *testing.T, h http.Handler, target string) map[string]any {
	t.Helper()
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	tick := time.Unix(0, 0)
	now := func() time.Time { tick = tick.Add(5 * time.Millisecond); return tick }
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("User-Agent", "test-agent")
	LogRequests(h, log, now).ServeHTTP(httptest.NewRecorder(), req)
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("log line %q: %v", buf.String(), err)
	}
	return m
}

func TestLogRequests(t *testing.T) {
	m := logOne(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte("hello"))
	}), "/v1/glance?city=almaty&lat=43.239&lon=76.890")
	want := map[string]any{
		"msg": "request in", "dir": "in", "method": "GET", "path": "/v1/glance",
		"query": "city=almaty&lat=43.239&lon=76.890", "status": 418.0, "bytes": 5.0,
		"user_agent": "test-agent",
	}
	for k, v := range want {
		if m[k] != v {
			t.Errorf("%s = %v, want %v", k, m[k], v)
		}
	}
	if m["duration"] == nil {
		t.Error("no duration")
	}
}

func TestLogRequestsDefaultStatus(t *testing.T) {
	m := logOne(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("{}"))
	}), "/healthz")
	if m["status"] != 200.0 || m["bytes"] != 2.0 || m["query"] != "" {
		t.Errorf("got %v", m)
	}
	m = logOne(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), "/x")
	if m["status"] != 200.0 || m["bytes"] != 0.0 {
		t.Errorf("empty handler: got %v", m)
	}
}
