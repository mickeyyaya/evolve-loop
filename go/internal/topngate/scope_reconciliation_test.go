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
	writeMemberTDDReport(t, workspace, declared, preamble, "go/acs/cycle1480/predicates_test.go")
}

func writeMemberTDDReport(t *testing.T, workspace string, declared []string, preamble string, testFiles ...string) {
	t.Helper()
	handoff, err := json.Marshal(map[string]any{
		"slugs":            declared,
		"testFiles":        testFiles,
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

// twoMemberLane commits alpha-member and beta-member, each declaring its own package.
func twoMemberLane(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()
	writeTriageReport(t, ws, "alpha-member", "beta-member")
	writeScoutReportTasks(t, ws,
		scoutTask{slug: "alpha-member", targetFiles: []string{"go/internal/alpha/alpha.go"}},
		scoutTask{slug: "beta-member", targetFiles: []string{"go/internal/beta/beta.go"}},
	)
	return ws
}

func TestTDDScopeGate_TwoMemberFileScopeDriftIsAdvised(t *testing.T) {
	ws := twoMemberLane(t)
	writeMemberTDDReport(t, ws, []string{"alpha-member", "beta-member"}, "", "go/internal/tokenresolver/resolver_test.go")

	reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
	if block {
		t.Fatalf("file-scope drift must stay advisory for a multi-member lane; got block=true reason=%q", reason)
	}
	if !strings.Contains(reason, "file scope drift") {
		t.Fatalf("a complete two-member declaration authoring outside both members' scopes must emit the file-scope advisory; got %q", reason)
	}
	for _, want := range []string{
		"alpha-member", "beta-member",
		"go/internal/tokenresolver/resolver_test.go",
		"go/internal/alpha/alpha.go", "go/internal/beta/beta.go",
	} {
		if !strings.Contains(reason, want) {
			t.Errorf("advisory must name both members, the authored files and every member's declared targetFiles; missing %q in %q", want, reason)
		}
	}
	if res := reviewTDD(t, ws); !res.Approve {
		t.Errorf("the multi-member file-scope advisory must approve at enforce; got %+v", res)
	}
}

func TestTDDScopeGate_TwoMemberInEitherScopeStaysSilent(t *testing.T) {
	for _, tc := range []struct {
		name     string
		authored []string
	}{
		{"inside the first member's scope", []string{"go/internal/alpha/alpha_test.go"}},
		{"inside the second member's scope", []string{"go/internal/beta/beta_test.go"}},
		{"one overlapping file among several", []string{"go/acs/cycle1480/predicates_test.go", "go/internal/beta/beta_test.go"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := twoMemberLane(t)
			writeMemberTDDReport(t, ws, []string{"alpha-member", "beta-member"}, "", tc.authored...)

			reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
			if reason != "" || block {
				t.Errorf("authored files inside ANY member's declared scope must stay silent; got reason=%q block=%v", reason, block)
			}
		})
	}
}

func TestTDDScopeGate_TwoMemberWithoutDeclaredScopeStaysSilent(t *testing.T) {
	t.Run("no scout-report.md", func(t *testing.T) {
		ws := t.TempDir()
		writeTriageReport(t, ws, "alpha-member", "beta-member")
		writeMemberTDDReport(t, ws, []string{"alpha-member", "beta-member"}, "", "go/internal/tokenresolver/resolver_test.go")

		reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
		if reason != "" || block {
			t.Errorf("no declared scope to compare against must fail open; got reason=%q block=%v", reason, block)
		}
	})

	t.Run("scout declares neither member", func(t *testing.T) {
		ws := t.TempDir()
		writeTriageReport(t, ws, "alpha-member", "beta-member")
		writeScoutReportTasks(t, ws, scoutTask{slug: "decoy-task", targetFiles: []string{"go/internal/other/other.go"}})
		writeMemberTDDReport(t, ws, []string{"alpha-member", "beta-member"}, "", "go/internal/tokenresolver/resolver_test.go")

		reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
		if reason != "" || block {
			t.Errorf("a sibling task's scope must not stand in for the committed members'; got reason=%q block=%v", reason, block)
		}
	})
}

func TestTDDScopeGate_IncompleteMemberDeclarationBlocksBeforeScopeCheck(t *testing.T) {
	ws := twoMemberLane(t)
	writeMemberTDDReport(t, ws, []string{"alpha-member"}, "", "go/internal/tokenresolver/resolver_test.go")

	reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
	if !block || !strings.Contains(reason, "scope-mismatch") {
		t.Fatalf("an incomplete member declaration must still block on scope-mismatch; got reason=%q block=%v", reason, block)
	}
	if strings.Contains(reason, "file scope") {
		t.Errorf("file-scope drift is judged only after a complete reconciliation, never folded into the mismatch block; got %q", reason)
	}
}

func TestTDDScopeGate_SingleMemberFileScopeAdvisoryTextUnchanged(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "committed-slug")
	writeScoutReport(t, ws, "committed-slug", "go/internal/topngate/gate.go", "go/internal/topngate/gate_test.go")
	writeTDDReport(t, ws, "committed-slug", "go/internal/tokenresolver/resolver_test.go")

	want := "file scope drift (advisory): TDD authored test file(s) {go/internal/tokenresolver/resolver_test.go}" +
		" but the committed item 'committed-slug' declares targetFiles {go/internal/topngate/gate.go, go/internal/topngate/gate_test.go}" +
		" — zero path overlap"
	reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
	if block || reason != want {
		t.Errorf("single-member lanes must keep today's advisory byte for byte;\n got reason=%q block=%v\nwant reason=%q block=false", reason, block, want)
	}
}

func TestTDDScopeGate_TwoMemberWithOneUndeclaredScopeStaysSilent(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "alpha-member", "beta-member")
	writeScoutReportTasks(t, ws, scoutTask{slug: "alpha-member", targetFiles: []string{"go/internal/alpha/alpha.go"}})
	writeMemberTDDReport(t, ws, []string{"alpha-member", "beta-member"}, "", "go/internal/tokenresolver/resolver_test.go")

	reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
	if reason != "" || block {
		t.Errorf("a member with no declared scope could own any file, so the union check must fail open; got reason=%q block=%v", reason, block)
	}
}
