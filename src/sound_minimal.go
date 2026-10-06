//go:build minimal && !mugen

package main

import (
	"github.com/gopxl/beep/v2"
	"io"
)

func xmpDecode(f io.ReadSeekCloser) (beep.StreamSeekCloser, beep.Format, error) {
	return nil, beep.Format{}, Error("module music is disabled in minimal builds")
}
