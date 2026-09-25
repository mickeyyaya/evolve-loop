package core_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestSealCycle_SymlinkedStateLocksCanonicalTarget is the cycle-1690 pin for
// the linkGuardDeps topology: a worktree's .evolve/state.json is a link to the
// canonical state file. SealCycle must take the "<canonical>.lock" sidecar
// every canonical-path writer (statemap.UpdateStateMap, storage.UpdateState)
// contends on — never a sidecar beside the link — and its failurelog.Record +
// RMW must write THROUGH the link, leaving it intact (the cycle-999 sever).
func TestSealCycle_SymlinkedStateLocksCanonicalTarget(t *testing.T) {
	// Under a live fleet lane this names the RUNNING cycle's state, which
	// SealCycle would otherwise seal and delete.
	t.Setenv("EVOLVE_CYCLE_STATE_FILE", "")

	canonDir := t.TempDir()
	concurrencySealFixture(t, canonDir, 42) // canonical state.json{lastCycleNumber:41}
	canonical := filepath.Join(canonDir, "state.json")

	ev := t.TempDir()
	concurrencySealFixture(t, ev, 42)
	link := filepath.Join(ev, "state.json")
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(canonical, link); err != nil {
		t.Fatal(err)
	}

	if _, err := core.SealCycle(context.Background(), noopLedger{}, core.SealOptions{
		EvolveDir:   ev,
		ProjectRoot: ev,
		Reason:      "symlinked state regression test",
		Now:         func() time.Time { return time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC) },
		GitHead:     func(string) (string, error) { return "testhead1690", nil },
	}); err != nil {
		t.Fatalf("SealCycle through a linked state.json: %v", err)
	}

	if _, err := os.Lstat(canonical + ".lock"); err != nil {
		t.Errorf("SealCycle did not lock the canonical sidecar %s.lock: %v", canonical, err)
	}
	if _, err := os.Lstat(link + ".lock"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("SealCycle locked beside the link (%s.lock, err=%v) — no canonical-path writer contends there", link, err)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("state.json link was severed into a regular file (err=%v) — the cycle-999 defect", err)
	}
	final := readJSON(t, canonical)
	if got := intField(final, "lastCycleNumber"); got != 42 {
		t.Errorf("canonical lastCycleNumber=%d, want 42", got)
	}
	fa, _ := final["failedApproaches"].([]any)
	if len(fa) != 1 {
		t.Errorf("canonical failedApproaches=%v, want the one operator-reset record", final["failedApproaches"])
	}
}
