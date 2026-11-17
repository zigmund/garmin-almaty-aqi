// Package config loads, defaults and validates the airmon-server YAML config.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Defaults applied when a field is omitted.
const (
	DefaultListen      = "127.0.0.1:8790"
	DefaultLogLevel    = "info"
	DefaultLogFormat   = "json"
	DefaultAAIBaseURL  = "https://backend.air.org.kz/v1"
	DefaultAAITimeout  = 8 * time.Second
	DefaultCacheTTL    = 15 * time.Minute
	DefaultStaleTTL    = 2 * time.Hour
	DefaultLockWait    = 5 * time.Second
	DefaultRedisAddr   = "127.0.0.1:6379"
	DefaultRedisPrefix = "airmon:"
	DefaultPerHour     = 80

	// MaxPerHour is the largest accepted budget.per_hour: a hard ceiling on
	// upstream requests, kept well below the provider's own limit.
	MaxPerHour = 100
	// MaxAAITimeout is the largest accepted aai.timeout.
	MaxAAITimeout = 2 * time.Minute
	// MinCacheTTL is the smallest accepted cache.ttl: a shorter one would
	// let clients drive the upstream request rate.
	MinCacheTTL = time.Minute
	// LockTTLSlack is added to 2*aai.timeout to get the fill lock TTL.
	LockTTLSlack = 5 * time.Second
)

// LockTTL is the fill lock TTL derived from the upstream timeout.
func LockTTL(aaiTimeout time.Duration) time.Duration { return 2*aaiTimeout + LockTTLSlack }

// MaxLockWait is the largest accepted cache.lock_wait for an upstream timeout.
func MaxLockWait(aaiTimeout time.Duration) time.Duration { return LockTTL(aaiTimeout) }

// Cache backends.
const (
	BackendMemory = "memory"
	BackendRedis  = "redis"
)

var quotedRe = regexp.MustCompile("`[^`]*`")

// Config is the root configuration.
type Config struct {
	Listen string `yaml:"listen"`
	Log    Log    `yaml:"log"`
	AAI    AAI    `yaml:"aai"`
	Cache  Cache  `yaml:"cache"`
	Redis  Redis  `yaml:"redis"`
	Budget Budget `yaml:"budget"`
}

// Log configures logging.
type Log struct {
	Level  string `yaml:"level"`  // debug|info|warn|error
	Format string `yaml:"format"` // json|text
}

// AAI configures the upstream Almaty Air Initiative API client.
type AAI struct {
	BaseURL string `yaml:"base_url"`
	// KeyFile holds the API key; the key itself is never part of the config.
	KeyFile string        `yaml:"key_file"`
	Timeout time.Duration `yaml:"timeout"`
}

// Cache configures the response cache.
type Cache struct {
	Backend  string        `yaml:"backend"`   // memory|redis
	TTL      time.Duration `yaml:"ttl"`       // fresh lifetime
	StaleTTL time.Duration `yaml:"stale_ttl"` // total lifetime, stale data is served within it
	LockWait time.Duration `yaml:"lock_wait"` // how long to wait for another refresher
}

// Redis configures the redis cache backend.
type Redis struct {
	Addr         string `yaml:"addr"`
	PasswordFile string `yaml:"password_file"` // optional
	DB           int    `yaml:"db"`
	Prefix       string `yaml:"prefix"`
}

// Budget caps upstream requests.
type Budget struct {
	PerHour int `yaml:"per_hour"` // sliding 1h window
}

// defaults returns a Config with every default filled in.
func defaults() Config {
	return Config{
		Listen: DefaultListen,
		Log:    Log{Level: DefaultLogLevel, Format: DefaultLogFormat},
		AAI:    AAI{BaseURL: DefaultAAIBaseURL, Timeout: DefaultAAITimeout},
		Cache: Cache{
			Backend:  BackendMemory,
			TTL:      DefaultCacheTTL,
			StaleTTL: DefaultStaleTTL,
			LockWait: DefaultLockWait,
		},
		Redis:  Redis{Addr: DefaultRedisAddr, Prefix: DefaultRedisPrefix},
		Budget: Budget{PerHour: DefaultPerHour},
	}
}

// Load reads, decodes, defaults and validates the config file at p. Relative
// file paths are resolved against the file's directory.
func Load(p string) (*Config, error) {
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	cfg, err := Parse(data, filepath.Dir(p))
	if err != nil {
		return nil, fmt.Errorf("config %s: %w", p, err)
	}
	return cfg, nil
}

// Parse decodes config data strictly over the defaults and validates it.
// baseDir resolves relative file paths.
func Parse(data []byte, baseDir string) (*Config, error) {
	cfg := defaults()
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	err := dec.Decode(&cfg)
	if err == nil {
		var extra yaml.Node
		if err2 := dec.Decode(&extra); !errors.Is(err2, io.EOF) {
			if err2 != nil {
				return nil, decodeError(err2)
			}
			return nil, errors.New("decoding yaml: multiple documents are not supported")
		}
	} else if !errors.Is(err, io.EOF) {
		return nil, decodeError(err)
	}
	cfg.normalize()
	cfg.resolvePaths(baseDir)
	if errs := cfg.validate(); len(errs) > 0 {
		return nil, fmt.Errorf("invalid config: %w", errors.Join(errs...))
	}
	return &cfg, nil
}

