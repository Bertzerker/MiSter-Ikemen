package mistermusic

import (
	"bytes"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/vorbis"
)

func TestMusicCachePreservesSamplesAndSeek(t *testing.T) {
	for _, name := range []string{"valid_44100hz_22050_samples.ogg", "valid_44100hz_x_padded_samples.mp3"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("../../packages/beep/internal/testdata", name)
			dir := t.TempDir()
			if _, err := Convert(path, dir); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			cached, format, ok := OpenCached(bytes.NewReader(data), dir)
			if !ok {
				t.Fatal("cache miss")
			}
			defer cached.Close()
			var original beep.StreamSeekCloser
			var originalFormat beep.Format
			source, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			if filepath.Ext(name) == ".ogg" {
				original, originalFormat, err = vorbis.Decode(source)
			} else {
				original, originalFormat, err = mp3.Decode(source)
			}
			if err != nil {
				t.Fatal(err)
			}
			defer original.Close()
			if format.SampleRate != originalFormat.SampleRate || cached.Len() != original.Len() {
				t.Fatal("sample rate/count changed")
			}
			for _, position := range []int{0, 1000, original.Len() - 13} {
				if err := original.Seek(position); err != nil {
					t.Fatal(err)
				}
				if err := cached.Seek(position); err != nil {
					t.Fatal(err)
				}
				var a, b [13][2]float64
				n, _ := original.Stream(a[:])
				m, _ := cached.Stream(b[:])
				if n != m {
					t.Fatal("seek length differs")
				}
				for i := 0; i < n; i++ {
					for c := 0; c < 2; c++ {
						x := math.Max(-1, math.Min(1-1.0/32768, a[i][c]))
						if math.Abs(x-b[i][c]) > 1.0/32768 {
							t.Fatalf("PCM differs at %d: %g vs %g", position+i, x, b[i][c])
						}
					}
				}
			}
			changed := bytes.NewReader(append(data, 0))
			if s, _, ok := OpenCached(changed, dir); ok {
				s.Close()
				t.Fatal("stale cache accepted")
			}
			if pos, _ := changed.Seek(0, io.SeekCurrent); pos != 0 {
				t.Fatal("cache miss did not rewind source")
			}
		})
	}
}
