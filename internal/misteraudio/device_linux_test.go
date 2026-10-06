package misteraudio

import "testing"

func TestStatusRingWrap(t *testing.T) {
	for _, tt := range []struct {
		s string
		n int
	}{
		{"rptr:      0, wptr:   2048, len:   2048, comp: 0\n", 2048},
		{"rptr: 524280, wptr:      8, len:     16, comp: 1\n", 16},
		{"rptr: 1000, wptr: 1000, len: 0, comp: 0", 0},
	} {
		n, e := ParseStatus(tt.s)
		if e != nil || n != tt.n {
			t.Fatalf("%q: %d %v", tt.s, n, e)
		}
	}
	for _, s := range []string{"", "rptr: -8, wptr: 0, len: 8, comp: 0", "rptr: 0, wptr: 8, len: 4, comp: 0", "rptr: 1, wptr: 8, len: 7, comp: 0"} {
		if _, e := ParseStatus(s); e == nil {
			t.Fatalf("accepted invalid status %q", s)
		}
	}
}
