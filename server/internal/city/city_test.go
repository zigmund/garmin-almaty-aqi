package city

import "testing"

func TestParse(t *testing.T) {
	for _, s := range []string{"almaty", "karaganda"} {
		if c, ok := Parse(s); !ok || string(c) != s {
			t.Errorf("Parse(%q) = %q, %v", s, c, ok)
		}
	}
	for _, s := range []string{"", "out", "Almaty"} {
		if _, ok := Parse(s); ok {
			t.Errorf("Parse(%q) should fail", s)
		}
	}
}

func TestBBox(t *testing.T) {
	if got := Almaty.BBox(); got != [4]float64{76.6, 43.14, 77.26, 43.5} {
		t.Errorf("almaty bbox = %v", got)
	}
	if got := Karaganda.BBox(); got != [4]float64{72.9, 49.7, 73.3, 49.95} {
		t.Errorf("karaganda bbox = %v", got)
	}
}

func TestLocate(t *testing.T) {
	n := Almaty.BBox()[3]
	cases := []struct {
		name     string
		lat, lon float64
		want     Code
		ok       bool
	}{
		{"almaty center", 43.24, 76.92, Almaty, true},
		{"almaty 2", 43.2389, 76.8897, Almaty, true},
		{"karaganda", 49.8, 73.1, Karaganda, true},
		{"astana", 51.13, 71.43, "", false},
		{"moscow", 55.75, 37.62, "", false},
		{"14.9 km north", n + 14.9/111.32, 76.9, Almaty, true},
		{"15.1 km north", n + 15.1/111.32, 76.9, "", false},
	}
	for _, c := range cases {
		got, ok := Locate(c.lat, c.lon)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: Locate = %q, %v; want %q, %v", c.name, got, ok, c.want, c.ok)
		}
	}
}
