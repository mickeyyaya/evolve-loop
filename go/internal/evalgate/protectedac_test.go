package evalgate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func tddReport(t *testing.T, body string) core.ReviewInput {
	t.Helper()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "test-report.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return core.ReviewInput{Phase: "tdd", Workspace: ws}
}

func cycle1788Table(disposition string) string {
	return "## AC-Materialization\n\n" +
		"| # | Criterion | Disposition | Needs changed | Predicates |\n" +
		"|---|-----------|-------------|---------------|------------|\n" +
		"| 1 | the lint flags an inverted idiom | predicate | `go/internal/evalqualitycheck/unsatisfiable.go` | 001 |\n" +
		"| 7 | `evolve eval quality-check` runs the lint | " + disposition + " | `go/internal/cli/guardcmd/eval.go` | 007 |\n" +
		"| 8 | quality-check exits 2 on a finding | " + disposition + " | go/internal/cli/guardcmd/eval.go:40 | 008 |\n" +
		"| 9 | a row cut short | predicate |\n\n" +
		"## RED Run Output\n"
}

func TestProtectedACGate_ThePredicatesOfCycle1788OnAProtectedPathBlock(t *testing.T) {
	reason, block := protectedACGate{}.check(tddReport(t, cycle1788Table("predicate")))

	if !block || !strings.Contains(reason, "AC 7, 8") || !strings.Contains(reason, "go/internal/cli/guardcmd/eval.go") || !strings.Contains(reason, "console-followup") {
		t.Fatalf("check = (%q, %v); a lane predicate on a protected path is unsatisfiable by construction, so it must become a console-followup", reason, block)
	}
	if strings.Contains(reason, "AC 1") {
		t.Errorf("reason %q names AC 1, whose path a lane can change", reason)
	}
}

func TestProtectedACGate_AConsoleFollowupOnAProtectedPathPasses(t *testing.T) {
	for _, disposition := range []string{"console-followup", "console-followup (no predicate)", "**console-followup**"} {
		if reason, block := (protectedACGate{}).check(tddReport(t, cycle1788Table(disposition))); block || reason != "" {
			t.Errorf("%s: check = (%q, %v); a console-followup is the legal disposition of a protected AC", disposition, reason, block)
		}
	}
	if _, block := (protectedACGate{}).check(tddReport(t, cycle1788Table("**predicate**"))); !block {
		t.Error("a bold predicate disposition is still a predicate")
	}
}

func TestProtectedACGate_AnACTableWithoutTheNeedsChangedColumnBlocks(t *testing.T) {
	report := "## AC-Materialization\n| # | Criterion | Disposition | Predicates |\n|---|---|---|---|\n| 1 | x | predicate | 001 |\n"

	reason, block := protectedACGate{}.check(tddReport(t, report))

	if !block || !strings.Contains(reason, "needs changed") {
		t.Fatalf("check = (%q, %v); without the column the gate cannot see a protected AC, so the table must carry it", reason, block)
	}
}

func TestProtectedACGate_AReportWithNoACTableIsNotChecked(t *testing.T) {
	for name, in := range map[string]core.ReviewInput{
		"no AC table":   tddReport(t, "## AC-Materialization\nTriage committed nothing.\n\n## RED Run Output\n| File | Count |\n|---|---|\n"),
		"no report":     {Phase: "tdd", Workspace: t.TempDir()},
		"no AC heading": tddReport(t, "## RED Run Output\n| # | Disposition |\n|---|---|\n| 1 | predicate |\n"),
	} {
		if reason, block := (protectedACGate{}).check(in); block || reason != "" {
			t.Errorf("%s: check = (%q, %v), want no finding", name, reason, block)
		}
	}
}

func TestProtectedACGate_WiredIntoReviewer(t *testing.T) {
	rv := NewReviewer(config.StageEnforce).(*reviewer)
	rv.logf = func(string, ...any) {}

	res := rv.Review(context.Background(), tddReport(t, cycle1788Table("predicate")))

	if res.Approve || !strings.Contains(res.Reason, "go/internal/cli/guardcmd/eval.go") {
		t.Fatalf("Review = %+v; the reviewer must run the protected-AC gate on the TDD phase", res)
	}
	if !(protectedACGate{}).appliesTo("tdd") || (protectedACGate{}).appliesTo("build") {
		t.Error("the gate reviews the TDD report only")
	}
}

func TestProtectedACGate_OnlyTheACSectionsTableIsRead(t *testing.T) {
	earlier := "## Coverage Gaps\n| Gap | Disposition |\n|---|---|\n| flaky path | deferred |\n\n"
	if reason, block := (protectedACGate{}).check(tddReport(t, earlier+cycle1788Table("console-followup"))); block || reason != "" {
		t.Fatalf("check = (%q, %v); a Disposition table outside the AC section is not the AC table", reason, block)
	}
	later := "## AC-Materialization\nAll ACs are predicates.\n\n## Notes\n| # | Disposition |\n|---|---|\n| 1 | predicate |\n"
	if reason, block := (protectedACGate{}).check(tddReport(t, later)); block || reason != "" {
		t.Fatalf("check = (%q, %v); a table after the AC section ends is not read", reason, block)
	}
	if reason, block := (protectedACGate{}).check(tddReport(t, "## Acceptance\n| # | Disposition | Needs changed |\n|---|---|---|\n| 7 | predicate | go/internal/cli/guardcmd/eval.go |\n")); !block || !strings.Contains(reason, "AC 7") {
		t.Fatalf("check = (%q, %v); the accepted heading alias ## Acceptance holds the AC table too", reason, block)
	}
}
