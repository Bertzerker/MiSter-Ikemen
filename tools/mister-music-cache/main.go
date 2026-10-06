// Convert Ogg/MP3 music into an optional MiSTer disk cache; never edit sources.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ikemen-engine/Ikemen-GO/internal/mistermusic"
)

func main() {
	game := flag.String("game", "", "game folder containing original music")
	cache := flag.String("cache", "", "separate output directory")
	flag.Parse()
	if *game == "" || *cache == "" {
		flag.Usage()
		os.Exit(2)
	}
	count := 0
	skipped := 0
	err := filepath.WalkDir(*game, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".ogg" && ext != ".mp3" {
			return nil
		}
		output, err := mistermusic.Convert(path, *cache)
		if err != nil {
			skipped++
			fmt.Fprintf(os.Stderr, "Skipped %s: %v\n", path, err)
			return nil
		}
		count++
		fmt.Printf("%s -> %s\n", path, output)
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Cached %d music files, skipped %d; originals unchanged.\n", count, skipped)
}
