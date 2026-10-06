package beep

import (
	"math"
	"reflect"
	"testing"
)

func stereoSource() Streamer {
	pos := 0
	return StreamerFunc(func(out [][2]float64) (int, bool) {
		n := 0
		for n < len(out) && pos < 4097 {
			out[n] = [2]float64{float64(pos%127)/63 - 1, float64(pos%83)/41 - 1}
			n++
			pos++
		}
		return n, n > 0
	})
}
func TestStereoWeightsExact(t *testing.T) {
	for _, quality := range []int{1, 3, 6, 16} {
		actual := Resample(quality, 44100, 48000, stereoSource())
		actual.SetFastPolynomial(true)
		expected := Resample(quality, 44100, 48000, stereoSource())
		for chunk := 0; chunk < 100; chunk++ {
			if chunk == 3 || chunk == 8 {
				ratio := float64(chunk) / 5
				actual.SetRatio(ratio)
				expected.SetRatio(ratio)
			}
			size := 1 + (chunk*71)%513
			a, b := make([][2]float64, size), make([][2]float64, size)
			na, oka := actual.Stream(a)
			nb, okb := referenceStereoStream(expected, b)
			equal := reflect.DeepEqual(a, b)
			if quality == 3 {
				equal = true
				for i := range a {
					for c := range a[i] {
						if math.IsNaN(a[i][c]) || math.Abs(a[i][c]-b[i][c]) > 1e-12 {
							equal = false
						}
					}
				}
			}
			if na != nb || oka != okb || !equal {
				t.Fatalf("quality %d chunk %d differs", quality, chunk)
			}
			if !oka {
				break
			}
		}
	}
}
func BenchmarkStereoWeights(b *testing.B) {
	benchmarkStereoQuality(b, 3)
}
func BenchmarkLinearWeights(b *testing.B) {
	benchmarkStereoQuality(b, 1)
}

func TestUnitRatioTransitions(t *testing.T) {
	for _, quality := range []int{1, 3, 6, 16} {
		actual := Resample(quality, 48000, 48000, stereoSource())
		expected := Resample(quality, 48000, 48000, stereoSource())
		actual.SetFastPolynomial(true)
		for chunk := 0; chunk < 200; chunk++ {
			if chunk == 5 || chunk == 9 || chunk == 15 {
				ratio := 1.0
				if chunk == 5 {
					ratio = 0.75
				}
				actual.SetRatio(ratio)
				expected.SetRatio(ratio)
			}
			a, ref := make([][2]float64, 1+(chunk*73)%600), make([][2]float64, 1+(chunk*73)%600)
			na, oka := actual.Stream(a)
			nr, okr := referenceStereoStream(expected, ref)
			if na != nr || oka != okr {
				t.Fatalf("quality=%d chunk=%d length/EOF", quality, chunk)
			}
			for i := range a {
				for c := range a[i] {
					if math.IsNaN(a[i][c]) || math.Abs(a[i][c]-ref[i][c]) > 1e-12 {
						t.Fatalf("quality=%d chunk=%d sample=%d", quality, chunk, i)
					}
				}
			}
			if !oka {
				break
			}
		}
	}
}

func BenchmarkUnitRatio(b *testing.B) {
	for _, fast := range []bool{false, true} {
		name := "interpolation"
		if fast {
			name = "copy"
		}
		b.Run(name, func(b *testing.B) {
			source := StreamerFunc(func(out [][2]float64) (int, bool) {
				for i := range out {
					out[i] = [2]float64{0.3, -0.7}
				}
				return len(out), true
			})
			r := Resample(1, 48000, 48000, source)
			r.SetFastPolynomial(fast)
			out := make([][2]float64, 512)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				r.Stream(out)
			}
		})
	}
}
func benchmarkStereoQuality(b *testing.B, quality int) {
	for _, ref := range []bool{true, false} {
		name := "shared"
		if ref {
			name = "original"
		}
		b.Run(name, func(b *testing.B) {
			samples := make([][2]float64, 512)
			source := StreamerFunc(func(out [][2]float64) (int, bool) {
				for i := range out {
					out[i] = [2]float64{0.3, -0.7}
				}
				return len(out), true
			})
			r := Resample(quality, 44100, 48000, source)
			r.SetFastPolynomial(true)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if ref {
					referenceStereoStream(r, samples)
				} else {
					r.Stream(samples)
				}
			}
		})
	}
}

func referenceStereoStream(r *Resampler, samples [][2]float64) (n int, ok bool) {
	for len(samples) > 0 {
		// Calculate the current position in the original data.
		wantPos := r.pos * r.ratio

		// Determine the quality*2 closest sample positions for the interpolation.
		// The window has length len(r.pts) and is centered around wantPos.
		windowStart := int(wantPos) - (len(r.pts)-1)/2 // (inclusive)
		windowEnd := int(wantPos) + len(r.pts)/2 + 1   // (exclusive)

		// Prepare the buffers.
		for windowEnd > r.off+resamplerSingleBufferSize {
			// We load into buf1.
			sn, _ := r.s.Stream(r.buf1)
			if sn < len(r.buf1) {
				r.end = r.off + resamplerSingleBufferSize + sn
			}

			// Swap buffers.
			r.buf1, r.buf2 = r.buf2, r.buf1
			r.off += resamplerSingleBufferSize
		}

		// Exit when wantPos is after the end of the original data.
		if int(wantPos) >= r.end {
			return n, n > 0
		}

		// Adjust the window to be within the available buffers.
		//windowStart = max(windowStart, 0)
		if windowStart < 0 {
			windowStart = 0
		}
		//windowEnd = min(windowEnd, r.end)
		if windowEnd > r.end {
			windowEnd = r.end
		}

		// For each channel...
		for c := range samples[0] {
			// Get the points.
			numPts := windowEnd - windowStart
			pts := r.pts[:numPts]
			for i := range pts {
				x := windowStart + i
				var y float64
				if x < r.off {
					// Sample is in buf1.
					offBuf1 := r.off - resamplerSingleBufferSize
					y = r.buf1[x-offBuf1][c]
				} else {
					// Sample is in buf2.
					y = r.buf2[x-r.off][c]
				}
				pts[i] = point{
					X: float64(x),
					Y: y,
				}
			}

			// Calculate the resampled sample using polynomial interpolation from the
			// quality*2 closest samples.
			samples[0][c] = lagrange(pts, wantPos)
		}

		samples = samples[1:]
		n++
		r.pos++
	}

	return n, true
}
