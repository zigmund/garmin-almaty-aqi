package respond

import (
	"encoding/base64"
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/zigmund/garmin-airmon/server/internal/aai"
	"github.com/zigmund/garmin-airmon/server/internal/city"
)

var (
	t0  = time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC)
	now = t0.Add(5 * time.Minute)
)

func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestGlanceAlmatyMean(t *testing.T) {
	d := aai.Districts{PMs: []float64{51.1, 49.6, 48.1, 41.0, 36.6, 35.2, 27.4, 18.4}, Time: t0}
	e, err := GlanceAlmaty(d, now)
	if err != nil {
		t.Fatal(err)
	}
	m := decode(t, e.Body)
	if m["pm"] != 38.4 || m["aqi"] != float64(108) || m["c"] != "almaty" || m["m"] != "city" {
		t.Fatalf("body: %s", e.Body)
	}
	if _, ok := m["a"]; ok {
		t.Fatal("unexpected a")
	}
	if m["t"] != "2026-11-17T10:00:00Z" {
		t.Fatalf("t: %v", m["t"])
	}
	if !e.T.Equal(t0) || !e.Fetched.Equal(now) {
		t.Fatalf("entry times: %v %v", e.T, e.Fetched)
	}
	want := `{"m":"city","c":"almaty","aqi":108,"pm":38.4,"t":"2026-11-17T10:00:00Z"}`
	if string(e.Body) != want {
		t.Fatalf("got %s", e.Body)
	}
}

func TestGlanceStationsMean(t *testing.T) {
	st := []aai.Station{{PM: 30}, {PM: 50}, {PM: 100}}
	e, err := GlanceStations(city.Karaganda, st, t0, now)
	if err != nil {
		t.Fatal(err)
	}
	m := decode(t, e.Body)
	if m["pm"] != float64(60) || m["aqi"] != float64(154) || m["c"] != "karaganda" {
		t.Fatalf("body: %s", e.Body)
	}
}

func grid(t *testing.T, m map[string]any) []byte {
	t.Helper()
	g, err := base64.StdEncoding.DecodeString(m["g"].(string))
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestHeatmapCenterStation(t *testing.T) {
	b := city.Almaty.BBox()
	lat, lon := (b[1]+b[3])/2, (b[0]+b[2])/2
	e, err := Heatmap(city.Almaty, []aai.Station{{Lat: lat, Lon: lon, PM: 60}}, t0, now)
	if err != nil {
		t.Fatal(err)
	}
	m := decode(t, e.Body)
	g := grid(t, m)
	if len(g) != 576 {
		t.Fatalf("len %d", len(g))
	}
	if g[11*24+11] != 3 {
		t.Fatalf("center %d", g[11*24+11])
	}
	for _, i := range []int{0, 23, 552, 575} {
		if g[i] != 255 {
			t.Fatalf("corner %d = %d", i, g[i])
		}
	}
	wantS := []any{[]any{math.Round(lat*1e4) / 1e4, math.Round(lon*1e4) / 1e4, float64(3)}}
	gotS := m["s"].([]any)
	if len(gotS) != 1 || gotS[0].([]any)[2] != wantS[0].([]any)[2] ||
		gotS[0].([]any)[0] != wantS[0].([]any)[0] || gotS[0].([]any)[1] != wantS[0].([]any)[1] {
		t.Fatalf("s: %v", gotS)
	}
	bb := m["bbox"].([]any)
	for i := range bb {
		if bb[i] != b[i] {
			t.Fatalf("bbox %v", bb)
		}
	}
	if m["w"] != float64(24) || m["h"] != float64(24) || m["m"] != "city" || m["c"] != "almaty" {
		t.Fatalf("body %s", e.Body)
	}
	if _, ok := m["a"]; ok {
		t.Fatal("a")
	}
	if _, ok := m["me"]; ok {
		t.Fatal("me")
	}
}

func TestHeatmapNWCell(t *testing.T) {
	b := city.Almaty.BBox()
	lat := b[3] - 0.5*(b[3]-b[1])/24
	lon := b[0] + 0.5*(b[2]-b[0])/24
	e, err := Heatmap(city.Almaty, []aai.Station{{Lat: lat, Lon: lon, PM: 5}}, t0, now)
	if err != nil {
		t.Fatal(err)
	}
	if g := grid(t, decode(t, e.Body)); g[0] != 0 {
		t.Fatalf("g[0]=%d", g[0])
	}
}

func TestEmptyInputs(t *testing.T) {
	if _, err := GlanceAlmaty(aai.Districts{Time: t0}, now); err == nil {
		t.Error("almaty empty")
	}
	if _, err := GlanceStations(city.Karaganda, nil, t0, now); err == nil {
		t.Error("stations empty")
	}
	if _, err := Heatmap(city.Almaty, nil, t0, now); err == nil {
		t.Error("heatmap empty")
	}
}

func TestZeroTime(t *testing.T) {
	st := []aai.Station{{Lat: 43.3, Lon: 76.9, PM: 10}}
	if _, err := GlanceAlmaty(aai.Districts{PMs: []float64{10}}, now); err == nil {
		t.Error("almaty zero time")
	}
	if _, err := GlanceStations(city.Almaty, st, time.Time{}, now); err == nil {
		t.Error("stations zero time")
	}
	if _, err := Heatmap(city.Almaty, st, time.Time{}, now); err == nil {
		t.Error("heatmap zero time")
	}
}
