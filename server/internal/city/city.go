// Package city defines the supported cities and their station bounding boxes.
package city

import "math"

// Code identifies a supported city.
type Code string

const (
	Almaty    Code = "almaty"
	Karaganda Code = "karaganda"
)

const (
	marginKm = 15.0
	kmPerDeg = 111.32
)

// all is in a fixed order so Locate is deterministic.
var all = [...]Code{Almaty, Karaganda}

// Parse returns the city for an exact (lowercase) code.
func Parse(s string) (Code, bool) {
	c := Code(s)
	for _, x := range all {
		if x == c {
			return c, true
		}
	}
	return "", false
}

// BBox returns [west, south, east, north] of the city's station area.
func (c Code) BBox() [4]float64 {
	switch c {
	case Almaty:
		return [4]float64{76.6, 43.14, 77.26, 43.5}
	case Karaganda:
		return [4]float64{72.9, 49.7, 73.3, 49.95}
	}
	return [4]float64{}
}

// Locate returns the city whose bbox, expanded by 15 km, contains the point.
func Locate(lat, lon float64) (Code, bool) {
	dLat := marginKm / kmPerDeg
	for _, c := range all {
		b := c.BBox()
		dLon := marginKm / (kmPerDeg * math.Cos((b[1]+b[3])/2*math.Pi/180))
		if lat >= b[1]-dLat && lat <= b[3]+dLat && lon >= b[0]-dLon && lon <= b[2]+dLon {
			return c, true
		}
	}
	return "", false
}
