// Package respond builds the ready JSON responses that the cache stores.
package respond

import (
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/zigmund/garmin-almaty-aqi/server/internal/aai"
	"github.com/zigmund/garmin-almaty-aqi/server/internal/aqi"
	"github.com/zigmund/garmin-almaty-aqi/server/internal/cache"
	"github.com/zigmund/garmin-almaty-aqi/server/internal/city"
)

var (
	errEmpty    = errors.New("respond: no data")
	errZeroTime = errors.New("respond: zero data time")
)

type glanceBody struct {
	M   string  `json:"m"`
	C   string  `json:"c"`
	AQI int     `json:"aqi"`
	PM  float64 `json:"pm"`
	T   string  `json:"t"`
}

// GlanceAlmaty builds the glance response from district averages.
func GlanceAlmaty(d aai.Districts, now time.Time) (cache.Entry, error) {
	return glance(city.Almaty, d.PMs, d.Time, now)
}

// GlanceStations builds the glance response from the mean of station PM values.
func GlanceStations(cc city.Code, st []aai.Station, t, now time.Time) (cache.Entry, error) {
	pms := make([]float64, len(st))
	for i, s := range st {
		pms[i] = s.PM
	}
	return glance(cc, pms, t, now)
}

func glance(cc city.Code, pms []float64, t, now time.Time) (cache.Entry, error) {
	if len(pms) == 0 {
		return cache.Entry{}, errEmpty
	}
	if t.IsZero() {
		return cache.Entry{}, errZeroTime
	}
	var sum float64
	for _, v := range pms {
		sum += v
	}
	pm := sum / float64(len(pms))
	return entry(glanceBody{
		M:   "city",
		C:   string(cc),
		AQI: aqi.FromPM(pm),
		PM:  math.Round(pm*10) / 10,
		T:   t.UTC().Format(time.RFC3339),
	}, t, now)
}

func entry(body any, t, now time.Time) (cache.Entry, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return cache.Entry{}, err
	}
	return cache.Entry{Body: b, T: t, Fetched: now}, nil
}
