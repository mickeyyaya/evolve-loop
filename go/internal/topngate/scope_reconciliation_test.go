package topngate

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// mdFence exists because a raw string literal cannot contain a backtick.
const mdFence = "```"

// writeScopedTDDReport states declared in both the ## Task: header and the
// handoff slugs[]; preamble goes between the header and the RED output.
func writeScopedTDDReport(t *testing.T, workspace string, declared []string, preamble string) {
	t.Helper()
	handoff, err := json.Marshal(map[string]any{
		"slugs":            declared,
		"testFiles":        []string{"go/acs/cycle1480/predicates_test.go"},
		"redRunConfirmed":  true,
		"doNotModifyTests": true,
	})
	if err != nil {
		t.Fatalf("marshal handoff: %v", err)
	}
	body := strings.Join([]string{
		"# TDD Report — Cycle 1480",
		"",
		"## Task: " + strings.Join(declared, ", "),
		"",
		preamble,
		"## RED Run Output",
		"",
		mdFence,
		"FAIL",
		mdFence,
		"",
		"## Handoff to Builder",
		"",
		mdFence + "json",
		string(handoff),
		mdFence,
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(workspace, "test-report.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write test-report: %v", err)
	}
}

func reviewTDD(t *testing.T, workspace string) core.ReviewResult {
	t.Helper()
	return NewReviewer(config.StageEnforce).Review(context.Background(), core.ReviewInput{
		Phase:     string(core.PhaseTDD),
		Workspace: workspace,
	})
}

func TestTDDScopeGate_TwoSlugHandoffOmittingCommittedMemberBlocks(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "minted-phase-verdict-contract-unsatisfiable", "dead-api-sweep")
	writeScopedTDDReport(t, ws, []string{"minted-phase-verdict-contract-unsatisfiable"}, "")

	got := reviewTDD(t, ws)
	if got.Approve || !strings.Contains(got.Reason, "scope-mismatch") {
		t.Fatalf("a two-member commitment with a one-member TDD declaration must block before Build "+
			"with a named scope-mismatch defect; got %+v", got)
	}
	if !strings.Contains(got.Reason, "dead-api-sweep") {
		t.Errorf("the block must NAME the undelivered member so the defect is actionable without re-deriving the diff; reason=%q", got.Reason)
	}
}

func TestTDDScopeGate_TwoSlugCompleteDeclarationProceeds(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "minted-phase-verdict-contract-unsatisfiable", "dead-api-sweep")
	writeScopedTDDReport(t, ws, []string{"minted-phase-verdict-contract-unsatisfiable", "dead-api-sweep"}, "")

	if got := reviewTDD(t, ws); !got.Approve {
		t.Fatalf("a two-member commitment whose TDD declaration covers BOTH members must proceed to Build; got %+v", got)
	}
}

func TestTDDScopeGate_TwoSlugDeclarationOrderIsIrrelevant(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "minted-phase-verdict-contract-unsatisfiable", "dead-api-sweep")
	writeScopedTDDReport(t, ws, []string{"dead-api-sweep", "minted-phase-verdict-contract-unsatisfiable"}, "")

	if got := reviewTDD(t, ws); !got.Approve {
		t.Fatalf("member ORDER must not decide the reconciliation — the committed set is unordered work; got %+v", got)
	}
}

func TestTDDScopeGate_SingleSlugLaneUnaffected(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "multi-slug-lane-scope-reconciliation")
	writeScopedTDDReport(t, ws, []string{"multi-slug-lane-scope-reconciliation"}, "")

	if got := reviewTDD(t, ws); !got.Approve {
		t.Fatalf("a single-member lane whose TDD declaration matches must keep proceeding; got %+v", got)
	}
}

func TestTDDScopeGate_ThreeSlugPartialDeclarationBlocks(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "alpha", "beta", "gamma")
	writeScopedTDDReport(t, ws, []string{"alpha", "beta"}, "")

	got := reviewTDD(t, ws)
	if got.Approve || !strings.Contains(got.Reason, "scope-mismatch") {
		t.Fatalf("a three-member commitment with a two-member declaration must block; got %+v", got)
	}
	if !strings.Contains(got.Reason, "gamma") {
		t.Errorf("the block must name the omitted member `gamma`; reason=%q", got.Reason)
	}
}

func TestTDDScopeGate_OuterFenceFakeCompleteHandoffStillBlocks(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "minted-phase-verdict-contract-unsatisfiable", "dead-api-sweep")
	fake := strings.Join([]string{
		"## Handoff schema (documentation only — NOT this cycle's declaration)",
		"",
		"~~~markdown",
		"## Handoff to Builder",
		"",
		mdFence + "json",
		`{"slugs": ["minted-phase-verdict-contract-unsatisfiable", "dead-api-sweep"], "testFiles": ["go/acs/cycleN/predicates_test.go"], "redRunConfirmed": true}`,
		mdFence,
		"~~~",
		"",
	}, "\n")
	writeScopedTDDReport(t, ws, []string{"minted-phase-verdict-contract-unsatisfiable"}, fake)

	got := reviewTDD(t, ws)
	if got.Approve || !strings.Contains(got.Reason, "scope-mismatch") {
		t.Fatalf("a complete handoff shown inside an OUTER example fence is illustration, not declaration — "+
			"the lane must still block on its real one-member declaration; got %+v", got)
	}
}

func TestTDDScopeGate_ScopeMismatchIsDeterministic(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "alpha", "beta", "gamma")
	writeScopedTDDReport(t, ws, []string{"alpha"}, "")

	first := reviewTDD(t, ws)
	if first.Approve || !strings.Contains(first.Reason, "scope-mismatch") {
		t.Fatalf("expected a scope-mismatch block to compare against; got %+v", first)
	}
	for i := 0; i < 8; i++ {
		if got := reviewTDD(t, ws); got != first {
			t.Fatalf("run %d diverged from the first verdict:\n first=%+v\n   got=%+v", i+2, first, got)
		}
	}
}

func TestTDDScopeGate_AppliesBeforeBuildNotAfter(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "minted-phase-verdict-contract-unsatisfiable", "dead-api-sweep")
	writeScopedTDDReport(t, ws, []string{"minted-phase-verdict-contract-unsatisfiable"}, "")
	writeBuildReport(t, ws, "minted-phase-verdict-contract-unsatisfiable")

	if got := reviewTDD(t, ws); got.Approve {
		t.Fatalf("the TDD boundary must be where the mismatch aborts; got %+v", got)
	}
	got := NewReviewer(config.StageEnforce).Review(context.Background(), core.ReviewInput{
		Phase:     string(core.PhaseBuild),
		Workspace: ws,
	})
	if !got.Approve && strings.Contains(got.Reason, "scope-mismatch") {
		t.Errorf("the TDD-scope reconciliation must not re-fire at the build boundary — "+
			"the whole point is aborting BEFORE the build spend; got %+v", got)
	}
}
