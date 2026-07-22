//go:build acs

// Package cycle1070 materialises the cycle-1070 acceptance criteria for the one
// fleet-scoped task pinned to this lane:
//
//   - tdd-topn-scope-gate → extend internal/topngate so the TDD phase's
//     authored predicate/scaffold set is bound to triage's ## top_n commitment.
//
// The defect (cycle-660, inbox tdd-topn-binding-gate, 3rd recurrence): triage
// commits an EMPTY ## top_n, but TDD reads scout-report.md — not
// triage-report.md — and still authors RED scaffolds for a slug triage
// explicitly declined. Build then honours the empty top_n correctly and chokes
// on the orphan scaffolds. topngate already binds the build->audit transition
// (topNBindingGate, gate.go); nothing binds triage->TDD.
//
// Predicate strategy — every predicate below DRIVES the real system through its
// exported seam, topngate.NewReviewer(stage).Review(ctx, core.ReviewInput{...}),
// against t.TempDir()-backed fixture workspaces. No predicate greps production
// source for a magic string (the cycle-85 degenerate-predicate ban): a builder
// who adds the string but not the gate still fails 001/002, and a builder who
// blanket-blocks the TDD phase fails 003/004.
//
//   - 001 is the cycle-660 crux: empty committed top_n + a TDD deliverable whose
//     handoff JSON testFiles[] is non-empty must be REJECTED at StageEnforce.
//   - 002 is the out-of-lane case: a non-empty top_n that does not contain the
//     slug the TDD report claims must be REJECTED at StageEnforce.
//   - 003 is the anti-overreach negative: an in-lane TDD deliverable must be
//     APPROVED (a gate that blocks everything is not a gate).
//   - 004 is the second anti-overreach negative: StageShadow observes but never
//     blocks, even on the 001 violation — the rollout control for this package.
//   - 005 is the no-regression predicate: the build-side gate's own package
//     tests still pass under -race (subprocess `go test`).
//
// Fixture shapes are the CONTRACTED report shapes, not invented ones:
// triage-report.md's "## top_n (commit to THIS cycle)" + "- <slug>: ..." bullets
// (agents/evolve-triage.md Step 4, already parsed by readTopNSlugs), and
// test-report.md's "## Task: <slug>" header + "## Handoff to Builder" JSON
// fence carrying "testFiles" (agents/evolve-tdd.md Step 6).
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

// writeTriage writes a triage-report.md whose ## top_n section lists slugs.
// Passing no slugs writes the EMPTY-section form — the cycle-660 input.
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

// writeTestReport writes a test-report.md claiming claimedSlug and declaring
// testFiles in its ## Handoff to Builder JSON fence. An empty testFiles slice
// writes the "authored nothing" form (the compliant response to an empty top_n).
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

// reviewTDD drives the exported reviewer seam for the TDD phase over workspace ws.
func reviewTDD(t *testing.T, stage config.Stage, ws string) core.ReviewResult {
	t.Helper()
	return topngate.NewReviewer(stage).Review(context.Background(), core.ReviewInput{
		Phase:     string(core.PhaseTDD),
		Workspace: ws,
	})
}

// TestC1070_001_EmptyTopNBlocksAuthoredTDDFiles is the cycle-660 regression: an
// empty committed top_n means TDD must author NOTHING; a non-empty handoff
// testFiles[] under an empty top_n is an unambiguous out-of-lane authoring and
// must be rejected at StageEnforce with a populated reason.
func TestC1070_001_EmptyTopNBlocksAuthoredTDDFiles(t *testing.T) {
	ws := t.TempDir()
	writeTriage(t, ws) // empty ## top_n — triage committed nothing
	writeTestReport(t, ws, "declined-slug", "go/acs/cycle660/predicates_test.go")

	res := reviewTDD(t, config.StageEnforce, ws)
	if res.Approve {
		t.Errorf("empty top_n + TDD-authored scaffolds must be REJECTED at enforce; got Approve=true (cycle-660 repro: TDD authored files for a slug triage declined)")
	}
	if !res.Approve && strings.TrimSpace(res.Reason) == "" {
		t.Errorf("a blocked review must carry a non-empty Reason (operators need the abort reason); got %q", res.Reason)
	}
}

