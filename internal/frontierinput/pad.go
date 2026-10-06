// Package frontierinput translates MiSTer logical buttons into SDL controller
// indices used by IKEMEN. No USB device is opened and OSD mapping stays authoritative.
package frontierinput

type State struct {
	Buttons [15]byte
	Axes    [6]float32
}

func PauseHeld(bits uint32) bool { return bits&((1<<10)|(1<<9)) == (1<<10)|(1<<9) }

// One toggle per press, even when held across many polling/render frames.
type PauseLatch struct{ held bool }

func (p *PauseLatch) Update(players [4]uint32) bool {
	held := false
	for _, bits := range players {
		held = held || PauseHeld(bits)
	}
	pressed := held && !p.held
	p.held = held
	return pressed
}

func Decode(bits uint32) (s State) {
	// MiSTer direction order is Right, Left, Down, Up.
	// Preserve the existing seven-button OSD definition:
	// Attack, Jump, Special, Attack2, Attack3, Attack4, Start.
	for _, mapping := range [][2]uint{{0, 14}, {1, 13}, {2, 12}, {3, 11},
		{4, 0}, {5, 1}, {7, 2}, {8, 3}, {9, 10}, {10, 6}} {
		if bits&(1<<mapping[0]) != 0 {
			s.Buttons[mapping[1]] = 1
		}
	}
	// The default IKEMEN six-button layout binds C to RT and Z to RB.
	if bits&(1<<6) != 0 {
		s.Axes[5] = 1
	}
	// Existing seven-button mappings have no spare Menu button. Start+Attack4
	// requests Menu (or lightweight pause); suppress taunt/Z while held.
	if PauseHeld(bits) {
		s.Buttons[4] = 1
		s.Buttons[6] = 0
		s.Buttons[10] = 0
	}
	return
}
