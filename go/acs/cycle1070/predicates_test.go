//go:build acs

package cycle1070

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/topngate"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func writeTriage(t *testing.T, ws string, topN ...string) {
	t.Helper()
	var b strings.Builder
	b.WriteString("<!-- ANCHOR:triage_decision -->\n# Triage Decision — Cycle 1070\n\n")
	b.WriteString("## top_n (commit to THIS cycle)\n")
	for _, s := range topN {
		b.WriteString("- " + s + ": placeholder — priority=H, evidence=x, source=inbox\n")
	}
	b.WriteString("\n## deferred (carry to NEXT cycle's carryoverTodos)\n- some-other-item: deferred\n")
	if err := os.WriteFile(filepath.Join(ws, "triage-report.md"), []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write triage-report.md: %v", err)
	}
}

func writeTestReport(t *testing.T, ws, claimedSlug string, testFiles ...string) {
	t.Helper()
	var b strings.Builder
	b.WriteString("# TDD Report — Cycle 1070\n\n## Task: " + claimedSlug + "\n\n")
	b.WriteString("## Test Files Written\n| File | Test Count | Framework |\n|---|---|---|\n")
	for _, f := range testFiles {
		b.WriteString("| " + f + " | 1 | Go |\n")
	}
	b.WriteString("\n## Handoff to Builder\n```json\n{\n  \"testFiles\": [")
	for i, f := range testFiles {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("\"" + f + "\"")
	}
	b.WriteString("],\n  \"redRunConfirmed\": true,\n  \"doNotModifyTests\": true\n}\n```\n")
	if err := os.WriteFile(filepath.Join(ws, "test-report.md"), []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write test-report.md: %v", err)
	}
}

func reviewTDD(t *testing.T, stage config.Stage, ws string) core.ReviewResult {
	t.Helper()
	return topngate.NewReviewer(stage).Review(context.Background(), core.ReviewInput{
		Phase:     string(core.PhaseTDD),
		Workspace: ws,
	})
}

func TestC1070_001_EmptyTopNBlocksAuthoredTDDFiles(t *testing.T) {
	ws := t.TempDir()
	writeTriage(t, ws)
	writeTestReport(t, ws, "declined-slug", "go/acs/cycle660/predicates_test.go")

	res := reviewTDD(t, config.StageEnforce, ws)
	if res.Approve {
		t.Errorf("empty top_n + TDD-authored scaffolds must be REJECTED at enforce; got Approve=true (cycle-660 repro: TDD authored files for a slug triage declined)")
	}
	if !res.Approve && strings.TrimSpace(res.Reason) == "" {
		t.Errorf("a blocked review must carry a non-empty Reason (operators need the abort reason); got %q", res.Reason)
	}
}

func TestC1070_002_OutOfLaneSlugAdvisoryTDD(t *testing.T) {
	ws := t.TempDir()
	writeTriage(t, ws, "committed-slug-a", "committed-slug-b")
	writeTestReport(t, ws, "totally-other-slug", "go/acs/cycle1070/predicates_test.go")

	res := reviewTDD(t, config.StageEnforce, ws)
	if !res.Approve {
		t.Errorf("label drift against a NON-EMPTY committed top_n must be advisory, not a block; got Approve=false reason=%q", res.Reason)
	}
}

func TestC1070_003_InLaneTDDIsApproved(t *testing.T) {
	ws := t.TempDir()
	writeTriage(t, ws, "tdd-topn-scope-gate", "other-committed")
	writeTestReport(t, ws, "tdd-topn-scope-gate", "go/acs/cycle1070/predicates_test.go")

	if res := reviewTDD(t, config.StageEnforce, ws); !res.Approve {
		t.Errorf("in-lane TDD deliverable must be APPROVED at enforce; got Approve=false reason=%q", res.Reason)
	}

	bare := t.TempDir()
	writeTestReport(t, bare, "whatever", "go/acs/cycle1070/predicates_test.go")
	if res := reviewTDD(t, config.StageEnforce, bare); !res.Approve {
		t.Errorf("missing triage-report.md must fail OPEN (no committed set to bind against); got Approve=false reason=%q", res.Reason)
	}
}

func TestC1070_004_ShadowStageNeverBlocksTDD(t *testing.T) {
	ws := t.TempDir()
	writeTriage(t, ws)
	writeTestReport(t, ws, "declined-slug", "go/acs/cycle660/predicates_test.go")

	if res := reviewTDD(t, config.StageShadow, ws); !res.Approve {
		t.Errorf("StageShadow must observe-and-approve, never block; got Approve=false reason=%q", res.Reason)
	}
}

func TestC1070_005_BuildSideGateHasNoRegression(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", filepath.Join(root, "go"), "test", "-race", "-count=1", "./internal/topngate/...")
	if err != nil || code != 0 {
		t.Errorf("go test -race ./internal/topngate/... must pass (no build-gate regression); code=%d err=%v\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}
}
