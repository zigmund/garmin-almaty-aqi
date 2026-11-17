package aqi

import "testing"

func TestFromPM(t *testing.T) {
	cases := []struct {
		pm   float64
		want int
	}{
		{0, 0}, {8.8, 49}, {9.0, 50}, {9.1, 51}, {35.4, 100}, {35.5, 101},
		{55.4, 150}, {125.4, 200}, {225.4, 300}, {325.4, 500}, {999, 500},
		{8.86, 49}, {-3, 0},
	}
	for _, c := range cases {
		if got := FromPM(c.pm); got != c.want {
			t.Errorf("FromPM(%v) = %d, want %d", c.pm, got, c.want)
		}
	}
}

func TestCategory(t *testing.T) {
	in := []int{0, 50, 51, 100, 101, 150, 151, 200, 201, 300, 301, 500}
	want := []int{0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5}
	for i, a := range in {
		if got := Category(a); got != want[i] {
			t.Errorf("Category(%d) = %d, want %d", a, got, want[i])
		}
	}
}
