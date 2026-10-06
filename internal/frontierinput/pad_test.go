package frontierinput

import "testing"

func TestPauseLatch(t *testing.T) {
	var p PauseLatch
	var pads [4]uint32
	pads[1] = (1 << 9) | (1 << 10)
	if !p.Update(pads) {
		t.Fatal("first press")
	}
	if p.Update(pads) {
		t.Fatal("held chord retriggered")
	}
	if p.Update([4]uint32{}) {
		t.Fatal("release triggered")
	}
	if !p.Update(pads) {
		t.Fatal("second press did not trigger")
	}
	p.Update([4]uint32{})
	if p.Update([4]uint32{1 << 9, 1 << 10}) {
		t.Fatal("buttons from separate players must not combine")
	}
}

func TestOSDMappedButtons(t *testing.T) {
	for bit, index := range map[uint]int{0: 14, 1: 13, 2: 12, 3: 11, 4: 0, 5: 1, 7: 2, 8: 3, 9: 10, 10: 6} {
		s := Decode(1 << bit)
		for i, b := range s.Buttons {
			if (b == 1) != (i == index) {
				t.Fatalf("bit %d, button %d: %d", bit, i, b)
			}
		}
	}
	if s := Decode(1 << 6); s.Axes[5] != 1 {
		t.Fatal("third attack must drive RT")
	}
	if s := Decode((1 << 9) | (1 << 10)); s.Buttons[4] != 1 || s.Buttons[6] != 0 || s.Buttons[10] != 0 {
		t.Fatal("pause chord must suppress taunt/attack")
	}
	if s := Decode(0); s != (State{}) {
		t.Fatal("release must clear all inputs")
	}
}
