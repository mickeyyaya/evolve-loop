package registry

import (
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// TestSnapshotForTest verifies snapshot captures the current set and restore
// re-establishes exactly it (discarding intervening registrations).
func TestSnapshotForTest(t *testing.T) {
	// Restore the real registry state at test end so we don't disturb peers.
	defer SnapshotForTest()()

	ResetForTesting()
	restore := SnapshotForTest() // snapshot of the empty registry
	Register("snaptest", func(core.PhaseRequest) core.PhaseRunner { return nil })
	if _, ok := For("snaptest"); !ok {
		t.Fatal("snaptest should be registered before restore")
	}

	restore()
	if _, ok := For("snaptest"); ok {
		t.Error("SnapshotForTest restore did not discard the intervening registration")
	}
}
