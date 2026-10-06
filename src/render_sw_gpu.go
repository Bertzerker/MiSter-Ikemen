//go:build desktop || mugen

package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"slices"
)

// Optional native FPGA row backend. Unsupported frames replay the saved draws
// in their original order on the CPU, including a previous-frame readback when
// the caller requested preservation rather than clearing.
type swGPUSink interface {
	GPUAvailable() bool
	GPUUpload(uint32, []byte) error
	GPUFrame([]byte, []byte) error
	GPUReadback([]byte) error
}
type swGPUTextureKey struct {
	serial, version uint64
	opaque          bool
}
type swGPUTexture struct {
	address uint32
	data    []byte
}
type swGPUReplay struct {
	draw        swRowDraw
	state       swQuadState
	texture     swTexture
	first, last int
}
type swGPUState struct {
	sink                         swGPUSink
	active, stale, lastHardware  bool
	async                        bool
	groupRows                    bool
	pending                      chan error
	commands, palettes           []byte
	spareCommands, sparePalettes []byte
	uploads                      []*swGPUTexture
	paletteAddresses             map[string]uint32
	palettePointers              map[*byte]uint32
	lastPalettePointer           *byte
	lastPaletteAddress           uint32
	replay                       []*swGPUReplay
	textures                     map[swGPUTextureKey]*swGPUTexture
	lastTextureKey               swGPUTextureKey
	lastTexture                  *swGPUTexture
	next                         uint32
	hardware, software           int
	genericDraws                 uint64
	reasons                      map[int]int
	details                      map[string]int
}

func (r *Renderer_SW) gpuBegin(clearColor bool) {
	g := r.gpu
	if g == nil {
		return
	}
	if clearColor {
		g.stale = false
	}
	g.active = clearColor || g.lastHardware
	g.commands = g.commands[:0]
	g.palettes = g.palettes[:0]
	clear(g.paletteAddresses)
	clear(g.palettePointers)
	g.lastPalettePointer = nil
	g.replay = g.replay[:0]
	// Never recycle texture addresses within an in-flight frame.
	if g.next > 20*1024*1024 {
		r.gpuWait()
		g.textures = make(map[swGPUTextureKey]*swGPUTexture)
		g.lastTexture = nil
		clear(g.uploads)
		g.uploads = g.uploads[:0]
		g.next = 0
	}
	if g.active && clearColor {
		c := [16]uint32{3, 0, 320 * 240}
		g.command(c)
	}
}
func (g *swGPUState) command(c [16]uint32) {
	start := len(g.commands)
	g.commands = append(g.commands, make([]byte, 64)...)
	for i, v := range c {
		binary.LittleEndian.PutUint32(g.commands[start+i*4:], v)
	}
}

func (g *swGPUState) palette(data []byte) (uint32, bool) {
	if len(data) != 2048 {
		return 0, false
	}
	if g.lastPalettePointer == &data[0] {
		return g.lastPaletteAddress, true
	}
	// rasterizeQuadWindow supplies immutable cached tables. Repeated tiles
	// normally share that exact table; avoid hashing/comparing all 2048 bytes.
	if address, ok := g.palettePointers[&data[0]]; ok {
		g.lastPalettePointer, g.lastPaletteAddress = &data[0], address
		return address, true
	}
	if g.palettePointers == nil {
		g.palettePointers = make(map[*byte]uint32)
	}
	// Byte-slice-to-string map lookup does not allocate. Only a new palette
	// needs an owned key; identical tiles share its upload and replay snapshot.
	if address, ok := g.paletteAddresses[string(data)]; ok {
		g.palettePointers[&data[0]] = address
		g.lastPalettePointer, g.lastPaletteAddress = &data[0], address
		return address, true
	}
	if len(g.palettes)+len(data) > 2*1024*1024 {
		return 0, false
	}
	if g.paletteAddresses == nil {
		g.paletteAddresses = make(map[string]uint32)
	}
	address := 0x38200000 + uint32(len(g.palettes))
	g.palettes = append(g.palettes, data...)
	g.paletteAddresses[string(data)] = address
	g.palettePointers[&data[0]] = address
	g.lastPalettePointer, g.lastPaletteAddress = &data[0], address
	return address, true
}

