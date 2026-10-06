//go:build debug

package main

import (
	"fmt"
	"os"
	"time"
)

// Explicit diagnostic opt-in: no per-frame output in ordinary debug builds.
var perfStallEnabled = os.Getenv("IKEMEN_STALL_LOG") == "1"

var stallLastDraw time.Time
var stallSkipped int

// Called on the main thread at presentation, including deliberately skipped
// renders. Distinguish one blocked operation from a catch-up run of many ticks.
func perfStallPacing(skipped bool) {
	if !perfStallEnabled {
		return
	}
	if skipped {
		stallSkipped++
		return
	}
	now := time.Now()
	if !stallLastDraw.IsZero() && now.Sub(stallLastDraw) >= 100*time.Millisecond {
		fmt.Fprintf(os.Stderr, "[PACING] start_unix_ms=%d draw_gap_ms=%.2f skipped=%d\n", stallLastDraw.UnixMilli(), float64(now.Sub(stallLastDraw))/float64(time.Millisecond), stallSkipped)
	}
	stallLastDraw, stallSkipped = now, 0
}

func perfStallBegin() time.Time {
	if perfStallEnabled {
		return time.Now()
	}
	return time.Time{}
}

func perfStallEnd(t time.Time, phase string) {
	if !perfStallEnabled || t.IsZero() {
		return
	}
	elapsed := time.Since(t)
	if elapsed >= 50*time.Millisecond {
		fmt.Fprintf(os.Stderr, "[STALL] start_unix_ms=%d phase=%s elapsed_ms=%.2f\n", t.UnixMilli(), phase, float64(elapsed)/float64(time.Millisecond))
	}
}
