//go:build desktop || mugen

package main

import (
	"bytes"
	"testing"
)

func TestSWGPUGenericSpans(t *testing.T) {
	shapes := [][4]swVertex{
		{{28, 223, 0, 0}, {13, 192, 0, 1}, {73, 210, 1, 0}, {58, 179, 1, 1}},
		{{-8, 251, 1.2, -0.3}, {3, 210, 1.2, 1.1}, {42, 239, -0.2, -0.3}, {53, 198, -0.2, 1.1}},
		{{15, 220, 0, 0}, {15, 220, 0, 1}, {15, 220, 1, 0}, {15, 220, 1, 1}},
		{{25, 225, 0, 0}, {10, 184, 0, 1}, {50, 225, 1, 0}, {76, 184, 1, 1}},
		{{60, 235, 0, 0}, {1, 91, 0, 1}, {315, 191, 1, 0}, {255, 47, 1, 1}},
		{{5, 235, 0, 0}, {310, 8, 0, 1}, {315, 230, 1, 0}, {1, 13, 1, 1}},
	}
	for mode := 0; mode < 8; mode++ {
		for shape, v := range shapes {
			for replay := 0; replay < 2; replay++ {
				ref, hw := newSWTestRenderer(320, 240), newSWTestRenderer(320, 240)
				sink := &testGPUSink{pixels: make([]byte, 320*240*4), textures: make(map[uint32][]byte)}
				for i := range ref.pix {
					ref.pix[i] = byte(i*17 + 61)
				}
				copy(hw.pix, ref.pix)
				copy(sink.pixels, ref.pix)
				hw.gpu = &swGPUState{sink: sink, lastHardware: true, reasons: make(map[int]int)}
				hw.gpuBegin(false)
				q := swState()
				q.tex = &swTexture{width: 16, height: 16, depth: 8, data: make([]byte, 256)}
				q.pal = &swTexture{}
				for i := range q.tex.data {
					q.tex.data[i] = byte(i * 7)
				}
				tab := make([]byte, 2048)
				for i := 0; i < 256; i++ {
					a := i
					if i%5 == 0 {
						a = 0
					} // Preserve nonzero raw RGB behind zero alpha.
					tab[i*8+3] = byte(a)
					for channel := 0; channel < 3; channel++ {
						raw := (i*23 + channel*49) & 255
						tab[i*8+channel+4] = byte(raw)
						tab[i*8+channel] = byte((raw*a + 127) / 255)
					}
				}
				if shape == 3 {
					q.isTrapez = true
					q.x1x2x4x3 = [4]float32{25, 50, 10, 76}
				}
				ref.rasterGeneric(q, v[0], v[1], v[2], v[3], mode, tab)
				hw.rasterGeneric(q, v[0], v[1], v[2], v[3], mode, tab)
				if len(hw.gpu.commands) > 480*64 {
					t.Fatal("generic packet grew with pixel/color count")
				}
				if !hw.gpu.active {
					t.Fatalf("mode %d shape %d unexpectedly fell back", mode, shape)
				}
				if replay != 0 {
					// A later unsupported draw must replay the original colors,
					// even if the source texture/palette has since changed.
					clear(q.tex.data)
					clear(tab)
					hw.gpuFallback()
				} else {
					if !hw.gpuEnd() {
						t.Fatal("unexpected fallback")
					}
					hw.gpuSync()
				}
				if !bytes.Equal(ref.pix, hw.pix) {
					t.Fatalf("mode %d shape %d replay %d pixels differ", mode, shape, replay)
				}
			}
		}
	}
}

func TestSWGPUGenericSpanCapacity(t *testing.T) {
	r := newSWTestRenderer(320, 240)
	r.gpu = &swGPUState{active: true, commands: make([]byte, 2*1024*1024-64)}
	q := swState()
	q.tex = &swTexture{width: 2, height: 2, depth: 8, data: []byte{0, 1, 1, 0}}
	q.pal = &swTexture{}
	tab := make([]byte, 2048)
	tab[3] = 255
	tab[8] = 127
	tab[11] = 255
	d := swRowDraw{r: r, q: q, tab: tab, px0: 0, px1: 3, mode: 3,
		v0: swVertex{0, 240, 0, 0}, v1: swVertex{0, 236, 0, 1}, v2: swVertex{4, 240, 1, 0}, v3: swVertex{4, 236, 1, 1}}
	before := len(r.gpu.commands)
	if r.gpuGenericSpans(&d, 0, 3) {
		t.Fatal("over-capacity spans accepted")
	}
	if len(r.gpu.commands) != before || len(r.gpu.replay) != 0 {
		t.Fatal("partial generic commands escaped rollback")
	}
}
