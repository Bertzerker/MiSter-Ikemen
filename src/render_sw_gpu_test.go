//go:build desktop || mugen

package main

import (
	"bytes"
	"encoding/binary"
	"math/rand"
	"testing"
	"time"
)

// Compare the optimized packet with the original sampler expressions, including
// fractional coordinates, flipped UVs, clipping and nonrectangular UVs.
func TestSWGPUUVRounding(t *testing.T) {
	r := newSWTestRenderer(320, 240)
	r.gpu = &swGPUState{groupRows: true, textures: make(map[swGPUTextureKey]*swGPUTexture), reasons: make(map[int]int)}
	rng := rand.New(rand.NewSource(73))
	for sample := 0; sample < 300; sample++ {
		r.gpu.active = true
		r.gpu.commands = r.gpu.commands[:0]
		u0, u1, v0, v1 := rng.Float32()*2-0.5, rng.Float32()*2-0.5, rng.Float32()*2-0.5, rng.Float32()*2-0.5
		d := swRowDraw{kind: swRowPalAO, n: 31, px0: 3, tx0: rng.Float32(), wSpanPix: 40,
			tex: &swTexture{width: 17, height: 23, serial: 991, data: make([]byte, 17*23)}, tab: make([]byte, 2048),
			l0: swVertex{3, 235.25, u0, v0}, l1: swVertex{3, 11.75, u0, v1},
			r0: swVertex{34, 235.25, u1, v0}, r1: swVertex{34, 11.75, u1, v1}}
		if sample%3 == 0 {
			d.r1.v += 0.2
		}
		if !r.gpuRows(&d, 10, 210) {
			t.Fatal("unexpected fallback")
		}
		expanded := expandGPURows(r.gpu.commands)
		if sample%3 != 0 && len(r.gpu.commands) != 51*64 {
			t.Fatal("rows were not grouped")
		}
		for py := 10; py <= 210; py++ {
			wy := float32(r.h) - float32(py) - 0.5
			tl, tr := (wy-d.l0.y)/(d.l1.y-d.l0.y), (wy-d.r0.y)/(d.r1.y-d.r0.y)
			ul, ur := d.l0.u+(d.l1.u-d.l0.u)*tl, d.r0.u+(d.r1.u-d.r0.u)*tr
			vl, vr := d.l0.v+(d.l1.v-d.l0.v)*tl, d.r0.v+(d.r1.v-d.r0.v)*tr
			want := [4]int32{
				int32((ul+d.tx0*(ur-ul))*float32(d.tex.width)*(1<<swFP) + 0.5),
				int32((vl+d.tx0*(vr-vl))*float32(d.tex.height)*(1<<swFP) + 0.5),
				int32((ur - ul) * float32(d.tex.width) * (1 << swFP) / float32(d.wSpanPix)),
				int32((vr - vl) * float32(d.tex.height) * (1 << swFP) / float32(d.wSpanPix)),
			}
			for i, value := range want {
				got := int32(binary.LittleEndian.Uint32(expanded[(py-10)*64+24+i*4:]))
				if got != value {
					t.Fatalf("sample %d row %d UV %d: %d != %d", sample, py, i, got, value)
				}
			}
		}
		r.gpu.clearReplay()
	}
}

type blockedGPUSink struct {
	*testGPUSink
	started, release chan struct{}
}

