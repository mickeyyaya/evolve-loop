// stage_ignored_paths_test.go — regression lock for the cycle-1101 ship fatal
// (2026-07-27, batch-12 attempt-2): the eval-quality contract makes every
// test-report declare its eval file (`.evolve/evals/<slug>.md`), and .evolve/*
// is gitignored BY DESIGN (runtime state, never committed). The declared
// manifest therefore always contains an ignored path, and `git add -A --
// <paths>` REFUSES with rc=1 ("The following paths are ignored by one of your
// .gitignore files") — it stages the legit paths and STILL exits 1, so every
// green cycle aborted at ship after two futile "transient" retries. Layer 2 of
// the staging onion: cycle-1098's absolute-pathspec rc=128 fatal (fixed at
// d202aeb6) fired first and masked this one.
//
// Contract: stageExplicitPaths pre-filters the pathspec through
// `git check-ignore` — ignored entries are dropped and logged, never handed to
// `git add`. A broken check-ignore probe fails OPEN (full set, loud log): a
// probe failure must not block ship.
package ship

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestShipDirect_CycleClass_DropsGitignoredDeclaredPaths — the crux: a
// declared-but-gitignored eval path never reaches the `git add` argv; the
// declared source paths still do.
func TestShipDirect_CycleClass_DropsGitignoredDeclaredPaths(t *testing.T) {
	root := stageExplicitTree(t)
	// The eval file exists in the tree (isFile passes — that is how it slipped
	// into the pathspec on cycle-1101).
	evalRel := ".evolve/evals/persona-budget-inlane-gate.md"
	mustWrite(t, filepath.Join(root, filepath.FromSlash(evalRel)), "# eval\n")
	ws := writeWorkspaceReports(t,
		"go/internal/phases/ship/gitops.go",
		evalRel)
	cap := &porcelainCapture{
		porcelain: " M go/internal/phases/ship/gitops.go\n",
		ignored:   []string{evalRel},
	}
	opts := stageExplicitOpts(root, ws, ClassCycle, cap.runner())

	res := &RunResult{}
	if err := shipDirect(context.Background(), opts, res, "main"); err != nil {
		t.Fatalf("shipDirect(cycle): %v", err)
	}

	pathspec, sawAdd := cap.addPathspec()
	if !sawAdd {
		t.Fatal("cycle ship never invoked git add")
	}
	if slices.Contains(pathspec, evalRel) {
		t.Errorf("gitignored declared path %q reached git add argv %v — git refuses ignored paths with rc=1 (cycle-1101)", evalRel, pathspec)
	}
	if !slices.Contains(pathspec, "go/internal/phases/ship/gitops.go") {
		t.Errorf("legit declared path missing from pathspec %v", pathspec)
	}
	// The drop must be LOUD — an operator reading the ship log sees what was
	// excluded and why, never a silent shrink of the staged set.
	var logged bool
	for _, l := range res.Logs {
		if strings.Contains(l, "gitignored") && strings.Contains(l, evalRel) {
			logged = true
		}
	}
	if !logged {
		t.Errorf("dropped ignored path was not logged; logs=%v", res.Logs)
	}
}

// TestShipDirect_CheckIgnoreProbeFailure_FailsOpen — a broken probe (rc>1)
// must not block staging: the full declared set flows through unchanged (the
// add's own stderr now travels in the ship error if the refusal survives),
// and the probe failure is logged.
func TestShipDirect_CheckIgnoreProbeFailure_FailsOpen(t *testing.T) {
	root := stageExplicitTree(t)
	ws := writeWorkspaceReports(t, "go/internal/phases/ship/gitops.go")
	cap := &porcelainCapture{
		porcelain:     " M go/internal/phases/ship/gitops.go\n",
		checkIgnoreRC: 128,
	}
	opts := stageExplicitOpts(root, ws, ClassCycle, cap.runner())

	res := &RunResult{}
	if err := shipDirect(context.Background(), opts, res, "main"); err != nil {
		t.Fatalf("shipDirect(cycle) with broken check-ignore: %v", err)
	}
	pathspec, sawAdd := cap.addPathspec()
	if !sawAdd || !slices.Contains(pathspec, "go/internal/phases/ship/gitops.go") {
		t.Fatalf("fail-open must stage the full declared set (sawAdd=%v pathspec=%v)", sawAdd, pathspec)
	}
	var warned bool
	for _, l := range res.Logs {
		if strings.Contains(l, "check-ignore") {
			warned = true
		}
	}
	if !warned {
		t.Errorf("probe failure was silent; logs=%v", res.Logs)
	}
}
