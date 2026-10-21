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

func cycle1773Input(t *testing.T, rows string) (core.ReviewInput, func(string, string) string) {
	t.Helper()
	in := tddReport(t, acReport("## AC-Materialization", rows))
	item := filepath.Join(t.TempDir(), "prune-guard.json")
	if err := os.WriteFile(item, []byte(`{"id":"prune-guard","acceptance":["prune aborts on a checked-out branch","a continuation-bound branch is never pruned"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(in.Workspace, "triage-decision.json"), []byte(`{"top_n":[{"id":"prune-guard"}],"deferred":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return in, func(_, id string) string {
		if id == "prune-guard" {
			return item
		}
		return ""
	}
}

const cycle1773OneRow = "| 1 | prune aborts on a checked-out branch | predicate | `go/internal/branches/prune.go` | 001 |\n"

func TestAcceptanceRowsGate_ANarrowedACTableIsRefusedBeforeTheBuild(t *testing.T) {
	t.Parallel()
	in, resolve := cycle1773Input(t, cycle1773OneRow)

	reason, block := acceptanceRowsGate{resolve: resolve}.check(in)

	if !block || !strings.Contains(reason, "2 filed acceptance criteria") || !strings.Contains(reason, "1 row") || !strings.Contains(reason, "deferred") {
		t.Errorf("check = (%q, %v); want the narrowed table refused, naming both counts and the deferred disposition", reason, block)
	}
}

func TestAcceptanceRowsGate_EveryCriterionWithARowPasses(t *testing.T) {
	t.Parallel()
	in, resolve := cycle1773Input(t, cycle1773OneRow+"| 2 | a continuation-bound branch is never pruned | deferred | - | - |\n")
	unresolved, _ := cycle1773Input(t, cycle1773OneRow)

	if reason, block := (acceptanceRowsGate{resolve: resolve}).check(in); block || reason != "" {
		t.Errorf("check = (%q, %v); a table with a row for each criterion, one deferred, must pass", reason, block)
	}
	if reason, block := (acceptanceRowsGate{}).check(unresolved); block || reason != "" {
		t.Errorf("check without a resolver = (%q, %v); a gate that cannot see the contract must not block", reason, block)
	}
}

func TestAcceptanceRowsGate_WiredIntoReviewer(t *testing.T) {
	t.Parallel()
	in, resolve := cycle1773Input(t, cycle1773OneRow)

	res := NewReviewer(config.StageEnforce, WithScopePathResolver(resolve)).Review(context.Background(), in)

	if res.Approve || !strings.Contains(res.Reason, "2 filed acceptance criteria") {
		t.Errorf("Review = %+v; NewReviewer must run the acceptance-rows gate with the resolver it is given", res)
	}
}

func TestAcceptanceRowsGate_NoReportOrNoACSectionIsNotChecked(t *testing.T) {
	t.Parallel()
	noSection, resolve := cycle1773Input(t, cycle1773OneRow)
	if err := os.WriteFile(filepath.Join(noSection.Workspace, "test-report.md"), []byte("## RED Run Output\nall red\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	noReport, resolveNoReport := cycle1773Input(t, cycle1773OneRow)
	if err := os.Remove(filepath.Join(noReport.Workspace, "test-report.md")); err != nil {
		t.Fatal(err)
	}

	for name, c := range map[string]struct {
		in      core.ReviewInput
		resolve func(string, string) string
	}{"no AC section": {noSection, resolve}, "no report": {noReport, resolveNoReport}} {
		if reason, block := (acceptanceRowsGate{resolve: c.resolve}).check(c.in); block || reason != "" {
			t.Errorf("%s: check = (%q, %v); the gate counts rows only in a table that exists", name, reason, block)
		}
	}
}
