package main

import (
	"math"
	"math/rand"
	"testing"
)

func TestNormalizerFixedMatchesAdaptiveOutput(t *testing.T) {
	oldRate := sys.cfg.Sound.SampleRate
	defer func() { sys.cfg.Sound.SampleRate = oldRate }()
	for _, rate := range []int32{22050, 44100, 48000} {
		sys.cfg.Sound.SampleRate = rate
		for _, volume := range []float64{0, 0.25, 1, 4} {
			ref := NormalizerLR{1, 0, 1, 1 / 32.0, 0, 0}
			got := ref
			rng := rand.New(rand.NewSource(1))
			for i := 0; i < 10000; i++ {
				a := (rng.Float64()*2 - 1) * 4
				if i > 9000 {
					a = 0
				} // filter tail after loud/clipped input
				b := a
				ref.process(volume, &a)
				got.processFixed(volume, &b)
				if math.Float64bits(a) != math.Float64bits(b) {
					t.Fatalf("rate=%d volume=%g sample=%d: got %g want %g", rate, volume, i, b, a)
				}
			}
		}
	}
}
