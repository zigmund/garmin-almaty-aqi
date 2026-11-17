package respond

import (
	"encoding/base64"
	"math"
	"time"

	"github.com/zigmund/garmin-almaty-aqi/server/internal/aai"
	"github.com/zigmund/garmin-almaty-aqi/server/internal/aqi"
	"github.com/zigmund/garmin-almaty-aqi/server/internal/cache"
	"github.com/zigmund/garmin-almaty-aqi/server/internal/city"
)

const (
	gridSize = 24
	noData   = 255
	noDataKm = 3.0
	kNearest = 8
	kmPerDeg = 111.32
	rad      = math.Pi / 180
)

type heatmapBody struct {
	M    string       `json:"m"`
	C    string       `json:"c"`
	BBox [4]float64   `json:"bbox"`
	W    int          `json:"w"`
	H    int          `json:"h"`
	G    string       `json:"g"`
	S    [][3]float64 `json:"s"`
	T    string       `json:"t"`
}

// Heatmap builds the grid response: AQI categories over the city bbox
// (IDW of the nearest stations) plus station dots.
func Heatmap(cc city.Code, st []aai.Station, t, now time.Time) (cache.Entry, error) {
	if len(st) == 0 {
		return cache.Entry{}, errEmpty
	}
	if t.IsZero() {
		return cache.Entry{}, errZeroTime
	}
	bbox := cc.BBox()
	dots := make([][3]float64, len(st))
	for i, s := range st {
		dots[i] = [3]float64{round4(s.Lat), round4(s.Lon), float64(aqi.Category(aqi.FromPM(s.PM)))}
	}
	return entry(heatmapBody{
		M:    "city",
		C:    string(cc),
		BBox: bbox,
		W:    gridSize,
		H:    gridSize,
		G:    base64.StdEncoding.EncodeToString(computeGrid(bbox, st)),
		S:    dots,
		T:    t.UTC().Format(time.RFC3339),
	}, t, now)
}

func round4(x float64) float64 { return math.Round(x*1e4) / 1e4 }

// computeGrid builds the heat map grid: cell centers,
// equirectangular distance (cos of the pair's mean latitude), K nearest
// stations (ties keep station order), NO_DATA beyond 3 km, IDW 1/d^2.
func computeGrid(bbox [4]float64, st []aai.Station) []byte {
	w, s, e, n := bbox[0], bbox[1], bbox[2], bbox[3]
	var bd [kNearest]float64 // squared distances in degree units, ascending
	var bv [kNearest]float64
	out := make([]byte, gridSize*gridSize)
	for row := 0; row < gridSize; row++ {
		lat := n - (float64(row)+0.5)*(n-s)/gridSize
		for col := 0; col < gridSize; col++ {
			lon := w + (float64(col)+0.5)*(e-w)/gridSize
			k := 0
			for _, p := range st {
				x := (p.Lon - lon) * math.Cos((p.Lat+lat)/2*rad)
				y := p.Lat - lat
				d2 := x*x + y*y
				var j int
				switch {
				case k < kNearest:
					j = k
					k++
				case d2 < bd[kNearest-1]:
					j = kNearest - 1
				default:
					continue
				}
				for j > 0 && bd[j-1] > d2 {
					bd[j], bv[j] = bd[j-1], bv[j-1]
					j--
				}
				bd[j], bv[j] = d2, p.PM
			}
			if k == 0 || kmPerDeg*math.Sqrt(bd[0]) > noDataKm {
				out[row*gridSize+col] = noData
				continue
			}
			out[row*gridSize+col] = byte(aqi.Category(aqi.FromPM(idw(bd[:k], bv[:k]))))
		}
	}
	return out
}

func idw(d2, v []float64) float64 {
	var num, den float64
	for i := range d2 {
		d := kmPerDeg * math.Sqrt(d2[i])
		if d < 0.001 {
			return v[i]
		}
		wt := 1 / (d * d)
		num += wt * v[i]
		den += wt
	}
	return num / den
}
