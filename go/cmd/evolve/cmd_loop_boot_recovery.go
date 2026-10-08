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
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/pkg/version"
)

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
		return sysexec.Command(context.Background(), "git", "-C", projectRoot, "merge-base", "--is-ancestor", c, "HEAD").Run() == nil
	}
}

// defaultBootRecovery self-heals a dirty/stranded/tampered tree at boot so the
// first cycle's tree-diff guard runs against a clean baseline. Every step is
// fail-open: a failure WARNs to stderr and leaves that signal false.
func defaultBootRecovery(ctx context.Context, cfg loopConfig, ledger core.Ledger, stderr io.Writer) bootRecoveryResult {
	var res bootRecoveryResult

	// Detect a ship-binary SHA mismatch first, before quarantine — quarantine
	// would otherwise stash an untracked ship binary out from under the SHA read.
	if mismatch, _ := detectShipSHAMismatch(cfg, stderr); mismatch {
		res.SHAMismatch = true
		if withinVersionShipSHAMismatch(cfg) {
			res.HaltSelfSHA = true
			printSelfSHAHaltRecipe(cfg, stderr)
			return res
		}
		if attemptBootRepin(cfg, stderr) {
			res.Healed = true
			res.SHAMismatch = false // re-pinned in place; the ship gate now passes
		}
	}

	// Auto-seal a stranded cycle-state marker whose owner PID is dead, so a
	// crashed cycle's role-gate no longer blocks the next dispatch. Reuses
	// SealCycle(Force); ErrNothingToReset (no marker) is the common case.
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

	// Quarantine leaked tracked-source dirt last, after the SHA read, so it
	// never stashes the binary being verified.
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

// printSelfSHAHaltRecipe writes the operator-unblock recipe for a
// within-version self-SHA halt, mirroring the per-phase-integrity self-heal
// recipe.
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

// attemptBootRepin uses the same phaseintegrity.RepinIfDrifted primitive as
// the post-build repin (core.repinShipSHAAfterBuild), so boot and post-build
// can never diverge.
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
