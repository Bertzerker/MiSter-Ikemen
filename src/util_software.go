//go:build softwareonly

package main

import (
	"fmt"
	findfont "github.com/flopp/go-findfont"
	"os"
)

func ShowInfoDialog(message, title string) { fmt.Fprintln(os.Stderr, title+": "+message) }
func ShowErrorDialog(message string)       { fmt.Fprintln(os.Stderr, message) }

// TTF font loading
func LoadFntTtf(f *Fnt, fontfile string, filename string, height int32) {
	// Search in local directory
	fileDir := SearchFile(filename, []string{fontfile, sys.motif.Def, "", "data/"}, "font/")
	// Search in system directory
	fp := fileDir
	if fp = FileExist(fp); len(fp) == 0 {
		var err error
		fileDir, err = findfont.Find(fileDir)
		if err != nil {
			panic(fmt.Errorf("failed to find ttf font %v: %w", fileDir, err))
		}
	}
	// Load ttf
	if height == -1 {
		height = int32(f.Size[1])
	} else {
		f.Size[1] = uint16(height)
	}
	ttf, err := gfxFont.LoadFont(fileDir, height, int(sys.gameWidth), int(sys.gameHeight))
	if err != nil {
		panic(fmt.Errorf("failed to load ttf font %v: %w", fileDir, err))
	}
	f.ttf = ttf.(Font)

	// Create Ttf dummy palettes
	f.palettes = make([][256]uint32, 1)
	for i := 0; i < 256; i++ {
		f.palettes[0][i] = 0
	}
}

func selectRenderer(cfgVal string) (Renderer, FontRenderer) {
	return &Renderer_SW{}, &FontRenderer_SW{}
}

func platformDefaultConfig(cfg *Config) { cfg.Video.RenderMode = "SDL2 Software" }

// The desktop dispatcher lives in render_gl33.go. Keep the software queue
// available when that GPU backend is excluded from the binary.
func flushSpriteQueueBatched(queue []SpriteDrawCall) {
	gfx.(*Renderer_SW).flushSWQueue(queue)
}