func TestSWGPUResourceCacheInvalidation(t *testing.T) {
	r := newSWTestRenderer(320, 240)
	g := &swGPUState{textures: make(map[swGPUTextureKey]*swGPUTexture)}
	r.gpu = g
	a, b := make([]byte, 2048), make([]byte, 2048)
	a[0], b[0] = 1, 2
	r.gpuBegin(true)
	first, _ := g.palette(a)
	g.palette(b)
	again, _ := g.palette(a)
	if again != first {
		t.Fatal("palette lookup changed its address")
	}
	r.gpuBegin(true)
	g.palette(a)
	if len(g.palettes) != 2048 || g.palettes[0] != 1 {
		t.Fatal("previous frame's palette pointer skipped upload")
	}
	r.gpuBegin(true)
	g.palette(b)
	next, _ := g.palette(a)
	if next != first+2048 {
		t.Fatal("palette address survived a changed frame order")
	}
	tex := &swTexture{serial: 921, width: 1, height: 1, depth: 32, data: []byte{255, 0, 0, 0}}
	original := g.texture(tex, false)
	opaque := g.texture(tex, true)
	if opaque == original || opaque.data[3] != 255 || original.data[3] != 0 {
		t.Fatal("opaque texture cache alias")
	}
	tex.dataVersion++
	tex.data[1] = 255
	updated := g.texture(tex, true)
	if updated == opaque || updated.data[1] != 255 {
		t.Fatal("stale texture after edit")
	}
	g.next = 21 * 1024 * 1024
	r.gpuBegin(true)
	recycled := g.texture(tex, true)
	if recycled == updated || recycled.address != 0x38400000 || len(g.uploads) != 1 {
		t.Fatal("stale texture after arena recycle")
	}
}

func (s *blockedGPUSink) GPUFrame(commands, palettes []byte) error {
	close(s.started)
	<-s.release
	return s.testGPUSink.GPUFrame(commands, palettes)
}

func TestSWGPUAsyncFences(t *testing.T) {
	for _, operation := range []string{"recycle", "readback", "fallback", "close"} {
		t.Run(operation, func(t *testing.T) {
			r := newSWTestRenderer(320, 240)
			sink := &blockedGPUSink{testGPUSink: &testGPUSink{pixels: make([]byte, 320*240*4)}, started: make(chan struct{}), release: make(chan struct{})}
			r.gpu = &swGPUState{sink: sink, async: true}
			r.gpuBegin(true)
			if !r.gpuEnd() {
				t.Fatal("frame was not submitted")
			}
			<-sink.started
			finished := make(chan struct{})
			go func() {
				switch operation {
				case "recycle":
					r.gpu.next = 21 * 1024 * 1024
					r.gpuBegin(true)
				case "readback":
					r.gpuSync()
				case "fallback":
					r.gpuBegin(false)
					r.gpuFallback()
				case "close":
					r.Close()
				}
				close(finished)
			}()
			select {
			case <-finished:
				close(sink.release)
				t.Fatal("operation reused resources before GPU completion")
			case <-time.After(20 * time.Millisecond):
			}
			close(sink.release)
			select {
			case <-finished:
			case <-time.After(2 * time.Second):
				t.Fatal("GPU completion did not release fence")
			}
			if sink.frames != 1 {
				t.Fatalf("submitted %d frames", sink.frames)
			}
		})
	}
}

type packetGPUSink struct {
	*testGPUSink
	started, release chan struct{}
	results          []uint32
}

func (s *packetGPUSink) GPUFrame(commands, palettes []byte) error {
	if s.frames == 0 {
		close(s.started)
		<-s.release
	}
	if err := s.testGPUSink.GPUFrame(commands, palettes); err != nil {
		return err
	}
	s.results = append(s.results, binary.LittleEndian.Uint32(s.pixels[:4]))
	return nil
}

