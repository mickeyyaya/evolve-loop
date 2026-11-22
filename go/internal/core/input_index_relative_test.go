package core

import "testing"

func TestCycleRecord_RefusesRelativeProjectRoot(t *testing.T) {
	t.Parallel()

	if _, err := cycleRecord(indexTarget{projectRoot: "relative", cycle: 1}, nil); err == nil {
		t.Fatal("cycleRecord(relative) = nil error, want an error")
	}
}
