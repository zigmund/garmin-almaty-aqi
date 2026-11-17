// Package aqi computes US AQI from PM2.5 (EPA 2024 breakpoints).
package aqi

import "math"

// [pmLow, pmHigh, aqiLow, aqiHigh]
var breakpoints = [...][4]float64{
	{0.0, 9.0, 0, 50},
	{9.1, 35.4, 51, 100},
	{35.5, 55.4, 101, 150},
	{55.5, 125.4, 151, 200},
	{125.5, 225.4, 201, 300},
	{225.5, 325.4, 301, 500},
}

// FromPM converts a PM2.5 concentration (ug/m3) to US AQI.
func FromPM(pm float64) int {
	if !(pm > 0) {
		return 0
	}
	// Truncate to 0.1; the epsilon guards against float error (e.g. 35.4 * 10).
	c := math.Floor(pm*10+1e-9) / 10
	for _, b := range breakpoints {
		if c <= b[1] {
			// math.Round rounds half away from zero; values are positive,
			// so this matches JS Math.round.
			return int(math.Round((b[3]-b[2])/(b[1]-b[0])*(c-b[0]) + b[2]))
		}
	}
	return 500
}

// Category maps an AQI value to a category index 0..5.
func Category(aqi int) int {
	switch {
	case aqi <= 50:
		return 0
	case aqi <= 100:
		return 1
	case aqi <= 150:
		return 2
	case aqi <= 200:
		return 3
	case aqi <= 300:
		return 4
	default:
		return 5
	}
}