func TestSWGPUPacketOverlap(t *testing.T) {
	r := newSWTestRenderer(320, 240)
	sink := &packetGPUSink{testGPUSink: &testGPUSink{pixels: make([]byte, 320*240*4), textures: make(map[uint32][]byte)}, started: make(chan struct{}), release: make(chan struct{})}
	r.gpu = &swGPUState{sink: sink, async: true, textures: make(map[swGPUTextureKey]*swGPUTexture), reasons: make(map[int]int)}
	q := swState()
	q.tex = &swTexture{width: 1, height: 1, depth: 8, serial: 401, data: []byte{1}}
	q.pal = &swTexture{width: 256, height: 1, depth: 32, serial: 402, data: make([]byte, 1024)}
	q.pal.data[4], q.pal.data[7] = 255, 255
	draw := func() {
		r.rasterizeQuadWindow(q, swVertex{0, 240, 0, 0}, swVertex{0, 239, 0, 1}, swVertex{1, 240, 1, 0}, swVertex{1, 239, 1, 1})
	}
	r.gpuBegin(true)
	draw()
	r.gpuEnd()
	<-sink.started
	prepared := make(chan struct{})
	go func() {
		r.gpuBegin(true)
		q.tex.data[0] = 2
		q.tex.dataVersion++
		q.pal.data[9], q.pal.data[11] = 255, 255
		q.pal.dataVersion++
		draw()
		close(prepared)
	}()
	select {
	case <-prepared:
	case <-time.After(2 * time.Second):
		close(sink.release)
		t.Fatal("CPU packet preparation waited for the GPU")
	}
	// Submitting the next packet must still wait: neither DDR uploads nor the
	// prior packet's command/palette storage can be replaced while it runs.
	finished := make(chan struct{})
	go func() { r.gpuEnd(); r.gpuSync(); close(finished) }()
	select {
	case <-finished:
		close(sink.release)
		t.Fatal("next submission bypassed the GPU fence")
	case <-time.After(20 * time.Millisecond):
	}
	close(sink.release)
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("packets did not complete")
	}
	if len(sink.results) != 2 || sink.results[0] != 0xff0000ff || sink.results[1] != 0xff00ff00 {
		t.Fatalf("in-flight command, palette or texture data changed: %x", sink.results)
	}
}

// Protocol model for ARM encoder tests; FPGA arithmetic/DMA are independently
// checked by the Verilog tests. The oracle here remains the existing rasterizer.
type testGPUSink struct {
	pixels   []byte
	textures map[uint32][]byte
	frames   int
}

func (s *testGPUSink) GPUAvailable() bool { return true }
func (s *testGPUSink) GPUUpload(a uint32, b []byte) error {
	s.textures[a] = append([]byte(nil), b...)
	return nil
}
func (s *testGPUSink) GPUReadback(b []byte) error { copy(b, s.pixels); return nil }
func (s *testGPUSink) GPUFrame(commands, palettes []byte) error {
	s.frames++
	commands = expandGPURows(commands)
	for off := 0; off < len(commands); off += 64 {
		var c [16]uint32
		for i := range c {
			c[i] = binary.LittleEndian.Uint32(commands[off+i*4:])
		}
		kind := c[0] & 3
		mode := int(c[0] >> 8)
		u, v := int32(c[6]), int32(c[7])
		for p := int(c[1]); p < int(c[1]+c[2]); p++ {
			dst := s.pixels[p*4 : p*4+4]
			if kind == 3 {
				clear(dst)
				continue
			}
			var entry [8]byte
			if kind == 0 {
				binary.LittleEndian.PutUint32(entry[:], c[11])
				binary.LittleEndian.PutUint32(entry[4:], c[12])
			} else {
				x, y := int(u>>16), int(v>>16)
				if x < 0 {
					x = 0
				}
				if y < 0 {
					y = 0
				}
				if x >= int(c[4]) {
					x = int(c[4]) - 1
				}
				if y >= int(c[5]) {
					y = int(c[5]) - 1
				}
				tex := s.textures[c[3]]
				index := y*int(c[4]) + x
				if kind == 1 {
					start := int(c[10]-0x38200000) + int(tex[index])*8
					copy(entry[:], palettes[start:start+8])
				} else {
					copy(entry[4:7], tex[index*4:index*4+3])
					a := int(tex[index*4+3])
					entry[3] = byte(a)
					for j := 0; j < 3; j++ {
						entry[j] = byte((int(entry[j+4])*a + 127) / 255)
					}
				}
			}
			swBlendPixRef(dst, [3]int{int(entry[4]), int(entry[5]), int(entry[6])}, [3]int{int(entry[0]), int(entry[1]), int(entry[2])}, int(entry[3]), mode)
			u += int32(c[8])
			v += int32(c[9])
		}
	}
	return nil
}

