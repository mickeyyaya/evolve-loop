package topngate

// scope_reconciliation_test.go — the red-first contract for inbox item
// `multi-slug-lane-scope-reconciliation` (cycle-1620).
//
// The defect (cycle-1480, batch-20260815c wave-2, recurred cycle-1483). A lane
// bundled two slugs: `minted-phase-verdict-contract-unsatisfiable` and
// `dead-api-sweep`. TDD minted a cycle-wide predicate suite covering BOTH; the
// Builder's deliverable contract bound only the FIRST. Nothing reconciled the
// two scopes, so the lane ran the full ~12-phase spine and FAILed at audit with
// slug 2 entirely undelivered. Verbatim audit H1: "TDD minted a cycle-wide
// predicate suite covering both slugs while the Builder contract bound only the
// first slug; nothing reconciles the two scopes."
//
// The contract these tests freeze. When triage's ## top_n commits TWO OR MORE
// members, the TDD deliverable's DECLARED member set must EQUAL the committed
// set; a missing committed member is a CERTAIN scope mismatch and must abort at
// the TDD->Build boundary with a reason containing the marker `scope-mismatch`
// and NAMING every omitted member. A one-member commitment keeps today's
// fail-open / label-drift-advisory behavior exactly (no regression).
//
// The declared member set is stated BOTH ways a TDD report can state it — the
// "## Task:" header (comma-separated when plural) and the "## Handoff to
// Builder" JSON's slugs[] — so these tests bind to the DECLARATION, never to
// one syntax. JSON shown inside an OUTER `~~~markdown` example fence is
// illustration, not declaration, and must never be read as the handoff
// (recovery review 2026-09-09, the false-accept control).

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

// mdFence is the markdown code-fence delimiter. Named because a Go raw string
// literal cannot contain a backtick.
const mdFence = "```"

// writeScopedTDDReport writes a test-report.md declaring exactly `declared` as
// its member set, stated in BOTH the "## Task:" header and the handoff JSON's
// slugs[]. preamble is rendered verbatim between the header and the RED output
// (the outer-fence false-accept control injects its fake handoff there).
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

// reviewTDD drives the PRODUCTION reviewer at the TDD->Build boundary — the
// same constructor and stage cmd/evolve/cmd_cycle.go:657 wires into the cycle's
// reviewer chain (config default TopNGate=StageEnforce). A helper called
// directly would prove nothing about the shipped path.
func reviewTDD(t *testing.T, workspace string) core.ReviewResult {
	t.Helper()
	return NewReviewer(config.StageEnforce).Review(context.Background(), core.ReviewInput{
		Phase:     string(core.PhaseTDD),
		Workspace: workspace,
	})
}

// TestTDDScopeGate_TwoSlugHandoffOmittingCommittedMemberBlocks is the cycle-1480
// shape verbatim: two committed members, one declared. Build must not start.
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

// TestTDDScopeGate_TwoSlugCompleteDeclarationProceeds is the anti-no-op control:
// blocking every multi-member lane would pass the test above and kill the fleet's
// only legitimate bundling path. A COMPLETE declaration must proceed.
func TestTDDScopeGate_TwoSlugCompleteDeclarationProceeds(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "minted-phase-verdict-contract-unsatisfiable", "dead-api-sweep")
	writeScopedTDDReport(t, ws, []string{"minted-phase-verdict-contract-unsatisfiable", "dead-api-sweep"}, "")

	if got := reviewTDD(t, ws); !got.Approve {
		t.Fatalf("a two-member commitment whose TDD declaration covers BOTH members must proceed to Build; got %+v", got)
	}
}

// TestTDDScopeGate_TwoSlugDeclarationOrderIsIrrelevant pins SET equality, not
// list equality: the members are unordered work items, and a gate that keyed on
// order would false-block correct work (the cycles 916/1012 destruction class).
func TestTDDScopeGate_TwoSlugDeclarationOrderIsIrrelevant(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "minted-phase-verdict-contract-unsatisfiable", "dead-api-sweep")
	writeScopedTDDReport(t, ws, []string{"dead-api-sweep", "minted-phase-verdict-contract-unsatisfiable"}, "")

	if got := reviewTDD(t, ws); !got.Approve {
		t.Fatalf("member ORDER must not decide the reconciliation — the committed set is unordered work; got %+v", got)
	}
}

// TestTDDScopeGate_SingleSlugLaneUnaffected is the no-regression control: the
// overwhelming majority of lanes commit ONE member, and their fail-open /
// label-drift-advisory behavior must be untouched by the new equality rule.
func TestTDDScopeGate_SingleSlugLaneUnaffected(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "multi-slug-lane-scope-reconciliation")
	writeScopedTDDReport(t, ws, []string{"multi-slug-lane-scope-reconciliation"}, "")

	if got := reviewTDD(t, ws); !got.Approve {
		t.Fatalf("a single-member lane whose TDD declaration matches must keep proceeding; got %+v", got)
	}
}

// TestTDDScopeGate_ThreeSlugPartialDeclarationBlocks is the OOD case: the defect
// is not specific to N=2. Two of three declared is still an undelivered member.
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

// TestTDDScopeGate_OuterFenceFakeCompleteHandoffStillBlocks is the false-accept
// control from the 2026-09-09 recovery review. The report DOCUMENTS a complete
// handoff inside an outer `~~~markdown` example fence while actually declaring
// one member. Illustration is not declaration: the lane must still block.
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

// TestTDDScopeGate_ScopeMismatchIsDeterministic pins the AC's "deterministic
// check": identical inputs must yield an identical verdict every time. A gate
// that depends on map iteration order (the committed/declared sets are set
// comparisons) would report the omitted members in a shuffling order and make
// the block un-reproducible in the audit trail.
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

// TestTDDScopeGate_AppliesBeforeBuildNotAfter pins the BOUNDARY the inbox item
// names: the reconciliation must abort at TDD->Build, not later. Reviewing the
// build phase's deliverable with the same workspace must not resurrect the
// TDD-scope block (that would move the abort past the spend it exists to save).
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
