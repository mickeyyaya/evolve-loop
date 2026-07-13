// gitops_binarychurn_test.go — RED contract for inbox defect
// ship-manual-deletes-running-binary (2026-07-13T08-45Z; live incidents
// 2026-07-12 19:5x and 23:53, cycle-243 precedent).
//
// Root cause: `evolve ship --class manual` routes through shipDirect, which
// calls discardBinaryChurn on the SUCCESS path before `git add -A`.
// cmd_ship.go never sets Options.ShipBinaryPath, so the churn discard falls
// back to os.Executable() — the running go/bin/evolve. That path is UNTRACKED
// (go/.gitignore `/bin/`), so the discard loop hits os.Remove and deletes the
// binary every PreToolUse kernel hook resolves first. `git add -A` can never
// stage a gitignored file, so the removal had zero staging-hygiene value —
// pure harm. Rollback shells `evolve ship --class manual`
// (rollback.defaultRevertAndShip), so the deletion fired there transitively;
// the same shipDirect guard covers that path.
//
// Contract pinned here:
//  1. discardBinaryChurn must NEVER remove the currently-executing binary
//     (skip + WARN), regardless of how its path was resolved.
//  2. Manual-class shipDirect success leaves an untracked go/bin/evolve on
//     disk while KEEPING the untracked-go/evolve removal (the discard's actual
//     purpose — see TestShipDirect_CycleClass_KeepsChurnDiscardAndAddAll).
package ship

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubRunningExecutable points the osExecutable seam at path for the duration
// of the test, mirroring the production fallback where the running binary IS
// the discard candidate.
func stubRunningExecutable(t *testing.T, path string) {
	t.Helper()
	old := osExecutable
	osExecutable = func() (string, error) { return path, nil }
	t.Cleanup(func() { osExecutable = old })
}

// TestDiscardBinaryChurn_NeverRemovesRunningExecutable — RED today: with
// ShipBinaryPath empty (exactly what cmd_ship.go passes), the fallback
// resolves the running executable, ls-files reports it untracked, and the
// discard loop os.Remove()s it.
func TestDiscardBinaryChurn_NeverRemovesRunningExecutable(t *testing.T) {
	root := t.TempDir()
	runningBin := filepath.Join(root, "go", "bin", "evolve")
	mustWrite(t, runningBin, "running-binary\n")
	stubRunningExecutable(t, runningBin)

	cap := &stagingCapture{} // every ls-files → exit 0, empty stdout = untracked
	var warn strings.Builder
	opts := &Options{
		ProjectRoot:    root,
		Runner:         cap.runner(),
		Stderr:         &warn,
		ShipBinaryPath: "", // cmd_ship.go:79-90 never sets it — the incident path
	}

	if err := discardBinaryChurn(context.Background(), opts, root); err != nil {
		t.Fatalf("discardBinaryChurn: %v", err)
	}
	if _, err := os.Stat(runningBin); err != nil {
		t.Fatalf("RED (2026-07-12 incidents): discardBinaryChurn deleted the currently-executing binary %s: %v — every PreToolUse kernel hook degrades to the stale tracked fallback until rebuild", runningBin, err)
	}
	if !strings.Contains(warn.String(), "currently-executing") {
		t.Errorf("skip must WARN loudly (fail-loudly rule); stderr: %q", warn.String())
	}
}

// TestManualShipSuccess_LeavesUntrackedGoBinEvolvePresent — the incident
// end-to-end at the shipDirect level (the exact function the rollback
// shellout's `evolve ship --class manual` re-enters): a successful manual
// ship keeps the running go/bin/evolve, while the untracked tracked-path
// go/evolve churn is still discarded (guard is narrow, not a behavior revert).
func TestManualShipSuccess_LeavesUntrackedGoBinEvolvePresent(t *testing.T) {
	root := t.TempDir()
	runningBin := filepath.Join(root, "go", "bin", "evolve")
	mustWrite(t, runningBin, "running-binary\n")
	mustWrite(t, filepath.Join(root, "go", "evolve"), "post-audit churn\n")
	stubRunningExecutable(t, runningBin)

	cap := &stagingCapture{}
	opts := &Options{
		Class:          ClassManual,
		ProjectRoot:    root,
		PluginRoot:     root,
		CommitMessage:  "manual: operator commit",
		Runner:         cap.runner(),
		Stderr:         io.Discard,
		ShipBinaryPath: "", // CLI parity: manual ships resolve via os.Executable
	}

	if err := shipDirect(context.Background(), opts, &RunResult{}, "main"); err != nil {
		t.Fatalf("shipDirect(manual): %v", err)
	}
	if _, err := os.Stat(runningBin); err != nil {
		t.Errorf("RED: successful manual ship deleted the running go/bin/evolve: %v", err)
	}
	// Discriminator: the churn discard still removed the untracked go/evolve —
	// the guard protects ONLY the running executable.
	if _, err := os.Stat(filepath.Join(root, "go", "evolve")); !os.IsNotExist(err) {
		t.Error("guard over-reached: untracked go/evolve churn was NOT discarded (unaudited binaries would ride into commits)")
	}
}
