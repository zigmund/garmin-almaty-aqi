package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadDefaults(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "c.yaml", "aai:\n  key_file: /run/key\n")
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Listen != "127.0.0.1:8790" {
		t.Errorf("listen = %q", c.Listen)
	}
	if c.Log.Level != "info" || c.Log.Format != "json" || c.Log.Requests {
		t.Errorf("log = %+v", c.Log)
	}
	if c.AAI.BaseURL != "https://backend.air.org.kz/v1" || c.AAI.KeyFile != "/run/key" || c.AAI.Timeout != 8*time.Second {
		t.Errorf("aai = %+v", c.AAI)
	}
	if c.Cache.Backend != "memory" || c.Cache.TTL != 15*time.Minute ||
		c.Cache.StaleTTL != 2*time.Hour || c.Cache.LockWait != 5*time.Second {
		t.Errorf("cache = %+v", c.Cache)
	}
	if c.Redis.Addr != "127.0.0.1:6379" || c.Redis.PasswordFile != "" || c.Redis.DB != 0 || c.Redis.Prefix != "almatyaqi:" {
		t.Errorf("redis = %+v", c.Redis)
	}
	if c.Budget.PerHour != 80 {
		t.Errorf("budget = %+v", c.Budget)
	}
}

func TestLoadRejects(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{"backend", "aai: {key_file: /k}\ncache: {backend: disk}\n"},
		{"no key_file", "listen: 127.0.0.1:1\n"},
		{"per_hour zero", "aai: {key_file: /k}\nbudget: {per_hour: 0}\n"},
		{"per_hour 101", "aai: {key_file: /k}\nbudget: {per_hour: 101}\n"},
		{"stale < ttl", "aai: {key_file: /k}\ncache: {ttl: 10m, stale_ttl: 5m}\n"},
		{"http base_url", "aai: {key_file: /k, base_url: 'http://example.com'}\n"},
		{"unknown field", "aai: {key_file: /k}\nfoo: 1\n"},
		{"bad log level", "aai: {key_file: /k}\nlog: {level: loud}\n"},
		{"bad listen", "aai: {key_file: /k}\nlisten: nope\n"},
		{"redis addr no port", "aai: {key_file: /k}\ncache: {backend: redis}\nredis: {addr: 'localhost'}\n"},
		{"redis addr empty port", "aai: {key_file: /k}\ncache: {backend: redis}\nredis: {addr: 'localhost:'}\n"},
		{"redis addr empty", "aai: {key_file: /k}\ncache: {backend: redis}\nredis: {addr: ''}\n"},
		{"ttl 30s", "aai: {key_file: /k}\ncache: {ttl: 30s}\n"},
		{"lock_wait too long", "aai: {key_file: /k, timeout: 8s}\ncache: {lock_wait: 22s}\n"},
		{"zero timeout", "aai: {key_file: /k, timeout: 0s}\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := writeFile(t, t.TempDir(), "c.yaml", tc.yaml)
			if _, err := Load(p); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestLoadAcceptsBounds(t *testing.T) {
	for _, y := range []string{
		"aai: {key_file: /k}\ncache: {backend: redis}\nredis: {addr: 'redis.local:6379'}\n",
		"aai: {key_file: /k}\ncache: {ttl: 1m}\n",
		"aai: {key_file: /k, timeout: 8s}\ncache: {lock_wait: 21s}\n",
	} {
		if _, err := Load(writeFile(t, t.TempDir(), "c.yaml", y)); err != nil {
			t.Errorf("%q: %v", y, err)
		}
	}
}

func TestLoadAllowsLoopbackHTTP(t *testing.T) {
	p := writeFile(t, t.TempDir(), "c.yaml", "aai: {key_file: /k, base_url: 'http://127.0.0.1:9999/v1'}\n")
	if _, err := Load(p); err != nil {
		t.Fatal(err)
	}
}

func TestRelativeKeyFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "key", "abc\n")
	writeFile(t, dir, "pw", "secret\n")
	p := writeFile(t, dir, "c.yaml", "aai: {key_file: key}\nredis: {password_file: pw}\n")
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.AAI.KeyFile != filepath.Join(dir, "key") {
		t.Errorf("key_file = %q", c.AAI.KeyFile)
	}
	if k, err := c.ReadAAIKey(); err != nil || k != "abc" {
		t.Errorf("key = %q, %v", k, err)
	}
	if pw, err := c.ReadRedisPassword(); err != nil || pw != "secret" {
		t.Errorf("pw = %q, %v", pw, err)
	}
}

func TestReadAAIKey(t *testing.T) {
	dir := t.TempDir()
	c := &Config{}
	c.AAI.KeyFile = writeFile(t, dir, "k", " k3y\n")
	if k, err := c.ReadAAIKey(); err != nil || k != "k3y" {
		t.Fatalf("key = %q, %v", k, err)
	}
	c.AAI.KeyFile = writeFile(t, dir, "empty", " \n\n")
	if _, err := c.ReadAAIKey(); err == nil {
		t.Fatal("empty key must fail")
	}
	// A missing file must not leak anything but the path.
	c.AAI.KeyFile = filepath.Join(dir, "missing")
	if _, err := c.ReadAAIKey(); err == nil {
		t.Fatal("missing key must fail")
	}
}

func TestReadRedisPasswordUnset(t *testing.T) {
	c := &Config{}
	if pw, err := c.ReadRedisPassword(); err != nil || pw != "" {
		t.Fatalf("pw = %q, %v", pw, err)
	}
}

func TestUnknownFieldErrorHidesValue(t *testing.T) {
	p := writeFile(t, t.TempDir(), "c.yaml", "aai: {key_file: /k}\nsecret_thing: hunter2\n")
	_, err := Load(p)
	if err == nil || strings.Contains(err.Error(), "hunter2") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadLogRequests(t *testing.T) {
	p := writeFile(t, t.TempDir(), "c.yaml", "aai: {key_file: /k}\nlog: {requests: true}\n")
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Log.Requests {
		t.Errorf("log.requests = false, want true")
	}
}