// decodeError strips backtick-quoted values: yaml echoes offending scalars.
func decodeError(err error) error {
	return fmt.Errorf("decoding yaml: %s", quotedRe.ReplaceAllString(err.Error(), "`<redacted>`"))
}

func (c *Config) normalize() {
	c.Log.Level = strings.ToLower(strings.TrimSpace(c.Log.Level))
	c.Log.Format = strings.ToLower(strings.TrimSpace(c.Log.Format))
	c.Cache.Backend = strings.ToLower(strings.TrimSpace(c.Cache.Backend))
	c.AAI.BaseURL = strings.TrimSpace(c.AAI.BaseURL)
}

func (c *Config) resolvePaths(baseDir string) {
	for _, p := range []*string{&c.AAI.KeyFile, &c.Redis.PasswordFile} {
		if *p != "" && !filepath.IsAbs(*p) {
			*p = filepath.Join(baseDir, *p)
		}
	}
}

func (c *Config) validate() []error {
	var errs []error
	add := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }

	if _, port, err := net.SplitHostPort(c.Listen); err != nil {
		add("listen: %q must be host:port", c.Listen)
	} else if _, err := strconv.ParseUint(port, 10, 16); err != nil {
		add("listen: port %q must be a number in 0..65535", port)
	}

	switch c.Log.Level {
	case "debug", "info", "warn", "error":
	default:
		add("log.level: %q must be one of debug|info|warn|error", c.Log.Level)
	}
	switch c.Log.Format {
	case "json", "text":
	default:
		add("log.format: %q must be one of json|text", c.Log.Format)
	}

	if err := checkBaseURL(c.AAI.BaseURL); err != nil {
		add("aai.base_url: %v", err)
	}
	if c.AAI.KeyFile == "" {
		add("aai.key_file: required")
	}
	if c.AAI.Timeout <= 0 || c.AAI.Timeout > MaxAAITimeout {
		add("aai.timeout: must be a positive duration of at most %s", MaxAAITimeout)
	}

	switch c.Cache.Backend {
	case BackendMemory:
	case BackendRedis:
		if _, port, err := net.SplitHostPort(c.Redis.Addr); err != nil || port == "" {
			add("redis.addr: must be host:port when cache.backend is redis")
		}
	default:
		add("cache.backend: %q must be one of memory|redis", c.Cache.Backend)
	}
	if c.Cache.TTL < MinCacheTTL {
		add("cache.ttl: must be at least %s", MinCacheTTL)
	}
	if c.Cache.StaleTTL < c.Cache.TTL {
		add("cache.stale_ttl: must not be less than cache.ttl")
	}
	if c.Cache.LockWait <= 0 {
		add("cache.lock_wait: must be a positive duration")
	} else if max := MaxLockWait(c.AAI.Timeout); c.AAI.Timeout > 0 && c.Cache.LockWait > max {
		add("cache.lock_wait: must not exceed %s (2*aai.timeout + %s, the fill lock TTL)", max, LockTTLSlack)
	}
	if c.Redis.DB < 0 {
		add("redis.db: must not be negative")
	}

	if c.Budget.PerHour < 1 || c.Budget.PerHour > MaxPerHour {
		add("budget.per_hour: must be in 1..%d", MaxPerHour)
	}
	return errs
}

// checkBaseURL requires an absolute https URL; plain http only for loopback,
// because the API key is sent there. Errors never include the value.
func checkBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("is not a valid URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("must be an absolute http(s) URL")
	}
	if u.Host == "" {
		return errors.New("has no host")
	}
	if u.User != nil {
		return errors.New("must not contain userinfo")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return errors.New("must not contain a query or fragment")
	}
	if u.Scheme == "http" && !isLoopbackHost(u.Hostname()) {
		return errors.New("must use https (http is allowed only for loopback hosts: the API key is sent there)")
	}
	return nil
}

func isLoopbackHost(h string) bool {
	if strings.EqualFold(strings.TrimSuffix(h, "."), "localhost") {
		return true
	}
	a, err := netip.ParseAddr(h)
	return err == nil && a.Unmap().IsLoopback()
}

// ReadAAIKey returns the AAI API key from aai.key_file, whitespace trimmed.
// Errors never contain the key.
func (c *Config) ReadAAIKey() (string, error) {
	raw, err := os.ReadFile(c.AAI.KeyFile)
	if err != nil {
		return "", fmt.Errorf("aai.key_file: reading key file: %w", err)
	}
	key := strings.TrimSpace(string(raw))
	if key == "" {
		return "", errors.New("aai.key_file: key file is empty")
	}
	return key, nil
}

// ReadRedisPassword returns the redis password from redis.password_file,
// whitespace trimmed, or "" when no file is configured.
func (c *Config) ReadRedisPassword() (string, error) {
	if c.Redis.PasswordFile == "" {
		return "", nil
	}
	raw, err := os.ReadFile(c.Redis.PasswordFile)
	if err != nil {
		return "", fmt.Errorf("redis.password_file: reading password file: %w", err)
	}
	return strings.TrimSpace(string(raw)), nil
}
