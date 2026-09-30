package ship

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestShipDirect_CycleClass_DropsGitignoredDeclaredPaths(t *testing.T) {
	root := stageExplicitTree(t)
	// The eval file exists in the tree (isFile passes — that is how it slips
	// into the pathspec).
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

// TestShipDirect_CheckIgnoreProbeFailure_FailsOpen: a broken probe (rc>1)
// must not block staging — the full declared set flows through unchanged,
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
