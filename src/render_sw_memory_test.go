//go:build desktop || mugen

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSWLazySpriteEvictionAndLinks(t *testing.T) {
	r := newSWTestRenderer(2, 1)
	path := filepath.Join(t.TempDir(), "pixels.bin")
	if err := os.WriteFile(path, []byte{7, 9}, 0600); err != nil {
		t.Fatal(err)
	}
	s := &Sprite{Size: [2]uint16{2, 1}, coldepth: 8}
	r.deferSprite(s, path, 0, 2)
	linked := &Sprite{Size: s.Size, lazyTexture: s.lazyTexture}
	if r.spriteCacheBytes != 0 || s.isBlank() {
		t.Fatal("sprite must stay lazy and drawable")
	}
	s.ensureTex()
	linked.ensureTex()
	if s.Tex != linked.Tex || r.spriteCacheBytes != 2 {
		t.Fatal("linked sprites must share storage")
	}
	if got := s.Tex.(*swTexture).data; len(got) != 2 || got[1] != 9 {
		t.Fatal(got)
	}
	r.trimSpriteCache(0)
	if r.spriteCacheBytes != 0 || len(s.Tex.(*swTexture).data) != 0 {
		t.Fatal("not evicted")
	}
	linked.ensureTex()
	s.ensureTex()
	if s.Tex != linked.Tex || s.Tex.(*swTexture).data[0] != 7 {
		t.Fatal("reload failed")
	}
	s.Tex.Release()
	s.ensureTex()
	if r.spriteCacheBytes != 2 {
		t.Fatal("release/reload accounting", r.spriteCacheBytes)
	}
	r.trimSpriteCache(0)
}

func TestSWImmutableSpriteUpload(t *testing.T) {
	r := newSWTestRenderer(2, 1)
	pixels := []byte{4, 5}
	x := r.newSpriteTexture(pixels, 2, 1, 8, false).(*swTexture)
	if &x.data[0] != &pixels[0] {
		t.Fatal("unnecessary sprite copy")
	}
	x.SetSubData([]byte{8}, 0, 0, 1, 1, 1)
	if pixels[0] != 4 || x.data[0] != 8 {
		t.Fatal("shared sprite was mutated")
	}
}
