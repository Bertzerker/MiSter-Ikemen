//go:build desktop || mugen

package main

import "container/list"

type swCachedSprite struct {
	texture  *swTexture
	bytes    int
	resident bool
}

// newSpriteTexture adopts immutable decoded pixels. Other texture upload APIs
// retain their copy semantics, including copy-on-write for linked sprite data.
func (r *Renderer_SW) newSpriteTexture(data []byte, w, h, depth int32, filter bool) Texture {
	textureSerialNumber++
	return &swTexture{width: w, height: h, depth: depth, filter: filter,
		data: data, serial: textureSerialNumber, immutable: true}
}

// SFFv2 records have independent palettes and absolute data offsets. Retaining
// their headers lets menus use a few backgrounds without decoding whole packs.
// SFFv1 keeps the normal loader because its palettes depend on preceding records.
func (r *Renderer_SW) deferSprite(s *Sprite, filename string, offset int64, size uint32) {
	dimensions, depth, compression := s.Size, s.coldepth, s.rle
	var texture *swTexture
	var entry *list.Element
	failed := false
	s.lazyTexture = func() Texture {
		if failed {
			return nil
		}
		if texture != nil && len(texture.data) != 0 && !texture.released {
			r.spriteCache.MoveToBack(entry)
			return texture
		}
		stallStart := perfStallBegin()
		defer perfStallEnd(stallStart, "sprite-decode:"+filename)
		f, err := OpenFile(filename)
		if err != nil {
			LogMessage("[SFF] %s: %v", filename, err)
			failed = true
			return nil
		}
		decoded := &Sprite{Size: dimensions, coldepth: depth, rle: compression, noEagerUpload: true}
		err = decoded.readV2(f, offset, size)
		f.Close()
		if err != nil || len(decoded.pendingData) == 0 {
			LogMessage("[SFF] Unable to decode %s at %d: %v", filename, offset, err)
			failed = true
			return nil
		}
		texture = r.newSpriteTexture(decoded.pendingData, decoded.pendingW,
			decoded.pendingH, decoded.pendingDepth, decoded.pendingFilter).(*swTexture)
		// Release may have cleared a still-cached texture before this reload.
		if entry != nil && entry.Value.(*swCachedSprite).resident {
			r.spriteCacheBytes -= entry.Value.(*swCachedSprite).bytes
			r.spriteCache.Remove(entry)
		}
		entry = r.spriteCache.PushBack(&swCachedSprite{texture, len(texture.data), true})
		r.spriteCacheBytes += len(texture.data)
		return texture
	}
}
