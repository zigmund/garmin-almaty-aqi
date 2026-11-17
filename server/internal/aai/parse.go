package aai

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// maxIQAirAge is the oldest IQAir measurement still considered current.
const maxIQAirAge = 3 * time.Hour

// Station is a single air-quality station reading (PM2.5 in ug/m3).
type Station struct{ Lat, Lon, PM float64 }

// Districts holds the per-district PM2.5 averages for Almaty.
type Districts struct {
	PMs  []float64
	Time time.Time
}

// MapSnapshot is the station list of /measurements/map.
type MapSnapshot struct {
	Stations  []Station
	WindowEnd time.Time
}

func num(v any) (float64, bool) {
	f, ok := v.(float64)
	return f, ok && !math.IsNaN(f) && !math.IsInf(f, 0)
}

func parseTime(v any) time.Time {
	s, _ := v.(string)
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func decode(b []byte) (map[string]any, error) {
	var root map[string]any
	if err := json.Unmarshal(b, &root); err != nil || root == nil {
		return nil, fmt.Errorf("%w: unparsable json", ErrUpstream)
	}
	return root, nil
}

func records(root map[string]any, key string) ([]map[string]any, error) {
	arr, ok := root[key].([]any)
	if !ok {
		return nil, fmt.Errorf("%w: unexpected shape", ErrUpstream)
	}
	out := make([]map[string]any, 0, len(arr))
	for _, e := range arr {
		if r, ok := e.(map[string]any); ok {
			out = append(out, r)
		}
	}
	return out, nil
}

// ParseDistricts parses /measurements/districts, keeping pm25_avg >= 0.
func ParseDistricts(b []byte) (Districts, error) {
	root, err := decode(b)
	if err != nil {
		return Districts{}, err
	}
	recs, err := records(root, "districts")
	if err != nil {
		return Districts{}, err
	}
	d := Districts{Time: parseTime(root["timestamp"])}
	for _, r := range recs {
		if pm, ok := num(r["pm25_avg"]); ok && pm >= 0 {
			d.PMs = append(d.PMs, pm)
		}
	}
	if len(d.PMs) == 0 {
		return Districts{}, fmt.Errorf("%w: no districts", ErrUpstream)
	}
	return d, nil
}

// ParseMap parses /measurements/map; the station list may be empty. Stations need a numeric location_id,
// coordinates and pm25_avg >= 0.
func ParseMap(b []byte) (MapSnapshot, error) {
	root, err := decode(b)
	if err != nil {
		return MapSnapshot{}, err
	}
	recs, err := records(root, "data")
	if err != nil {
		return MapSnapshot{}, err
	}
	var m MapSnapshot
	if meta, ok := root["meta"].(map[string]any); ok {
		m.WindowEnd = parseTime(meta["window_end"])
	}
	for _, r := range recs {
		_, idOK := num(r["location_id"])
		lat, latOK := num(r["latitude"])
		lon, lonOK := num(r["longitude"])
		pm, pmOK := num(r["pm25_avg"])
		if idOK && latOK && lonOK && pmOK && pm >= 0 {
			m.Stations = append(m.Stations, Station{Lat: lat, Lon: lon, PM: pm})
		}
	}
	// An empty station list is valid here: Karaganda can still have IQAir
	// stations. Callers (respond) reject an empty result where it is fatal.
	return m, nil
}

// ParseIQAir parses /iqair/current, dropping stations without a valid
// reading or whose ts is missing or older than 3h relative to now.
func ParseIQAir(b []byte, now time.Time) ([]Station, error) {
	root, err := decode(b)
	if err != nil {
		return nil, err
	}
	recs, err := records(root, "data")
	if err != nil {
		return nil, err
	}
	out := []Station{}
	for _, r := range recs {
		lat, latOK := num(r["latitude"])
		lon, lonOK := num(r["longitude"])
		pm, pmOK := num(r["pm25_concentration"])
		ts := parseTime(r["ts"])
		if !latOK || !lonOK || !pmOK || pm < 0 || ts.IsZero() || now.Sub(ts) > maxIQAirAge {
			continue
		}
		out = append(out, Station{Lat: lat, Lon: lon, PM: pm})
	}
	return out, nil
}