// A single frame may run while the engine prepares the next CPU-side packet.
// Fence before submitting/recycling DDR storage, readback, or shutdown.
func (r *Renderer_SW) gpuWait() {
	if g := r.gpu; g != nil && g.pending != nil {
		stallStart := perfStallBegin()
		err := <-g.pending
		perfStallEnd(stallStart, "gpu-wait")
		g.pending = nil
		if err != nil {
			panic(fmt.Errorf("Frontier GPU frame: %w", err))
		}
	}
}
func (r *Renderer_SW) gpuSync() {
	r.gpuWait()
	if g := r.gpu; g != nil && g.stale {
		if err := g.sink.GPUReadback(r.pix); err != nil {
			panic(err)
		}
		g.stale = false
	}
}
func (r *Renderer_SW) gpuFallback() {
	g := r.gpu
	if g == nil {
		return
	}
	r.gpuSync()
	if !g.active {
		return
	}
	g.active = false
	for _, saved := range g.replay {
		saved.draw.draw(saved.first, saved.last)
	}
	g.clearReplay()
}

func (g *swGPUState) clearReplay() {
	// Reuse the records without retaining old texture/palette storage.
	for _, saved := range g.replay {
		// Release references without zeroing numeric state that is overwritten
		// by the next draw. Each record holds its own fallback snapshot.
		saved.draw.r, saved.draw.q, saved.draw.tex, saved.draw.tab = nil, nil, nil, nil
		saved.state.tex, saved.state.pal, saved.state.fontCov = nil, nil, nil
		saved.texture.data = nil
	}
	g.replay = g.replay[:0]
}
func (r *Renderer_SW) gpuEnd() bool {
	g := r.gpu
	if g == nil {
		return false
	}
	hardware := g.active
	if hardware {
		r.gpuWait()
		// New textures were staged in ordinary ARM RAM while the prior packet
		// was running. All transport operations remain strictly ordered here.
		uploadStart := perfStallBegin()
		for _, entry := range g.uploads {
			if err := g.sink.GPUUpload(entry.address, entry.data); err != nil {
				panic(err)
			}
		}
		perfStallEnd(uploadStart, "gpu-texture-uploads")
		clear(g.uploads)
		g.uploads = g.uploads[:0]
		if g.async {
			commands, palettes := g.commands, g.palettes
			g.commands, g.palettes = g.spareCommands[:0], g.sparePalettes[:0]
			g.spareCommands, g.sparePalettes = commands, palettes
			done := make(chan error, 1)
			g.pending = done
			go func() {
				stallStart := perfStallBegin()
				err := g.sink.GPUFrame(commands, palettes)
				perfStallEnd(stallStart, "gpu-submit-through-completion")
				done <- err
			}()
		} else if err := g.sink.GPUFrame(g.commands, g.palettes); err != nil {
			panic(fmt.Errorf("Frontier GPU frame: %w", err))
		}
		g.stale = true
		g.hardware++
	} else {
		g.software++
	}
	g.lastHardware = hardware
	g.active = false
	g.clearReplay()
	if (g.hardware+g.software)%120 == 0 {
		fmt.Fprintf(os.Stderr, "[Frontier GPU] hardware=%d software=%d generic-images=%d fallback kinds=%v\n", g.hardware, g.software, g.genericDraws, g.reasons)
	}
	return hardware
}
func (g *swGPUState) texture(t *swTexture, opaque bool) *swGPUTexture {
	key := swGPUTextureKey{t.serial, t.dataVersion, opaque}
	if g.lastTexture != nil && g.lastTextureKey == key {
		return g.lastTexture
	}
	if entry := g.textures[key]; entry != nil {
		g.lastTextureKey, g.lastTexture = key, entry
		return entry
	}
	size := len(t.data)
	aligned := (size + 63) &^ 63
	if size == 0 || uint64(g.next)+uint64(aligned) > 24*1024*1024 {
		return nil
	}
	entry := &swGPUTexture{address: 0x38400000 + g.next, data: append([]byte(nil), t.data...)}
	if opaque {
		for i := 3; i < len(entry.data); i += 4 {
			entry.data[i] = 255
		}
	}
	g.uploads = append(g.uploads, entry)
	g.next += uint32(aligned)
	g.textures[key] = entry
	g.lastTextureKey, g.lastTexture = key, entry
	return entry
}
func (r *Renderer_SW) gpuRows(d *swRowDraw, first, last int) bool {
	g := r.gpu
	if g == nil || !g.active {
		return false
	}
	unsupported := func(reason string) bool {
		g.reasons[d.kind]++
		if g.details == nil {
			g.details = make(map[string]int)
		}
		g.details[reason]++
		if g.details[reason] == 1 {
			w, h, size := int32(0), int32(0), 0
			tex := d.tex
			if tex == nil && d.q != nil {
				tex = d.q.tex
			}
			if tex != nil {
				w, h, size = tex.width, tex.height, len(tex.data)
			}
			fmt.Fprintf(os.Stderr, "[Frontier GPU] fallback: %s kind=%d commands=%d palettes=%d textureUsed=%d texture=%dx%d bytes=%d table=%d rows=%d\n", reason, d.kind, len(g.commands)/64, len(g.palettes)/2048, g.next, w, h, size, len(d.tab), last-first+1)
		}
		r.gpuFallback()
		return false
	}
	if last < first {
		return unsupported("command capacity or empty range")
	}
	if d.kind == swRowGeneric {
		if r.gpuGenericSpans(d, first, last) {
			g.genericDraws++
			return true
		}
		return unsupported("generic span limits or source")
	}
	var c [16]uint32
	mode := d.mode
	var texture *swGPUTexture
	var table []byte
	switch d.kind {
	case swRowFlatAO, swRowFlatOneOne, swRowFlatSrcAlphaOne, swRowFlatOneInvAlpha, swRowFlatZeroInvAlpha, swRowFlatSubOneOne, swRowFlatSubSrcAlphaOne, swRowFlatReplace:
		mode = []int{3, 0, 1, 2, 4, 5, 6, 7}[d.kind]
		c[11] = uint32(d.sp0) | uint32(d.sp1)<<8 | uint32(d.sp2)<<16 | uint32(d.sa)<<24
		c[12] = uint32(d.s0) | uint32(d.s1)<<8 | uint32(d.s2)<<16
	case swRowPalAO, swRowPalGeneric:
		if d.kind == swRowPalAO {
			mode = 3
		}
		address, ok := g.palette(d.tab)
		if !ok {
			return unsupported("palette table or capacity")
		}
		texture = g.texture(d.tex, false)
		if texture == nil {
			return unsupported("texture capacity or empty data")
		}
		c[0] = 1
		c[10] = address
		offset := int(address - 0x38200000)
		table = g.palettes[offset : offset+2048]
	case swRowRGBANearest:
		q := d.q
		if d.tex.depth != 32 || q.fontMode || q.alpha != 1 || q.hue != 0 || q.gray != 0 || q.neg || q.add != [3]float32{} || q.mult != [3]float32{1, 1, 1} || q.tint[3] != 0 {
			return unsupported("RGBA effects")
		}
		texture = g.texture(d.tex, q.mask == -1)
		if texture == nil {
			return unsupported("texture capacity or empty data")
		}
		c[0] = 2
	default:
		return unsupported("drawing kind")
	}
	c[0] |= uint32(mode) << 8
	c[2] = uint32(d.n)
	if texture != nil {
		if d.tex.width <= 0 || d.tex.height <= 0 || d.tex.width > 8192 || d.tex.height > 8192 {
			return unsupported("texture dimensions")
		}
		c[3] = texture.address
		c[4] = uint32(d.tex.width)
		c[5] = uint32(d.tex.height)
	}
	// Most words are constant for every row in a sprite. Encode the template
	// once, then copy it and update only destination/UV words. In particular,
	// avoid clearing and encoding sixteen words separately for every tiny row.
	start := len(g.commands)
	// Rectangular UVs have constant U across rows and constant V across each
	// row. Keep the software sampler's operation order/rounding, but perform
	// the invariant U calculations only once per sprite.
	rectUV := texture != nil && d.l0.u == d.l1.u && d.r0.u == d.r1.u &&
		d.l0.v == d.r0.v && d.l1.v == d.r1.v &&
		d.l0.y == d.r0.y && d.l1.y == d.r1.y && d.l0.y != d.l1.y
	if rectUV {
		c[6] = uint32(int32((d.l0.u+d.tx0*(d.r0.u-d.l0.u))*float32(d.tex.width)*(1<<swFP) + 0.5))
		c[8] = uint32(int32((d.r0.u - d.l0.u) * float32(d.tex.width) * (1 << swFP) / float32(d.wSpanPix)))
	}
	shift := uint(0)
	if g.groupRows && (rectUV || texture == nil) {
		shift = 2
	}
	mask := (1 << shift) - 1
	commandCount := (last - first + 1 + mask) >> shift
	if start+commandCount*64 > 2*1024*1024 {
		return unsupported("command capacity")
	}
	g.commands = slices.Grow(g.commands, commandCount*64)
	g.command(c)
	template := g.commands[start : start+64]
	g.commands = g.commands[:start+commandCount*64]
	// The same float32 expression and rounding as the software nearest sampler.
	for py := first; py <= last; py++ {
		index, subrow := (py-first)>>shift, (py-first)&mask
		row := g.commands[start+index*64 : start+(index+1)*64]
		if subrow == 0 {
			if py != first {
				copy(row, template)
			}
			binary.LittleEndian.PutUint32(row[0:4], c[0]|uint32(min(mask, last-py))<<4)
			binary.LittleEndian.PutUint32(row[4:8], uint32(py*320+d.px0))
		}
		if rectUV {
			wy := float32(r.h) - float32(py) - 0.5
			tl := (wy - d.l0.y) / (d.l1.y - d.l0.y)
			vl := d.l0.v + (d.l1.v-d.l0.v)*tl
			offset := 28
			if subrow != 0 {
				offset = 48 + subrow*4
			}
			binary.LittleEndian.PutUint32(row[offset:offset+4], uint32(int32(vl*float32(d.tex.height)*(1<<swFP)+0.5)))
		} else if texture != nil {
			wy := float32(r.h) - float32(py) - 0.5
			tl := (wy - d.l0.y) / (d.l1.y - d.l0.y)
			tr := (wy - d.r0.y) / (d.r1.y - d.r0.y)
			ul := d.l0.u + (d.l1.u-d.l0.u)*tl
			ur := d.r0.u + (d.r1.u-d.r0.u)*tr
			vl := d.l0.v + (d.l1.v-d.l0.v)*tl
			vr := d.r0.v + (d.r1.v-d.r0.v)*tr
			binary.LittleEndian.PutUint32(row[24:28], uint32(int32((ul+d.tx0*(ur-ul))*float32(d.tex.width)*(1<<swFP)+0.5)))
			binary.LittleEndian.PutUint32(row[28:32], uint32(int32((vl+d.tx0*(vr-vl))*float32(d.tex.height)*(1<<swFP)+0.5)))
			binary.LittleEndian.PutUint32(row[32:36], uint32(int32((ur-ul)*float32(d.tex.width)*(1<<swFP)/float32(d.wSpanPix))))
			binary.LittleEndian.PutUint32(row[36:40], uint32(int32((vr-vl)*float32(d.tex.height)*(1<<swFP)/float32(d.wSpanPix))))
		}
	}
	var saved *swGPUReplay
	if len(g.replay) < cap(g.replay) {
		saved = g.replay[:cap(g.replay)][len(g.replay)]
	}
	if saved == nil {
		saved = &swGPUReplay{}
	}
	saved.draw, saved.first, saved.last = *d, first, last
	if d.q != nil {
		saved.state = *d.q
		saved.draw.q = &saved.state
	}
	if texture != nil {
		saved.texture = *d.tex
		saved.texture.data = texture.data
		saved.draw.tex = &saved.texture
	}
	saved.draw.tab = table
	g.replay = append(g.replay, saved)
	return true
}
