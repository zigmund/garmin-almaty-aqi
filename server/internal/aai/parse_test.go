package aai

import (
	"errors"
	"os"
	"testing"
	"time"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseDistricts(t *testing.T) {
	d, err := ParseDistricts(fixture(t, "districts.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := []float64{51.1, 49.6, 48.1, 41.0, 36.6, 35.2, 27.4, 18.4}
	if len(d.PMs) != len(want) {
		t.Fatalf("PMs = %v", d.PMs)
	}
	for i, v := range want {
		if d.PMs[i] != v {
			t.Errorf("PMs[%d] = %v, want %v", i, d.PMs[i], v)
		}
	}
	if !d.Time.Equal(time.Date(2026, 11, 17, 4, 45, 0, 0, time.UTC)) {
		t.Errorf("Time = %v", d.Time)
	}
	for _, bad := range []string{`{"districts":[]}`, `garbage`, `{"districts":[{"pm25_avg":-1},{"pm25_avg":null}]}`} {
		if _, err := ParseDistricts([]byte(bad)); !errors.Is(err, ErrUpstream) {
			t.Errorf("%q: err = %v", bad, err)
		}
	}
}

func TestParseMap(t *testing.T) {
	m, err := ParseMap(fixture(t, "map.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Stations) != 2 {
		t.Fatalf("stations = %v", m.Stations)
	}
	if m.Stations[0] != (Station{Lat: 43.2937, Lon: 76.9154, PM: 49.7}) {
		t.Errorf("station 0 = %v", m.Stations[0])
	}
	if !m.WindowEnd.Equal(time.Date(2026, 11, 17, 4, 15, 0, 0, time.UTC)) {
		t.Errorf("WindowEnd = %v", m.WindowEnd)
	}
	for _, bad := range []string{`garbage`, `{}`} {
		if _, err := ParseMap([]byte(bad)); !errors.Is(err, ErrUpstream) {
			t.Errorf("%q: err = %v", bad, err)
		}
	}
}

func TestParseMapEmptyStations(t *testing.T) {
	for _, in := range []string{
		`{"data":[],"meta":{"window_end":"2026-11-17T04:15:00Z"}}`,
		`{"data":[{"location_id":1,"pm25_avg":-1,"latitude":1,"longitude":1}],"meta":{"window_end":"2026-11-17T04:15:00Z"}}`,
	} {
		m, err := ParseMap([]byte(in))
		if err != nil || len(m.Stations) != 0 || !m.WindowEnd.Equal(time.Date(2026, 11, 17, 4, 15, 0, 0, time.UTC)) {
			t.Errorf("%q: m=%+v err=%v", in, m, err)
		}
	}
}

func TestParseIQAir(t *testing.T) {
	now := time.Date(2026, 11, 17, 12, 1, 0, 0, time.UTC) // "a" is 3h01m old, "b" 3h02m
	got, err := ParseIQAir(fixture(t, "iqair.json"), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got %v, want none (all stale/invalid)", got)
	}
	now = time.Date(2026, 11, 17, 11, 59, 30, 0, time.UTC) // "a" 2h59m30s, "b" 3h00m30s: only a stays
	got, err = ParseIQAir(fixture(t, "iqair.json"), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != (Station{Lat: 49.9607, Lon: 73.1994, PM: 114.0}) {
		t.Fatalf("got %v", got)
	}
	if _, err := ParseIQAir([]byte("garbage"), now); !errors.Is(err, ErrUpstream) {
		t.Errorf("garbage: err = %v", err)
	}
}
