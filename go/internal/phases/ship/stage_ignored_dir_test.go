package ship

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func TestShipDirect_CycleClass_RetriesAfterGitNamesAnIgnoredPathspec(t *testing.T) {
	t.Parallel()
	root := stageExplicitTree(t)
	blind := ".evolve/inbox/processed"
	ws := writeWorkspaceReports(t,
		"go/internal/phases/ship/gitops.go",
		blind)
	cap := &porcelainCapture{
		porcelain:      " M go/internal/phases/ship/gitops.go\n?? " + blind + "\n",
		refuseAddPaths: []string{blind},
	}
	opts := stageExplicitOpts(root, ws, ClassCycle, cap.runner())

	res := &RunResult{}
	if err := shipDirect(context.Background(), opts, res, "main"); err != nil {
		t.Fatalf("shipDirect must survive a probe-blind ignored pathspec via git's own refusal: %v (2026-08-14 ship|unknown|99c38818 class)", err)
	}

	var adds [][]string
	for _, call := range cap.calls {
		if call[0] == "git" && slices.Contains(call, "add") {
			adds = append(adds, call)
		}
	}
	if len(adds) != 2 {
		t.Fatalf("want exactly ONE retry after the refusal (2 add calls), got %d: %v", len(adds), adds)
	}
	if !slices.Contains(adds[0], blind) {
		t.Fatalf("first add must have carried the blind path (that IS the refusal): %v", adds[0])
	}
	if slices.Contains(adds[1], blind) {
		t.Errorf("retry still carries the refused path — no progress, infinite refusal: %v", adds[1])
	}
	if !slices.Contains(adds[1], "go/internal/phases/ship/gitops.go") {
		t.Errorf("legit path missing from the retry: %v", adds[1])
	}
	var loud bool
	for _, l := range res.Logs {
		if strings.Contains(l, "refused") && strings.Contains(l, blind) {
			loud = true
		}
	}
	if !loud {
		t.Errorf("the git-named drop must be LOUD in ship logs; logs=%v", res.Logs)
	}
}

func TestShipDirect_CycleClass_ForeignOffenderMeansNoRetry(t *testing.T) {
	t.Parallel()
	root := stageExplicitTree(t)
	ws := writeWorkspaceReports(t, "go/internal/phases/ship/gitops.go")
	cap := &porcelainCapture{
		porcelain:       " M go/internal/phases/ship/gitops.go\n",
		refuseAddAlways: ".evolve/somewhere/else",
	}
	opts := stageExplicitOpts(root, ws, ClassCycle, cap.runner())
	res := &RunResult{}
	if err := shipDirect(context.Background(), opts, res, "main"); err == nil {
		t.Fatal("a refusal naming only foreign paths must stay an honest ship error")
	}
	var adds int
	for _, call := range cap.calls {
		if call[0] == "git" && slices.Contains(call, "add") {
			adds++
		}
	}
	if adds != 1 {
		t.Fatalf("no progress possible ⇒ exactly ONE add call (no doomed identical retry), got %d", adds)
	}
}
