package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
	"time"
)

func TestDurationString(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{ReplaceAttr: durationString}))
	log.Info("x", "duration", 511*time.Millisecond, "status", 200,
		slog.Group("aai", "timeout", 30*time.Second))

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", buf.String(), err)
	}
	if got["duration"] != "511ms" {
		t.Errorf("duration = %#v, want \"511ms\"", got["duration"])
	}
	if got["status"] != float64(200) {
		t.Errorf("status = %#v, want 200", got["status"])
	}
	if g, _ := got["aai"].(map[string]any); g == nil || g["timeout"] != "30s" {
		t.Errorf("aai = %#v, want timeout \"30s\"", got["aai"])
	}
}
