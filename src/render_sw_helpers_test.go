//go:build desktop || mugen

package main

import (
	"math"
	"math/rand"
	"testing"
)

func TestSWFloat32Helpers(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 100000; i++ {
		v := math.Float32frombits(rng.Uint32())
		want := float32(math.Mod(float64(v), 1))
		if want < 0 {
			want++
		}
		got := mod1f(v)
		if !math.IsNaN(float64(want)) && math.Float32bits(got) != math.Float32bits(want) {
			t.Fatalf("mod(%g): %g != %g", v, got, want)
		}
		if v >= -1<<30 && v <= 1<<30 && swFloorPixel(v) != int(math.Floor(float64(v))) {
			t.Fatalf("floor(%g)", v)
		}
		if math.Float32bits(swAbs32(v)) != math.Float32bits(float32(math.Abs(float64(v)))) && !math.IsNaN(float64(v)) {
			t.Fatalf("abs(%g)", v)
		}
	}
}

func BenchmarkSWMod1(b *testing.B) {
	for _, old := range []bool{true, false} {
		name := "float32"
		if old {
			name = "original"
		}
		b.Run(name, func(b *testing.B) {
			var result float32
			for i := 0; i < b.N; i++ {
				x := float32(i%8192)/100 - 40
				if old {
					result = float32(math.Mod(float64(x), 1))
					if result < 0 {
						result++
					}
				} else {
					result = mod1f(x)
				}
			}
			helperResult = result
		})
	}
}

var helperResult float32
