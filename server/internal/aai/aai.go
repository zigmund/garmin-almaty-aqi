// Package aai is the client for the Almaty Air Initiative upstream API.
package aai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/zigmund/garmin-airmon/server/internal/cache"
	"github.com/zigmund/garmin-airmon/server/internal/city"
)

// ErrUpstream marks any failure caused by the upstream API or its data.
var ErrUpstream = errors.New("upstream")

const maxBody = 8 << 20

// Budget limits upstream request rate; Take blocks or fails before each request.
type Budget interface {
	Take(ctx context.Context) error
}

// Client talks to the AAI API. The key is only ever sent as a header.
type Client struct {
	base    string
	key     string
	timeout time.Duration
	hc      *http.Client
	budget  Budget
	now     func() time.Time
	log     *slog.Logger
}

// New creates a Client. A nil hc means a default http.Client, a nil log
// means slog.Default().
func New(baseURL, key string, timeout time.Duration, hc *http.Client, budget Budget, now func() time.Time, log *slog.Logger) *Client {
	if log == nil {
		log = slog.Default()
	}
	if hc == nil {
		hc = &http.Client{}
	}
	return &Client{base: strings.TrimRight(baseURL, "/"), key: key, timeout: timeout, hc: hc, budget: budget, now: now, log: log}
}

// Districts fetches the Almaty district averages.
func (c *Client) Districts(ctx context.Context) (Districts, error) {
	b, err := c.get(ctx, "/measurements/districts", "lang=ru")
	if err != nil {
		return Districts{}, err
	}
	return ParseDistricts(b)
}

// Map fetches the AirGradient station map for a city.
func (c *Client) Map(ctx context.Context, cc city.Code) (MapSnapshot, error) {
	b, err := c.get(ctx, "/measurements/map", "city="+url.QueryEscape(string(cc)))
	if err != nil {
		return MapSnapshot{}, err
	}
	return ParseMap(b)
}

// IQAir fetches current IQAir stations for a city; an empty result is not an error.
func (c *Client) IQAir(ctx context.Context, cc city.Code) ([]Station, error) {
	b, err := c.get(ctx, "/iqair/current", "city="+url.QueryEscape(string(cc)))
	if err != nil {
		return nil, err
	}
	return ParseIQAir(b, c.now())
}

func (c *Client) get(ctx context.Context, path, query string) ([]byte, error) {
	if err := c.budget.Take(ctx); err != nil {
		if errors.Is(err, cache.ErrBudget) {
			c.log.Warn("aai budget denied", "path", path)
		}
		return nil, fmt.Errorf("aai budget: %w", err)
	}
	start := time.Now()
	status, outcome := 0, "ok"
	// One Info line per upstream request. Only the path is logged: never the
	// query, the URL, headers or the key.
	defer func() {
		c.log.Info("aai request", "path", path, "status", status,
			"duration", time.Since(start).Round(time.Millisecond), "outcome", outcome)
	}()
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path+"?"+query, nil)
	if err != nil {
		outcome = "bad_request"
		return nil, fmt.Errorf("%w: bad request", ErrUpstream)
	}
	req.Header.Set("x-api-key", c.key)
	resp, err := c.hc.Do(req)
	if err != nil {
		// Never log or wrap err itself: url.Error carries the full URL.
		kind := fmt.Sprintf("%T", errors.Unwrap(err))
		if ctx.Err() != nil {
			kind = "timeout_or_canceled"
		}
		outcome = "error:" + kind
		return nil, fmt.Errorf("%w: request failed (%s)", ErrUpstream, kind)
	}
	defer resp.Body.Close()
	status = resp.StatusCode
	if resp.StatusCode != http.StatusOK {
		outcome = "bad_status"
		return nil, fmt.Errorf("%w: status %d", ErrUpstream, resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		outcome = "error:read"
		return nil, fmt.Errorf("%w: read failed", ErrUpstream)
	}
	return b, nil
}
