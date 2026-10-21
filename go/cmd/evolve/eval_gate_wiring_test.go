package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func writeFixtureFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEvalGateReviewer_TheProductionReviewerRefusesANarrowedACTable(t *testing.T) {
	t.Parallel()
	root, ws := t.TempDir(), t.TempDir()
	writeFixtureFile(t, filepath.Join(root, ".evolve", "inbox", "2026-09-30T00-00-00Z-prune-guard.json"),
		`{"id":"prune-guard","title":"Prune guard","acceptance":["prune aborts on a checked-out branch","a continuation-bound branch is never pruned"]}`)
	writeFixtureFile(t, filepath.Join(ws, "triage-decision.json"), `{"top_n":[{"id":"prune-guard"}],"deferred":[]}`)
	writeFixtureFile(t, filepath.Join(ws, "test-report.md"), "## AC-Materialization\n\n| # | Criterion | Disposition | Needs changed | Predicates |\n|---|---|---|---|---|\n"+
		"| 1 | prune aborts on a checked-out branch | predicate | `go/internal/branches/prune.go` | 001 |\n\n## RED Run Output\n")

	res := evalGateReviewer(config.StageEnforce, nil).Review(context.Background(), core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws, ProjectRoot: root})

	if res.Approve || !strings.Contains(res.Reason, "2 filed acceptance criteria") {
		t.Errorf("Review = %+v; the reviewer that cmd wires must see the inbox item through the scope-path resolver and refuse the narrowed table", res)
	}
}
