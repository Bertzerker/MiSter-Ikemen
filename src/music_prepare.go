package main

import (
	"context"
	"io"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/vorbis"
	"github.com/ikemen-engine/Ikemen-GO/internal/mistermusic"
)

type preparedMusic struct {
	file   io.ReadSeekCloser
	stream beep.StreamSeekCloser
	format beep.Format
	cached bool
	err    error
}

func (p *preparedMusic) close() {
	if p.stream != nil {
		p.stream.Close()
	} else if p.file != nil {
		p.file.Close()
	}
}

// The worker owns the file/decoder until the main-thread callback accepts it.
// It never reads or mutates live BGM or game state.
func prepareMusic(filename, cacheDir string) *preparedMusic {
	start := perfStallBegin()
	defer perfStallEnd(start, "bgm-background-preparation")
	p := &preparedMusic{}
	p.file, p.err = OpenFile(filename)
	if p.err != nil {
		return p
	}
	p.stream, p.format, p.cached = mistermusic.OpenCached(p.file, cacheDir)
	if p.cached {
		p.file.Close()
		p.file = nil
	} else if HasExtension(filename, ".ogg") {
		p.stream, p.format, p.err = vorbis.Decode(p.file)
	} else {
		p.stream, p.format, p.err = mp3.Decode(p.file)
	}
	return p
}

var musicPreparationSlot = make(chan struct{}, 1)

func queuePreparedMusic(ctx context.Context, tasks chan func(), prepare func() *preparedMusic, accept func(*preparedMusic)) {
	select {
	case musicPreparationSlot <- struct{}{}:
	case <-ctx.Done():
		return
	}
	if ctx.Err() != nil {
		<-musicPreparationSlot
		return
	}
	p := prepare()
	<-musicPreparationSlot
	if ctx.Err() != nil {
		p.close()
		return
	}
	select {
	case tasks <- func() {
		if ctx.Err() != nil {
			p.close()
			return
		}
		accept(p)
	}:
	case <-ctx.Done():
		p.close()
	}
}
