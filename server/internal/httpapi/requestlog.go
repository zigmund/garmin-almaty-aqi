package httpapi

import (
	"log/slog"
	"net/http"
	"time"
)

// LogRequests wraps h to log one "request in" line (dir=in) per request after it completes. The query
// string is logged as is, so it carries the watch position: enable it only to debug.
func LogRequests(h http.Handler, log *slog.Logger, now func() time.Time) http.Handler {
	if log == nil {
		log = slog.Default()
	}
	if now == nil {
		now = time.Now
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := now()
		rw := &statusWriter{ResponseWriter: w}
		h.ServeHTTP(rw, r)
		if rw.status == 0 {
			rw.status = http.StatusOK
		}
		log.Info("request in", "dir", "in", "method", r.Method, "path", r.URL.Path, "query", r.URL.RawQuery,
			"status", rw.status, "bytes", rw.bytes, "duration", now().Sub(start),
			"user_agent", r.UserAgent())
	})
}

// statusWriter records the status code and the number of body bytes written.
type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
