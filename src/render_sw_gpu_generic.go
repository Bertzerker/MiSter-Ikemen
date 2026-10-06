//go:build desktop || mugen

package main

// ARM evaluates exact coverage/UV sampling into a compact indexed image. Each
// covered row segment is one hardware command, independent of color changes.
// Blending stays on FPGA, avoiding a whole-frame CPU replay for rotated sprites.
func (r *Renderer_SW) gpuGenericSpans(d *swRowDraw, first, last int) bool {
	stallStart := perfStallBegin()
	defer perfStallEnd(stallStart, "generic-indexed-sampling")
	q, g := d.q, r.gpu
	w, h := d.px1-d.px0+1, last-first+1
	if q == nil || q.fontMode || q.isFlat || q.pal == nil || q.tex == nil ||
		q.tex.depth != 8 || q.tex.width <= 0 || q.tex.height <= 0 ||
		int64(len(q.tex.data)) < int64(q.tex.width)*int64(q.tex.height) || len(d.tab) != 2048 ||
		w < 1 || w > 320 || h < 1 || h > 240 {
		return false
	}
	// Immutable original samples are only needed if a later draw forces replay.
	if g.textures == nil {
		g.textures = make(map[swGPUTextureKey]*swGPUTexture)
	}
	original := g.texture(q.tex, false)
	if original == nil {
		return false
	}
	palette, ok := g.palette(d.tab)
	if !ok {
		return false
	}
	aligned := (w*h + 63) &^ 63
	if uint64(g.next)+uint64(aligned) > 24*1024*1024 {
		return false
	}
	image := &swGPUTexture{address: 0x38400000 + g.next, data: make([]byte, w*h)}
	start := len(g.commands)
	var c [16]uint32
	c[0] = 1 | uint32(d.mode)<<8
	c[3], c[4], c[5], c[8], c[10] = image.address, uint32(w), uint32(h), 1<<16, palette
	emit := func() bool {
		if c[2] == 0 {
			return true
		}
		if len(g.commands)+64 > 2*1024*1024 {
			return false
		}
		g.command(c)
		c[2] = 0
		return true
	}
	for py := first; py <= last; py++ {
		c[7] = uint32(py-first) << 16
		pY := float32(r.h) - float32(py) - 0.5
		for px := d.px0; px <= d.px1; px++ {
			pX := float32(px) + 0.5
			W1, W2, det := baryWeights(pX, pY, d.v0, d.v1, d.v2)
			W0 := det - W1 - W2
			a, b, z := d.v0, d.v1, d.v2
			inside := triInside(W0, W1, W2, det, true)
			if !inside {
				W1, W2, det = baryWeights(pX, pY, d.v1, d.v2, d.v3)
				W0 = det - W1 - W2
				a, b, z = d.v1, d.v2, d.v3
				inside = triInside(W0, W1, W2, det, false)
			}
			if !inside {
				if !emit() {
					g.commands = g.commands[:start]
					return false
				}
				continue
			}
			inv := 1 / det
			u := a.u + (W1*inv)*(b.u-a.u) + (W2*inv)*(z.u-a.u)
			v := a.v + (W1*inv)*(b.v-a.v) + (W2*inv)*(z.v-a.v)
			if q.isTrapez {
				b0 := q.x1x2x4x3[2] + v*(q.x1x2x4x3[0]-q.x1x2x4x3[2])
				b1 := q.x1x2x4x3[3] + v*(q.x1x2x4x3[1]-q.x1x2x4x3[3])
				gap := b1 - b0
				if gap > 0.0001 || gap < -0.0001 {
					u = (pX - b0) / gap
				} else {
					u = 0.5
				}
			}
			image.data[(py-first)*w+px-d.px0] = byte(q.tex.sampleIndex(u, v))
			if c[2] == 0 {
				c[1] = uint32(py*320 + px)
				c[6] = uint32(px-d.px0) << 16
			}
			c[2]++
		}
		if !emit() {
			g.commands = g.commands[:start]
			return false
		}
	}
	// Upload after the prior GPU frame completes, using the existing arena and
	// fence. Transient images are not retained in the persistent texture cache.
	g.uploads = append(g.uploads, image)
	g.next += uint32(aligned)
	var saved *swGPUReplay
	if len(g.replay) < cap(g.replay) {
		saved = g.replay[:cap(g.replay)][len(g.replay)]
	}
	if saved == nil {
		saved = &swGPUReplay{}
	}
	saved.draw, saved.first, saved.last = *d, first, last
	saved.state = *q
	saved.texture = *q.tex
	saved.texture.data = original.data
	saved.state.tex = &saved.texture
	saved.draw.q = &saved.state
	offset := int(palette - 0x38200000)
	saved.draw.tab = g.palettes[offset : offset+2048]
	g.replay = append(g.replay, saved)
	return true
}