func expandGPURows(commands []byte) []byte {
	var out []byte
	for off := 0; off < len(commands); off += 64 {
		packet := commands[off : off+64]
		flags := binary.LittleEndian.Uint32(packet)
		for row := uint32(0); row <= (flags>>4)&3; row++ {
			start := len(out)
			out = append(out, packet...)
			binary.LittleEndian.PutUint32(out[start:], flags&^0x30)
			binary.LittleEndian.PutUint32(out[start+4:], binary.LittleEndian.Uint32(packet[4:])+row*320)
			if row != 0 {
				copy(out[start+28:start+32], packet[48+row*4:52+row*4])
			}
		}
	}
	return out
}
func TestSWGPUCommandsAndFallback(t *testing.T) {
	ref := newSWTestRenderer(320, 240)
	hw := newSWTestRenderer(320, 240)
	sink := &testGPUSink{pixels: make([]byte, 320*240*4), textures: make(map[uint32][]byte)}
	hw.gpu = &swGPUState{sink: sink, textures: make(map[swGPUTextureKey]*swGPUTexture), reasons: make(map[int]int)}
	hw.gpu.async = true
	tex := &swTexture{width: 32, height: 16, depth: 8, serial: 100, data: make([]byte, 512)}
	pal := &swTexture{width: 256, height: 1, depth: 32, serial: 101, data: make([]byte, 1024)}
	for i := range tex.data {
		tex.data[i] = byte(i)
	}
	for i := 0; i < 256; i++ {
		pal.data[i*4] = byte(i)
		pal.data[i*4+1] = byte(255 - i)
		pal.data[i*4+2] = 77
		pal.data[i*4+3] = byte(i)
	}
	q := swState()
	q.tex = tex
	q.pal = pal
	vertices := [4]swVertex{{10, 220, 0, 0}, {10, 130, 0, 1}, {280, 220, 1, 0}, {280, 130, 1, 1}}
	draw := func(r *Renderer_SW) { r.rasterizeQuadWindow(q, vertices[0], vertices[1], vertices[2], vertices[3]) }
	hw.gpuBegin(true)
	draw(ref)
	draw(hw)
	if !hw.gpuEnd() {
		t.Fatal("indexed frame unexpectedly fell back")
	}
	hw.gpuSync()
	if !bytes.Equal(ref.pix, hw.pix) {
		t.Fatal("hardware commands differ from software pixels")
	}
	// Preserve the prior frame, then force an unsupported RGBA draw after an accelerated
	// draw. All queued draws must replay over an exact previous-frame readback.
	clear(hw.pix)
	hw.gpu.stale = true
	hw.gpuBegin(false)
	vertices[0].u = 1
	vertices[1].u = 1
	vertices[2].u = 0
	vertices[3].u = 0
	draw(ref)
	draw(hw)
	vertices[0].x += 20
	q.pal = nil
	q.tex = &swTexture{width: 32, height: 16, depth: 32, serial: 109, data: make([]byte, 32*16*4)}
	for i := range q.tex.data {
		q.tex.data[i] = byte(i * 37)
	}
	draw(ref)
	draw(hw)
	if hw.gpuEnd() {
		t.Fatal("generic RGBA frame must use the correctness fallback")
	}
	if !bytes.Equal(ref.pix, hw.pix) {
		t.Fatal("mixed-frame fallback changed pixels/order")
	}
	if sink.frames != 1 {
		t.Fatal("fallback submitted a partial hardware frame")
	}
}

