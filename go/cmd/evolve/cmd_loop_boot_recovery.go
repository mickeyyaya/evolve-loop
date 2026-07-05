package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
	"github.com/mickeyyaya/evolve-loop/go/pkg/version"
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
	SHAMismatch bool // the ship binary's SHA != expected_ship_sha (still unhealed)
	Healed      bool // a provenance-verified SHA mismatch was auto-repinned at boot
}

// bootRecoverFn is the boot-recovery seam runLoop calls before the readiness
// gate. Overridable in tests (spy the call); production = defaultBootRecovery.
var bootRecoverFn = defaultBootRecovery

// shipRepinProvenanceFn resolves the running binary's build-commit and the
// provenance predicate used to authorize a boot-time auto-repin. A package-var
// seam (mirrors bootRecoverFn) so boot recovery stays git-free — hence
// deterministic — under test. Production = defaultShipRepinProvenance.
var shipRepinProvenanceFn = defaultShipRepinProvenance

// defaultShipRepinProvenance mirrors runResetSHA (cmd_resetsha.go): the running
// binary's embedded build-commit, plus a closure asserting that commit is an
// ancestor of HEAD (`git merge-base --is-ancestor`). An empty commit is
// unverifiable (returns false), so a stripped/tampered binary can never
// self-authorize a re-pin.
func defaultShipRepinProvenance(projectRoot string) (string, phaseintegrity.ProvenanceVerified) {
	return version.Commit(), func(c string) bool {
		if c == "" {
			return false
		}
		return exec.Command("git", "-C", projectRoot, "merge-base", "--is-ancestor", c, "HEAD").Run() == nil
	}
}

// defaultBootRecovery self-heals a dirty/stranded/tampered tree at boot so the
// first cycle's tree-diff guard runs against a clean baseline. Every step is
// fail-open: a failure WARNs to stderr and leaves that signal false.
func defaultBootRecovery(ctx context.Context, cfg loopConfig, ledger core.Ledger, stderr io.Writer) bootRecoveryResult {
	var res bootRecoveryResult

	// 1. Detect a ship-binary SHA mismatch FIRST — before quarantine, which would
	//    otherwise stash an untracked ship binary out from under the SHA read (the
	//    498/500/502 SELF_SHA_TAMPERED cascade, caught at boot).
	if mismatch, actual := detectShipSHAMismatch(cfg, stderr); mismatch {
		res.SHAMismatch = true
		// Auto-heal the 508-513 cascade: a legitimately-rebuilt binary (its
		// build-commit is an ancestor of HEAD — provenance-verified) is re-pinned
		// in place, the unattended-boot successor to `evolve reset-sha`, so the
		// ship gate stops falsely blocking every cycle. NEVER operatorAuthorized
		// from an unattended boot: an unverifiable binary (possible tampering) is
		// refused and stays flagged (res.SHAMismatch) so the gate still blocks.
		if attemptBootRepin(cfg, actual, stderr) {
			res.Healed = true
			res.SHAMismatch = false // re-pinned in place; the ship gate now passes
		}
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
	//    binary being verified. The label keeps the recognisable "boot-quarantine"
	//    prefix but carries a UTC RFC3339 timestamp so successive boot quarantines
	//    stay individually identifiable/recoverable (an operator can tell which
	//    stash came from which boot instead of every quarantine collapsing under one
	//    ambiguous fixed name).
	quarantineLabel := fmt.Sprintf("boot-quarantine-%s", time.Now().UTC().Format(time.RFC3339))
	if stashed, err := core.QuarantineDirtyTree(ctx, cfg.ProjectRoot, quarantineLabel); err != nil {
		fmt.Fprintf(stderr, "[loop] boot-recovery: quarantine: %v\n", err)
	} else if stashed {
		res.Quarantined = true
		fmt.Fprintf(stderr, "[loop] boot-recovery: quarantined leaked tracked-source dirt into a git stash (recover with: git stash pop)\n")
	}

	return res
}

// detectShipSHAMismatch compares the on-disk ship binary against
// state.json:expected_ship_sha, returning (mismatch, on-disk-sha). Absent state
// / binary / expectation ⇒ nothing to check (false, "", never a panic). It
// short-circuits before touching the binary when no pin exists, so a fresh
// project reaches neither the hash nor the downstream provenance/git path.
func detectShipSHAMismatch(cfg loopConfig, stderr io.Writer) (bool, string) {
	raw, err := os.ReadFile(filepath.Join(cfg.EvolveDir, "state.json"))
	if err != nil {
		return false, ""
	}
	var st map[string]any
	if json.Unmarshal(raw, &st) != nil {
		return false, ""
	}
	expected, _ := st["expected_ship_sha"].(string)
	if expected == "" {
		return false, ""
	}
	binPath := filepath.Join(cfg.ProjectRoot, "go", "bin", "evolve")
	mismatch, actual, err := core.ShipSHAMismatch(binPath, expected)
	if err != nil {
		return false, "" // no binary to compare ⇒ not a mismatch signal
	}
	if mismatch {
		fmt.Fprintf(stderr, "[loop] boot-recovery: ship binary SHA mismatch (expected %s, on-disk %s) — attempting provenance-gated auto-repin\n", expected, actual)
	}
	return mismatch, actual
}

// attemptBootRepin re-pins expected_ship_sha to the on-disk ship binary via the
// provenance-gated phaseintegrity.RepinShipSHA — the exact primitive `evolve
// reset-sha` uses. operatorAuthorized is always false here: an unattended boot
// must never let a tampered binary bypass the anti-tamper gate, so the re-pin
// fires only on verified provenance. Returns true iff the re-pin fired. Fail-open:
// a refusal/error WARNs and returns false, leaving the mismatch flagged.
func attemptBootRepin(cfg loopConfig, actualSHA string, stderr io.Writer) bool {
	commit, prov := shipRepinProvenanceFn(cfg.ProjectRoot)
	statePath := filepath.Join(cfg.EvolveDir, "state.json")
	res, err := phaseintegrity.RepinShipSHA(statePath, actualSHA, commit, "", prov, false)
	if err != nil {
		fmt.Fprintf(stderr, "[loop] boot-recovery: ship-SHA auto-repin declined (%v) — rebuild from committed source then `evolve reset-sha` to authorize, or investigate tampering\n", err)
		return false
	}
	fmt.Fprintf(stderr, "[loop] boot-recovery: auto-repinned expected_ship_sha %.12s -> %.12s (authorized: %s) — legitimate rebuild self-healed at boot\n", res.OldSHA, res.NewSHA, res.Authorized)
	return true
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
