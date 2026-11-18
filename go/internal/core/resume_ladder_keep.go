package core

import "sync"

type resumedLadderKey struct {
	o     *Orchestrator
	cycle int
}

type resumedLadderEntry struct {
	ladder  LadderState
	adopter *cycleRun
}

var resumedLadderStore struct {
	mu   sync.Mutex
	kept map[resumedLadderKey]*resumedLadderEntry
}

func keepResumedLadder(o *Orchestrator, cycle int, st *LadderState) {
	if st == nil {
		return
	}
	resumedLadderStore.mu.Lock()
	defer resumedLadderStore.mu.Unlock()
	if resumedLadderStore.kept == nil {
		resumedLadderStore.kept = map[resumedLadderKey]*resumedLadderEntry{}
	}
	resumedLadderStore.kept[resumedLadderKey{o, cycle}] = &resumedLadderEntry{ladder: *st}
}

func (cr *cycleRun) adoptResumedLadder() {
	resumedLadderStore.mu.Lock()
	defer resumedLadderStore.mu.Unlock()
	entry := resumedLadderStore.kept[resumedLadderKey{cr.o, cr.cycle}]
	if entry == nil || entry.adopter == cr {
		return
	}
	entry.adopter = cr
	adopted := entry.ladder
	cr.cs.Ladder = &adopted
}
