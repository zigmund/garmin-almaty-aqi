// Command almatyaqi-server serves air quality data to Garmin watches.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/zigmund/garmin-almaty-aqi/server/internal/aai"
	"github.com/zigmund/garmin-almaty-aqi/server/internal/cache"
	"github.com/zigmund/garmin-almaty-aqi/server/internal/config"
	"github.com/zigmund/garmin-almaty-aqi/server/internal/httpapi"
	"github.com/zigmund/garmin-almaty-aqi/server/internal/service"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

const (
	writeTimeout = 15 * time.Second
	// requestTimeout must be below writeTimeout so a slow fill ends in a
	// response (stale entry or 502) instead of a closed connection.
	requestTimeout = httpapi.DefaultRequestTimeout
)

// Compile-time check: requestTimeout < writeTimeout.
const _ = uint64(writeTimeout - requestTimeout - 1)

// store is what both cache backends provide.
type store interface {
	cache.Store
	cache.Budget
}

func main() {
	cfgPath := flag.String("config", "/etc/almatyaqi-server/config.yaml", "path to the YAML config")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}
	if err := run(*cfgPath); err != nil {
		fmt.Fprintln(os.Stderr, "almatyaqi-server:", err)
		os.Exit(1)
	}
}

func newLogger(c config.Log) *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(c.Level)); err != nil {
		lvl = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: lvl, ReplaceAttr: durationString}
	if strings.EqualFold(c.Format, "text") {
		return slog.New(slog.NewTextHandler(os.Stderr, opts))
	}
	return slog.New(slog.NewJSONHandler(os.Stderr, opts))
}

// durationString logs time.Duration values as "15m0s" / "511ms": the JSON handler would
// otherwise write them as integer nanoseconds.
func durationString(_ []string, a slog.Attr) slog.Attr {
	if a.Value.Kind() == slog.KindDuration {
		a.Value = slog.StringValue(a.Value.Duration().String())
	}
	return a
}

func run(cfgPath string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	log := newLogger(cfg.Log)
	slog.SetDefault(log)
	key, err := cfg.ReadAAIKey()
	if err != nil {
		return err
	}
	now := time.Now

	var st store
	switch cfg.Cache.Backend {
	case "redis":
		pw, err := cfg.ReadRedisPassword()
		if err != nil {
			return err
		}
		// No ping: the server starts even while Redis is down.
		rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr, Password: pw, DB: cfg.Redis.DB})
		defer rdb.Close()
		st = cache.NewRedis(rdb, cfg.Redis.Prefix, cfg.Budget.PerHour, now)
	case "memory":
		st = cache.NewMemory(cfg.Budget.PerHour, now)
	default:
		return fmt.Errorf("cache.backend: unknown backend %q", cfg.Cache.Backend)
	}

	// A redirect is returned as is (counts as non-200): no extra unbudgeted
	// request, and the key is not forwarded. Timeouts are aai's own.
	hc := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	up := aai.New(cfg.AAI.BaseURL, key, cfg.AAI.Timeout, hc, st, now, log)
	svc := service.New(st, up, service.Options{
		TTL:      cfg.Cache.TTL,
		StaleTTL: cfg.Cache.StaleTTL,
		LockWait: cfg.Cache.LockWait,
		LockTTL:  config.LockTTL(cfg.AAI.Timeout),
		Now:      now,
		Log:      log,
	})

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           httpapi.New(svc, now, log, httpapi.WithRequestTimeout(requestTimeout)),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       60 * time.Second,
	}
	log.Info("starting", "version", version, "listen", cfg.Listen,
		"backend", cfg.Cache.Backend, "ttl", cfg.Cache.TTL, "stale_ttl", cfg.Cache.StaleTTL,
		"lock_wait", cfg.Cache.LockWait, "budget.per_hour", cfg.Budget.PerHour,
		"aai.timeout", cfg.AAI.Timeout)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	log.Info("shutting down")
	sctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := srv.Shutdown(sctx); err != nil {
		return err
	}
	if err := <-errc; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