func TestSWGPUTextureAndBlendModes(t *testing.T) {
	for kind := 0; kind < 3; kind++ {
		for mode := 0; mode < 8; mode++ {
			ref, hw := newSWTestRenderer(320, 240), newSWTestRenderer(320, 240)
			sink := &testGPUSink{pixels: make([]byte, 320*240*4), textures: make(map[uint32][]byte)}
			hw.gpu = &swGPUState{groupRows: true, sink: sink, textures: make(map[swGPUTextureKey]*swGPUTexture), reasons: make(map[int]int)}
			hw.gpuBegin(true)
			background := swState()
			background.isFlat, background.blending = true, false
			background.tint = [4]float32{0.3, 0.6, 0.9, 1}
			q := swState()
			q.src = []BlendFunc{BlendOne, BlendSrcAlpha, BlendOne, BlendSrcAlpha, BlendZero, BlendOne, BlendSrcAlpha, BlendOne}[mode]
			q.dst = BlendOne
			if mode >= 2 && mode <= 4 {
				q.dst = BlendOneMinusSrcAlpha
			}
			if mode == 5 || mode == 6 {
				q.eq = BlendReverseSubtract
			}
			q.blending = mode != 7
			q.isFlat = kind == 0
			if q.isFlat {
				q.tint = [4]float32{0.9, 0.4, 0.1, 0.7}
			}
			tex := &swTexture{width: 16, height: 16, depth: 32, serial: 201, data: make([]byte, 1024)}
			for i := range tex.data {
				tex.data[i] = byte(i*37 + i/4)
			}
			q.tex = tex
			if kind == 1 {
				q.pal = tex
				q.tex = &swTexture{width: 16, height: 16, depth: 8, serial: 202, data: make([]byte, 256)}
				for i := range q.tex.data {
					q.tex.data[i] = byte(i)
				}
			}
			for _, r := range []*Renderer_SW{ref, hw} {
				r.rasterizeQuadWindow(background, swVertex{0, 240, 0, 0}, swVertex{0, 0, 0, 1}, swVertex{320, 240, 1, 0}, swVertex{320, 0, 1, 1})
				r.rasterizeQuadWindow(q, swVertex{13, 219, 1, 0}, swVertex{13, 21, 1, 1}, swVertex{300, 219, 0, 0}, swVertex{300, 21, 0, 1})
			}
			if !hw.gpuEnd() {
				t.Fatalf("kind %d mode %d unexpectedly fell back", kind, mode)
			}
			hw.gpuSync()
			if !bytes.Equal(ref.pix, hw.pix) {
				t.Fatalf("kind %d mode %d pixels differ", kind, mode)
			}
		}
	}
}

func TestSWGPURepeatedPaletteFrame(t *testing.T) {
	ref, hw := newSWTestRenderer(320, 240), newSWTestRenderer(320, 240)
	sink := &testGPUSink{pixels: make([]byte, 320*240*4), textures: make(map[uint32][]byte)}
	hw.gpu = &swGPUState{sink: sink, textures: make(map[swGPUTextureKey]*swGPUTexture), reasons: make(map[int]int)}
	q := swState()
	q.tex = &swTexture{width: 1, height: 1, depth: 8, serial: 301, data: []byte{1}}
	q.pal = &swTexture{width: 256, height: 1, depth: 32, serial: 302, data: make([]byte, 1024)}
	q.pal.data[4], q.pal.data[7] = 255, 255
	hw.gpuBegin(true)
	for i := 0; i < 1100; i++ {
		if i == 550 {
			q.pal.data[4], q.pal.data[5] = 0, 255
			q.pal.dataVersion++
		}
		x, y := float32(i%320), float32(240-i/320)
		for _, r := range []*Renderer_SW{ref, hw} {
			r.rasterizeQuadWindow(q, swVertex{x, y, 0, 0}, swVertex{x, y - 1, 0, 1}, swVertex{x + 1, y, 1, 0}, swVertex{x + 1, y - 1, 1, 1})
		}
	}
	if !hw.gpuEnd() {
		t.Fatal("repeated palettes exhausted the frame arena")
	}
	if len(hw.gpu.palettes) != 4096 {
		t.Fatalf("uploaded %d bytes, want two distinct palettes", len(hw.gpu.palettes))
	}
	hw.gpuSync()
	if !bytes.Equal(ref.pix, hw.pix) {
		t.Fatal("palette reuse or mutation changed pixels")
	}
}