// TestC1070_002_OutOfLaneSlugAdvisoryTDD covers the non-empty-top_n case: TDD
// claims a slug with zero overlap against the committed set.
//
// POLICY CHANGE 2026-07-23 (cycle-1073 tdd-topn-scope-gate): this case is now
// an ADVISORY, not a block — the same conversion #348/cbd088a1 made for the
// sibling build-side gate after cycles 916 + 1012 recorded two fatal
// rejections that discarded CORRECT work over label drift between two
// LLM-authored strings, with zero true-fraud catches. The lane exists BECAUSE
// triage committed these ids, so the committed set is the binding authority.
// The predicate is rebound (not deleted) to the advisory contract: approved,
// with the drift still surfaced in a populated reason. The empty-top_n case
// (001) stays fatal — there is no committed item the files could be a
// differently-labelled response to.
func TestC1070_002_OutOfLaneSlugAdvisoryTDD(t *testing.T) {
	ws := t.TempDir()
	writeTriage(t, ws, "committed-slug-a", "committed-slug-b")
	writeTestReport(t, ws, "totally-other-slug", "go/acs/cycle1070/predicates_test.go")

	res := reviewTDD(t, config.StageEnforce, ws)
	if !res.Approve {
		t.Errorf("label drift against a NON-EMPTY committed top_n must be advisory, not a block; got Approve=false reason=%q", res.Reason)
	}
	// The drift is still surfaced through the reviewer's structured logf seam,
	// which is not observable from outside the package; that half of the
	// contract is pinned white-box by
	// internal/topngate.TestTDDScopeGate_LabelDriftIsAdvisory.
}

// TestC1070_003_InLaneTDDIsApproved is the anti-overreach negative: the healthy
// path (TDD authoring predicates for a committed slug) must still be approved at
// StageEnforce. A gate that rejects this blocks every honest cycle.
func TestC1070_003_InLaneTDDIsApproved(t *testing.T) {
	ws := t.TempDir()
	writeTriage(t, ws, "tdd-topn-scope-gate", "other-committed")
	writeTestReport(t, ws, "tdd-topn-scope-gate", "go/acs/cycle1070/predicates_test.go")

	if res := reviewTDD(t, config.StageEnforce, ws); !res.Approve {
		t.Errorf("in-lane TDD deliverable must be APPROVED at enforce; got Approve=false reason=%q", res.Reason)
	}

	// Fail-open on ambiguity: an absent triage-report.md (nothing to bind
	// against) must never block — the gate.go fail-open rule for this package.
	bare := t.TempDir()
	writeTestReport(t, bare, "whatever", "go/acs/cycle1070/predicates_test.go")
	if res := reviewTDD(t, config.StageEnforce, bare); !res.Approve {
		t.Errorf("missing triage-report.md must fail OPEN (no committed set to bind against); got Approve=false reason=%q", res.Reason)
	}
}

// TestC1070_004_ShadowStageNeverBlocksTDD pins the rollout control: at
// StageShadow the 001 violation is observed but always approved. Stage-gating,
// not a feature flag, is this package's rollout mechanism.
func TestC1070_004_ShadowStageNeverBlocksTDD(t *testing.T) {
	ws := t.TempDir()
	writeTriage(t, ws) // empty top_n
	writeTestReport(t, ws, "declined-slug", "go/acs/cycle660/predicates_test.go")

	if res := reviewTDD(t, config.StageShadow, ws); !res.Approve {
		t.Errorf("StageShadow must observe-and-approve, never block; got Approve=false reason=%q", res.Reason)
	}
}

// TestC1070_005_BuildSideGateHasNoRegression runs the topngate package's own
// tests under -race in a subprocess: the new TDD gate must not disturb the
// shipped build->audit binding gate (scout AC2).
func TestC1070_005_BuildSideGateHasNoRegression(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", filepath.Join(root, "go"), "test", "-race", "-count=1", "./internal/topngate/...")
	if err != nil || code != 0 {
		t.Errorf("go test -race ./internal/topngate/... must pass (no build-gate regression); code=%d err=%v\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}
}
