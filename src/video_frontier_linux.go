//go:build softwareonly && linux

package main

import (
	"fmt"
	"github.com/ikemen-engine/Ikemen-GO/internal/frontierinput"
	"github.com/ikemen-engine/Ikemen-GO/internal/frontiervideo"
	"os"
)

func init() {
	if os.Getenv("IKEMEN_FRONTIER") == "1" {
		var previous [4]uint32
		trace := os.Getenv("IKEMEN_INPUT_TRACE") == "1"
		var pause frontierinput.PauseLatch
		nativeInputUpdate = func() {
			var players [4]uint32
			for i := range players {
				players[i] = frontiervideo.Buttons(i)
			}
			if pause.Update(players) && sys.gameRunning && !sys.sel.gameParams.PauseMenu {
				sys.paused = !sys.paused
				sys.frameStepFlag = false
				fmt.Fprintf(os.Stderr, "[Frontier] Pause: %t\n", sys.paused)
			}
		}
		nativePadState = func(player int) ([15]byte, [6]float32) {
			bits := frontiervideo.Buttons(player)
			if trace && player >= 0 && player < 4 && previous[player] != bits {
				fmt.Fprintf(os.Stderr, "[Frontier] Controller P%d: 0x%08x\n", player+1, bits)
				previous[player] = bits
			}
			state := frontierinput.Decode(bits)
			// Quick VS deliberately omits the graphical menu. Its chord uses
			// the lightweight pause toggle above, not a menu/exit command.
			if sys.gameRunning && !sys.sel.gameParams.PauseMenu {
				state.Buttons[4] = 0
			}
			return state.Buttons, state.Axes
		}
		nativeAudioFactory = func() AudioSink { return &FrontierSpeaker{} }
		swVideoSinkFactory = func(w, h int) (swVideoSink, error) {
			return frontiervideo.Open(w, h)
		}
	}
}
