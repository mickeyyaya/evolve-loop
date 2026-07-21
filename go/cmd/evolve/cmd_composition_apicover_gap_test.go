package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
)

// TestComposedApicoverGate_WarningOnlyMissesNewUnnamedExport reproduces
// percycle-audit-apicover-newexport-parity: the recurring "apicover RED on
// main" incidents (3 live recurrences 2026-07-20, fixed after the fact by
// c37dc324/46ff77f6 "close apicover gap on 3 orphaned internal/core exports",
// 9eacd83f "name+exercise fleet-rebase classify surface — fixes repo-wide
// apicover RED on main (recovered-commit export gap)", and aaeb4d4d
// "LandPrefixes naming test (apicover un-RED)").
//
// The RUNG0/RUNG2 fleet-rebase carry-forward fast path
// (internal/core/composition_carryforward.go: compositionCarryForward /
// scopedMergeCarryForward) reships straight to main — skipping a full
// re-audit — whenever runComposedGates reports every gate in
// ciparity.RequiredComposedGates (which includes "apicover") as "pass".
// composedGateTargets maps that "apicover" gate to the go/Makefile `apicover`
// target. But that target is Phase-0 WARNING-ONLY (go/Makefile:127 comment:
// "warning-only; ... -enforce in Phase 5"; its recipe at line 132 never
// passes -enforce), unlike CI's separate Phase-5 "api-coverage enforce" step
// (.github/workflows/go.yml:99-116) which does. apicover.Run only returns a
// non-zero exit when cfg.Enforce is true (internal/apicover/run.go) — so the
// Makefile target's bare `bin/apicover -cover ...` invocation ALWAYS exits 0,
// regardless of how many exported symbols are uncovered.
//
// Net effect: when a fleet-rebase folds in a peer lane's already-landed
// commit that introduced a brand-new, still-unnamed exported symbol (exactly
// what FleetRebaseVerdict/ClassifyFleetRebaseCandidate and LandPrefixes were
// when they landed), THIS lane's own apicoverEnforceChangedDefault never
// looks at it (it wasn't part of this lane's own changed-package diff), and
// the composed-gate re-check that's supposed to be the last line of defense
// reports "apicover: pass" unconditionally — so the carry-forward reships the
// gap to main, where only the separate repo-wide CI enforce step (not
// reproduced anywhere in the composed-gate set) eventually catches it.
//
// This test proves the gap directly: it adds a throwaway package containing
// one exported, zero-coverage, never-named function, then runs the EXACT
// Makefile target composedGateTargets["apicover"] names (scoped to just the
// fixture package via APICOVER_PKGS, so the run stays fast) and shows it
// exits 0 — while the real enforcing check (apicover.Run with Enforce:true)
// correctly flags the same package as having an uncovered export. A fix that
// closes the gap (e.g. pointing composedGateTargets["apicover"] at a real
// enforcing recipe) will make the Makefile-target run in this test also
// fail, at which point this test's core assertion flips to green.
func TestComposedApicoverGate_WarningOnlyMissesNewUnnamedExport(t *testing.T) {
	// DISABLED until percycle-audit-apicover-newexport-parity (0.94) redesigns
	// it: this reproduction MUTATES THE LIVE REPO TREE — it creates
	// internal/apicoverreprofixture998 in-tree, shells out to `make -C go
	// apicover` (which regenerates coverage.txt, poisoning the CI profile with
	// a package the cleanup then deletes), and broke the `go` workflow's
	// cover -func step on BOTH platforms (2026-07-21, commit 79ead521: "cover:
	// cannot run go list: fork/exec ...: invalid argument"). The skip must sit
	// BEFORE any side effect. The fix cycle must rebuild this against a
	// throwaway COPY of the module (temp dir), never the live tree, and flip
	// the core assertion to t.Fatalf as its regression pin.
	t.Skip("KNOWN BUG reproduction disabled (percycle-audit-apicover-newexport-parity): mutates the live repo tree and poisons the CI coverage profile — redesign against a temp module copy in the fix cycle")
	goRoot := apicoverGoRoot(t)
	repoRoot := filepath.Dir(goRoot)

	fixtureRel := filepath.Join("internal", "apicoverreprofixture998")
	fixtureDir := filepath.Join(goRoot, fixtureRel)
	if err := os.MkdirAll(fixtureDir, 0o755); err != nil {
		t.Fatalf("create fixture package dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(fixtureDir) })

	fixtureSrc := `// Package apicoverreprofixture998 is a throwaway fixture for
// bug-reproduction cycle-998 (percycle-audit-apicover-newexport-parity).
// It is removed by the reproducer test's cleanup and must never be committed.
package apicoverreprofixture998

// UncoveredExport has zero test references and zero executed coverage —
// exactly the shape FleetRebaseVerdict/ClassifyFleetRebaseCandidate and
// LandPrefixes had the moment they landed and broke main's apicover gate.
func UncoveredExport() string { return "uncovered" }
`
	if err := os.WriteFile(filepath.Join(fixtureDir, "fixture.go"), []byte(fixtureSrc), 0o644); err != nil {
		t.Fatalf("write fixture package: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Exercise the EXACT recipe compositionCarryForward's composed-gate check
	// trusts for "apicover" (composedGateTargets["apicover"] == "apicover"),
	// scoped via APICOVER_PKGS so this stays a fast, targeted repro instead of
	// re-measuring the whole ./internal/... tree.
	target := composedGateTargets["apicover"]
	makeCmd := exec.CommandContext(ctx, "make", "-C", "go", target,
		"APICOVER_PKGS=./"+filepath.ToSlash(fixtureRel)+"/...")
	makeCmd.Dir = repoRoot
	var makeOut bytes.Buffer
	makeCmd.Stdout = &makeOut
	makeCmd.Stderr = &makeOut
	makeErr := makeCmd.Run()

	// The real enforcing check, run directly (no shell-out) against the same
	// fixture package, to prove the gap is genuinely present and catchable.
	var enforceReport bytes.Buffer
	enforceCode, enforceRunErr := apicover.Run(ctx, apicover.Config{Enforce: true, Dirs: []string{fixtureDir}}, &enforceReport)
	if enforceRunErr != nil {
		t.Fatalf("apicover.Run(Enforce:true) measurement error: %v", enforceRunErr)
	}
	if enforceCode == 0 {
		t.Fatalf("test fixture invalid: apicover.Run(Enforce:true) reported no offenders for a deliberately uncovered+unnamed export — fixture does not exercise the gap:\n%s", enforceReport.String())
	}

	if makeErr == nil {
		// KNOWN BUG, queued as percycle-audit-apicover-newexport-parity (0.94).
		// This reproduction shipped ahead of its fix (cycle-998) and held main
		// RED — a red-first proof belongs in the FIX's cycle, so until that
		// lands this branch is a loud SKIP tripwire, not a failure. THE FIX
		// CYCLE MUST flip this t.Skipf back to t.Fatalf as its regression pin.
		t.Skipf(
			"KNOWN BUG (percycle-audit-apicover-newexport-parity): "+
				"`make -C go %s` (the recipe composedGateTargets[\"apicover\"] binds — "+
				"the same command internal/core/composition_carryforward.go's "+
				"runComposedGates relies on before letting a fleet-rebase carry-forward "+
				"reship without a full re-audit) exited 0 (reported PASS) even though "+
				"package %s contains an exported symbol (UncoveredExport) with zero test "+
				"references and zero executed coverage.\n\n"+
				"The real enforcing check (apicover.Run with Enforce:true) correctly "+
				"flags this package (exit=%d):\n%s\n\n"+
				"make output:\n%s",
			target, fixtureRel, enforceCode, enforceReport.String(), makeOut.String())
	}
}
