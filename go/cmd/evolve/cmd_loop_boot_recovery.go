package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// cmd_loop_boot_recovery.go — WIRING of the boot-time recovery primitives into
// runLoop's boot path (cycle 507, task wire-boot-recovery-functions). This is
// the layer cycle 506 was missing: it built the core primitives (fully unit
// tested) but never called them from runLoop (audit F1, CRITICAL — the project's
// own warnship_apicover_ci_gap "green unit test, absent integration" trap).
//
// Mirrors the established runLoopPreflightFn / wireOrchestratorDepsFn package-var
// seam idiom so tests can spy the invocation. Best-effort / fail-open: a recovery
// error WARNs but never halts the batch.

// bootRecoveryResult reports which boot-time self-heal actions fired.
type bootRecoveryResult struct {
	Quarantined bool // leaked tracked-source dirt was stashed
	Sealed      bool // a stranded dead-owner marker was auto-sealed
	SHAMismatch bool // the ship binary's SHA != expected_ship_sha
}

// bootRecoverFn is the boot-recovery seam runLoop calls before the readiness
// gate. Overridable in tests (spy the call); production = defaultBootRecovery.
var bootRecoverFn = defaultBootRecovery

// defaultBootRecovery self-heals a dirty/stranded/tampered tree at boot so the
// first cycle's tree-diff guard runs against a clean baseline. Every step is
// fail-open: a failure WARNs to stderr and leaves that signal false.
func defaultBootRecovery(ctx context.Context, cfg loopConfig, ledger core.Ledger, stderr io.Writer) bootRecoveryResult {
	var res bootRecoveryResult

	// 1. Detect a ship-binary SHA mismatch FIRST — before quarantine, which would
	//    otherwise stash an untracked ship binary out from under the SHA read (the
	//    498/500/502 SELF_SHA_TAMPERED cascade, caught at boot).
	if detectShipSHAMismatch(cfg, stderr) {
		res.SHAMismatch = true
	}

	// 2. Auto-seal a stranded cycle-state marker whose owner PID is dead, so a
	//    crashed cycle's role-gate no longer blocks the next dispatch. Reuses
	//    SealCycle(Force). ErrNothingToReset (no marker) is the common case.
	if _, sealed, err := core.AutosealStaleMarker(ctx, ledger, core.SealOptions{
		EvolveDir:   cfg.EvolveDir,
		ProjectRoot: cfg.ProjectRoot,
		Reason:      "boot auto-seal: stranded cycle-state marker, owner PID dead",
	}, pidAlive); err != nil {
		if !errors.Is(err, core.ErrNothingToReset) {
			fmt.Fprintf(stderr, "[loop] boot-recovery: autoseal: %v\n", err)
		}
	} else if sealed {
		res.Sealed = true
		fmt.Fprintf(stderr, "[loop] boot-recovery: auto-sealed a stranded dead-owner cycle marker\n")
	}

	// 3. Quarantine leaked tracked-source dirt (non-destructive stash) LAST so the
	//    tree-diff guard doesn't attribute pre-existing dirt to this batch's first
	//    phase and wedge the loop. Runs after the SHA read so it never stashes the
	//    binary being verified.
	if stashed, err := core.QuarantineDirtyTree(ctx, cfg.ProjectRoot, "boot-quarantine"); err != nil {
		fmt.Fprintf(stderr, "[loop] boot-recovery: quarantine: %v\n", err)
	} else if stashed {
		res.Quarantined = true
		fmt.Fprintf(stderr, "[loop] boot-recovery: quarantined leaked tracked-source dirt into a git stash (recover with: git stash pop)\n")
	}

	return res
}

// detectShipSHAMismatch compares the on-disk ship binary against
// state.json:expected_ship_sha. Absent state / binary / expectation ⇒ nothing to
// check (false, never a panic).
func detectShipSHAMismatch(cfg loopConfig, stderr io.Writer) bool {
	raw, err := os.ReadFile(filepath.Join(cfg.EvolveDir, "state.json"))
	if err != nil {
		return false
	}
	var st map[string]any
	if json.Unmarshal(raw, &st) != nil {
		return false
	}
	expected, _ := st["expected_ship_sha"].(string)
	if expected == "" {
		return false
	}
	binPath := filepath.Join(cfg.ProjectRoot, "go", "bin", "evolve")
	mismatch, actual, err := core.ShipSHAMismatch(binPath, expected)
	if err != nil {
		return false // no binary to compare ⇒ not a mismatch signal
	}
	if mismatch {
		fmt.Fprintf(stderr, "[loop] boot-recovery: ship binary SHA mismatch (expected %s, on-disk %s) — rebuild + reset-sha before shipping\n", expected, actual)
	}
	return mismatch
}

// pidAlive reports whether a process is alive via kill -0 (signal 0) semantics.
// A recycled pid may read as alive; boot recovery only uses this to spare a
// genuinely-live cycle, so a false-positive is the safe direction (skip seal).
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
