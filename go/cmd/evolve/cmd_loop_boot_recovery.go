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
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
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
	HaltSelfSHA bool // a WITHIN-version SHA mismatch — boot must HALT pre-scout (not auto-repinned)
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
	if mismatch, _ := detectShipSHAMismatch(cfg, stderr); mismatch {
		res.SHAMismatch = true
		// Classify the mismatch the SAME way the terminal ship gate does
		// (verifySelfSHA, internal/phases/ship/verify.go): a WITHIN-version change
		// (expected_ship_version present AND == the current plugin version, SHA
		// differs) is SELF_SHA_TAMPERED — real tampering/install corruption, NOT a
		// legit rebuild (a legit rebuild is version-bumped, or healed by the
		// post-build repin). Such a ship is doomed from boot, so HALT pre-scout with
		// the operator-unblock recipe instead of burning a full ~32-40 min lane on it
		// (8 cycles wasted, 625-634). Deliberately do NOT auto-repin — repinning a
		// tampered pin away is exactly the anti-tamper bypass the ship gate forbids.
		if withinVersionShipSHAMismatch(cfg) {
			res.HaltSelfSHA = true
			printSelfSHAHaltRecipe(cfg, stderr)
			return res
		}
		// Across-version / legacy-unversioned mismatch (a legit plugin/version bump):
		// auto-heal the 508-513 cascade. A provenance-verified binary (its build-commit
		// is an ancestor of HEAD) is re-pinned in place, the unattended-boot successor
		// to `evolve reset-sha`, so the ship gate stops falsely blocking every cycle.
		// NEVER operatorAuthorized from an unattended boot: an unverifiable binary
		// (possible tampering) is refused and stays flagged (res.SHAMismatch).
		if attemptBootRepin(cfg, stderr) {
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

// withinVersionShipSHAMismatch reports whether the detected ship-SHA mismatch is
// WITHIN the current plugin version — state.json:expected_ship_version is present
// AND equals ship.PluginVersion(ProjectRoot), the SAME resolver verifySelfSHA uses.
// That is the ship gate's SELF_SHA_TAMPERED case (tampering/corruption). An empty
// expected_ship_version (legacy pin) or a differing version (legit bump) is NOT
// within-version and stays on the auto-repin path. A missing/unreadable state.json
// yields false (nothing to classify).
func withinVersionShipSHAMismatch(cfg loopConfig) bool {
	raw, err := os.ReadFile(filepath.Join(cfg.EvolveDir, "state.json"))
	if err != nil {
		return false
	}
	var st map[string]any
	if json.Unmarshal(raw, &st) != nil {
		return false
	}
	expectedVer, _ := st["expected_ship_version"].(string)
	if expectedVer == "" {
		return false
	}
	return expectedVer == ship.PluginVersion(cfg.ProjectRoot)
}

// printSelfSHAHaltRecipe writes the operator-unblock recipe for a within-version
// self-SHA halt so a human can act without hunting for it (the 8-cycle waste was
// partly not-knowing-what-to-do). Mirrors the per-phase-integrity self-heal recipe.
func printSelfSHAHaltRecipe(cfg loopConfig, stderr io.Writer) {
	fmt.Fprintf(stderr, "[loop] boot-recovery: ship binary was modified WITHIN plugin version %q "+
		"(expected_ship_sha != on-disk go/bin/evolve) — SELF_SHA_TAMPERED. HALTING pre-scout; "+
		"no cycle will run on a ship doomed from boot.\n", ship.PluginVersion(cfg.ProjectRoot))
	fmt.Fprintln(stderr, "[loop]   To unblock (rebuild from committed source, then re-authorize the pin):")
	fmt.Fprintln(stderr, "[loop]     1. make -C go build")
	fmt.Fprintln(stderr, "[loop]     2. evolve reset-sha -operator")
	fmt.Fprintln(stderr, "[loop]     3. relaunch the loop")
	fmt.Fprintln(stderr, "[loop]   (If this is NOT expected, investigate local tampering / plugin install corruption before re-pinning.)")
}

// attemptBootRepin re-pins expected_ship_sha to the on-disk ship binary via the
// shared, provenance-gated phaseintegrity.RepinIfDrifted — the SAME primitive the
// post-build repin (core.repinShipSHAAfterBuild) uses, so boot and post-build can
// never diverge (cycle 636, "never duplicate, centralize"). operatorAuthorized is
// always false here: an unattended boot must never let a tampered binary bypass
// the anti-tamper gate, so the re-pin fires only on verified provenance. Returns
// true iff the re-pin fired. Fail-open: a refusal/error WARNs and returns false,
// leaving the mismatch flagged.
func attemptBootRepin(cfg loopConfig, stderr io.Writer) bool {
	commit, prov := shipRepinProvenanceFn(cfg.ProjectRoot)
	statePath := filepath.Join(cfg.EvolveDir, "state.json")
	binPath := filepath.Join(cfg.ProjectRoot, "go", "bin", "evolve")
	res, err := phaseintegrity.RepinIfDrifted(statePath, binPath, commit, "", prov)
	if err != nil {
		fmt.Fprintf(stderr, "[loop] boot-recovery: ship-SHA auto-repin declined (%v) — rebuild from committed source then `evolve reset-sha` to authorize, or investigate tampering\n", err)
		return false
	}
	if !res.Repinned {
		return false // no drift after all (nothing to heal) — leave the flag as-is
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
