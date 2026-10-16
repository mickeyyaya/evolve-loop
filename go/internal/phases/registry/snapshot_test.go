package registry

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestSnapshotForTest(t *testing.T) {
	defer SnapshotForTest()()

	ResetForTesting()
	restore := SnapshotForTest()
	Register("snaptest", func(core.PhaseRequest) core.PhaseRunner { return nil })
	if _, ok := For("snaptest"); !ok {
		t.Fatal("snaptest should be registered before restore")
	}

	restore()
	if _, ok := For("snaptest"); ok {
		t.Error("SnapshotForTest restore did not discard the intervening registration")
	}
}
