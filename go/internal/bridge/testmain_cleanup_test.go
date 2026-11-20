package bridge

import "sync"

var testMainCleanups struct {
	mu  sync.Mutex
	fns []func()
}

func AddTestMainCleanup(fn func()) {
	testMainCleanups.mu.Lock()
	defer testMainCleanups.mu.Unlock()
	testMainCleanups.fns = append(testMainCleanups.fns, fn)
}

func RunTestMainCleanups() {
	testMainCleanups.mu.Lock()
	fns := testMainCleanups.fns
	testMainCleanups.fns = nil
	testMainCleanups.mu.Unlock()
	for _, fn := range fns {
		fn()
	}
}
