package main

import (
	"math"
	"testing"
)

type stateTestController struct {
	calls  *int
	change bool
}

func (s stateTestController) Run(_ *Char, _ []int32) bool { *s.calls++; return s.change }

func TestStateBlockExecution(t *testing.T) {
	oldChar := sys.workingChar
	defer func() { sys.workingChar = oldChar }()
	c := &Char{hitStateChangeIdx: -1}
	calls := 0
	b := newStateBlock()
	b.persistentIndex = 0
	b.persistent = 3
	b.ctrls = []StateController{stateTestController{&calls, false}}
	ps := []int32{0}
	for i := 0; i < 7; i++ {
		if b.Run(c, ps) {
			t.Fatal("unexpected change")
		}
	}
	if calls != 3 || ps[0] != 3 {
		t.Fatalf("persistence: calls=%d counter=%d", calls, ps[0])
	}
	ps[0] = math.MaxInt32
	b.Run(c, ps)
	if calls != 3 || ps[0] != math.MaxInt32 {
		t.Fatal("run-once lock changed")
	}
	b.persistentIndex = -1
	c.hitPauseTime = 1
	b.Run(c, nil)
	if calls != 3 {
		t.Fatal("hitpause ignored")
	}
	b.ignorehitpause = -1
	b.ctrlsIgnorehitpause = true
	b.Run(c, nil)
	if calls != 4 {
		t.Fatal("ignorehitpause controller skipped")
	}
	c.hitPauseTime = 0
	b.trigger = BytecodeExp{OC_int8, 0}
	b.elseBlock = newStateBlock()
	b.elseBlock.ctrls = []StateController{stateTestController{&calls, true}}
	if !b.Run(c, nil) || calls != 5 {
		t.Fatal("else/change propagation")
	}
}

func TestStateLoopPrivateCounters(t *testing.T) {
	oldChar := sys.workingChar
	defer func() { sys.workingChar = oldChar }()
	c := &Char{hitStateChangeIdx: -1}
	calls := 0
	b := newStateBlock()
	b.loopBlock = true
	b.forLoop = true
	b.forExpression = [3]BytecodeExp{{OC_int8, 1}, {OC_int8, 3}, {OC_int8, 1}}
	b.ctrls = []StateController{stateTestController{&calls, false}}
	for i := 0; i < 2; i++ {
		b.Run(c, nil)
	}
	if calls != 6 || b.forBegin != 0 || b.forEnd != 0 || b.forIncrement != 0 {
		t.Fatalf("loop leaked: calls=%d block=%+v", calls, b)
	}
	snapshot := b.controllerSnapshot()
	b.persistent = 7
	if snapshot.persistent == 7 {
		t.Fatal("compiler snapshot aliases block")
	}
}

// Preserve the old per-call value-copy cost for an otherwise identical workload.
//
//go:noinline
func stateValueReference(b StateBlock, c *Char) bool { return b.run(c, nil) }

func BenchmarkStateBlockFalseTrigger(b *testing.B) {
	oldChar := sys.workingChar
	defer func() { sys.workingChar = oldChar }()
	c := &Char{hitStateChangeIdx: -1}
	block := newStateBlock()
	block.trigger = BytecodeExp{OC_int8, 0}
	b.Run("value", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			stateValueReference(*block, c)
		}
	})
	b.Run("pointer", func(b *testing.B) {
		var sc StateController = block
		for i := 0; i < b.N; i++ {
			sc.Run(c, nil)
		}
	})
}
