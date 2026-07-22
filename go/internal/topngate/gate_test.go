// Package topngate implements the build->audit BLOCKING gate that enforces
// Builder task-slug binding to triage-report.md's ## top_n (inbox
// builder-task-binding-topn-gate, weight 0.96, 7th recurrence: cycles 282,
// 310, 522, 575, 577, 599, 640). This file drives topNBindingGate.check
// directly (white-box, same package — mirrors internal/evalgate/gates_test.go
// for materializationGate).
package topngate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// writeTriageReport writes a triage-report.md ## top_n section listing the
// given slugs (prose form, matching agents/evolve-triage.md Step 4's real
// output shape).
func writeTriageReport(t *testing.T, workspace string, topN ...string) {
	t.Helper()
	var b strings.Builder
	b.WriteString("## top_n (commit to THIS cycle)\n")
	for _, s := range topN {
		b.WriteString("- " + s + ": placeholder description — priority=H, evidence=x, source=inbox\n")
	}
	b.WriteString("\n## deferred (carry to NEXT cycle's carryoverTodos)\n")
	if err := os.WriteFile(filepath.Join(workspace, "triage-report.md"), []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write triage-report: %v", err)
	}
}

// writeBuildReport writes a build-report.md whose ## Task: line claims
// claimedSlug (matching agents/evolve-builder.md's contracted header shape).
func writeBuildReport(t *testing.T, workspace, claimedSlug string) {
	t.Helper()
	body := "# Build Report\n\n## Task: " + claimedSlug + "\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(workspace, "build-report.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write build-report: %v", err)
	}
}

func TestTopNBindingGate(t *testing.T) {
	t.Run("in-lane slug passes", func(t *testing.T) {
		ws := t.TempDir()
		writeTriageReport(t, ws, "statefile-rmw-flock-single-source")
		writeBuildReport(t, ws, "statefile-rmw-flock-single-source")
		reason, block := topNBindingGate{}.check(core.ReviewInput{Phase: string(core.PhaseBuild), Workspace: ws})
		if reason != "" || block {
			t.Errorf("in-lane slug must pass; got reason=%q block=%v", reason, block)
		}
	})

	t.Run("out-of-lane slug is ADVISORY: WARN + pass (2026-07-22)", func(t *testing.T) {
		// POLICY CHANGE (operator-directed, cycles 916 + 1012): both recorded
		// fatal rejections discarded CORRECT work whose report merely labeled
		// the committed task differently — two LLM strings compared. The lane
		// is plan-driven by construction, so label drift WARNs loudly and the
		// binding authority is the committed set. Scope-based fraud
		// verification is the queued construction-level replacement.
		ws := t.TempDir()
		writeTriageReport(t, ws, "statefile-rmw-flock-single-source")
		writeBuildReport(t, ws, "fix-token-resolver-transcript-source")
		reason, block := topNBindingGate{}.check(core.ReviewInput{Phase: string(core.PhaseBuild), Workspace: ws})
		if block {
			t.Fatalf("label drift must never block; got reason=%q", reason)
		}
		// The reason is POPULATED (block=false) so the reviewer's single
		// structured logf seam emits the advisory — testable, unlike a raw
		// stderr write inside the gate.
		if !strings.Contains(reason, "label drift") || !strings.Contains(reason, "fix-token-resolver-transcript-source") || !strings.Contains(reason, "statefile-rmw-flock-single-source") {
			t.Fatalf("advisory reason must name the drift and both slug sets; got %q", reason)
		}
	})

	t.Run("multiple top_n slugs: any member passes", func(t *testing.T) {
		ws := t.TempDir()
		writeTriageReport(t, ws, "a", "b")
		writeBuildReport(t, ws, "b")
		reason, block := topNBindingGate{}.check(core.ReviewInput{Phase: string(core.PhaseBuild), Workspace: ws})
		if reason != "" || block {
			t.Errorf("member of top_n must pass; got reason=%q block=%v", reason, block)
		}
	})

	t.Run("no triage report → fail-open", func(t *testing.T) {
		ws := t.TempDir()
		writeBuildReport(t, ws, "anything")
		reason, block := topNBindingGate{}.check(core.ReviewInput{Phase: string(core.PhaseBuild), Workspace: ws})
		if reason != "" || block {
			t.Errorf("missing triage-report.md must fail open (ambiguity, not a certain violation); got reason=%q block=%v", reason, block)
		}
	})

	t.Run("no build report → fail-open", func(t *testing.T) {
		ws := t.TempDir()
		writeTriageReport(t, ws, "a")
		reason, block := topNBindingGate{}.check(core.ReviewInput{Phase: string(core.PhaseBuild), Workspace: ws})
		if reason != "" || block {
			t.Errorf("missing build-report.md must fail open; got reason=%q block=%v", reason, block)
		}
	})

	t.Run("empty top_n → fail-open (nothing committed to bind against)", func(t *testing.T) {
		ws := t.TempDir()
		writeTriageReport(t, ws) // no entries
		writeBuildReport(t, ws, "anything")
		reason, block := topNBindingGate{}.check(core.ReviewInput{Phase: string(core.PhaseBuild), Workspace: ws})
		if reason != "" || block {
			t.Errorf("empty top_n must fail open; got reason=%q block=%v", reason, block)
		}
	})
}

func TestTopNBindingGate_AppliesToBuildOnly(t *testing.T) {
	g := topNBindingGate{}
	if !g.appliesTo(string(core.PhaseBuild)) {
		t.Error("must apply to the build phase (reviews build-report.md right after build completes, before audit)")
	}
	for _, p := range []string{
		string(core.PhaseScout), string(core.PhaseTriage), string(core.PhaseTDD),
		string(core.PhaseAudit), string(core.PhaseShip),
	} {
		if g.appliesTo(p) {
			t.Errorf("must NOT apply to phase %q", p)
		}
	}
}

// writeTDDReport writes a test-report.md whose "## Task:" line claims
// claimedSlug and whose "## Handoff to Builder" fenced JSON declares testFiles
// (matching agents/evolve-tdd.md Step 6's contracted deliverable shape).
func writeTDDReport(t *testing.T, workspace, claimedSlug string, testFiles ...string) {
	t.Helper()
	var b strings.Builder
	b.WriteString("# TDD Report\n\n## Task: " + claimedSlug + "\n\n## RED Run Output\n\n```\nFAIL\n```\n\n## Handoff to Builder\n\n```json\n{\"testFiles\": [")
	for i, f := range testFiles {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("\"" + f + "\"")
	}
	b.WriteString("], \"redRunConfirmed\": true}\n```\n")
	if err := os.WriteFile(filepath.Join(workspace, "test-report.md"), []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write test-report: %v", err)
	}
}

// TestTDDScopeGate_LabelDriftIsAdvisory is the cycle-1073 crux. tddScopeGate's
// case 2 (non-empty committed top_n + an authored slug with zero overlap) is
// the SAME "two LLM-authored strings compared for exact equality" defect that
// #348 (cbd088a1) converted to an advisory for the sibling topNBindingGate,
// one phase later in the pipeline. Two recorded false rejections (cycles 916,
// 1012) discarded correct work over a label; the triage->TDD transition carries
// the identical risk and must warn, not block.
func TestTDDScopeGate_LabelDriftIsAdvisory(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "statefile-rmw-flock-single-source")
	writeTDDReport(t, ws, "fix-token-resolver-transcript-source", "go/acs/cycle1073/predicates_test.go")
	reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
	if block {
		t.Fatalf("label drift at triage->TDD must never block (mirrors #348's build-gate fix); got reason=%q", reason)
	}
	// The reason stays POPULATED at block=false so the reviewer's single
	// structured logf seam still emits the advisory (testable, unlike a raw
	// stderr write inside the gate).
	if !strings.Contains(reason, "label drift") {
		t.Errorf("advisory reason must be labelled %q so operators can grep it; got %q", "label drift", reason)
	}
	if !strings.Contains(reason, "fix-token-resolver-transcript-source") {
		t.Errorf("advisory reason must name the CLAIMED slug; got %q", reason)
	}
	if !strings.Contains(reason, "statefile-rmw-flock-single-source") {
		t.Errorf("advisory reason must name the COMMITTED top_n set; got %q", reason)
	}
}

// TestTDDScopeGate_EmptyTopNStillBlocks is the anti-overcorrection guard: case
// 1 (empty committed top_n + a non-empty authored set) is NOT a labelling
// dispute — no committed item exists that the authored files could be a
// differently-labelled response to — so it must stay fatal. A blanket
// "block=false" rewrite of check() would pass the advisory test above and fail
// here.
func TestTDDScopeGate_EmptyTopNStillBlocks(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws) // triage committed nothing
	writeTDDReport(t, ws, "orphan-task", "go/acs/cycle1073/predicates_test.go")
	reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
	if !block {
		t.Fatalf("orphan authoring under an EMPTY top_n must stay a hard block; got reason=%q block=false", reason)
	}
	if !strings.Contains(reason, "orphan-task") || !strings.Contains(reason, "EMPTY") {
		t.Errorf("fatal reason must name the claimed slug and the empty commitment; got %q", reason)
	}
}

func TestTDDScopeGate(t *testing.T) {
	t.Run("in-lane slug passes", func(t *testing.T) {
		ws := t.TempDir()
		writeTriageReport(t, ws, "tdd-topn-scope-gate")
		writeTDDReport(t, ws, "tdd-topn-scope-gate", "go/acs/cycle1073/predicates_test.go")
		reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
		if reason != "" || block {
			t.Errorf("in-lane authoring must pass silently; got reason=%q block=%v", reason, block)
		}
	})

	t.Run("multiple top_n slugs: any member passes", func(t *testing.T) {
		ws := t.TempDir()
		writeTriageReport(t, ws, "a", "b")
		writeTDDReport(t, ws, "b", "go/acs/cycle1073/predicates_test.go")
		reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
		if reason != "" || block {
			t.Errorf("member of top_n must pass; got reason=%q block=%v", reason, block)
		}
	})

	t.Run("no triage report → fail-open", func(t *testing.T) {
		ws := t.TempDir()
		writeTDDReport(t, ws, "anything", "go/acs/cycle1073/predicates_test.go")
		reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
		if reason != "" || block {
			t.Errorf("missing triage-report.md is ambiguity, not a certain violation; got reason=%q block=%v", reason, block)
		}
	})

	t.Run("no test report → fail-open", func(t *testing.T) {
		ws := t.TempDir()
		writeTriageReport(t, ws, "a")
		reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
		if reason != "" || block {
			t.Errorf("missing test-report.md must fail open; got reason=%q block=%v", reason, block)
		}
	})

	t.Run("authored nothing → fail-open no-op PASS", func(t *testing.T) {
		ws := t.TempDir()
		writeTriageReport(t, ws) // empty top_n
		writeTDDReport(t, ws, "orphan-task")
		reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
		if reason != "" || block {
			t.Errorf("an empty authored set is the compliant no-op deliverable; got reason=%q block=%v", reason, block)
		}
	})

	t.Run("unparseable claim with authored files → fail-open", func(t *testing.T) {
		ws := t.TempDir()
		writeTriageReport(t, ws, "committed-slug")
		writeTDDReport(t, ws, "", "go/acs/cycle1073/predicates_test.go")
		reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
		if reason != "" || block {
			t.Errorf("no parseable claim is ambiguous → fail open; got reason=%q block=%v", reason, block)
		}
	})
}

func TestTDDScopeGate_AppliesToTDDOnly(t *testing.T) {
	g := tddScopeGate{}
	if !g.appliesTo(string(core.PhaseTDD)) {
		t.Error("must apply to the tdd phase (reviews test-report.md at the triage->TDD transition)")
	}
	for _, p := range []string{
		string(core.PhaseScout), string(core.PhaseTriage), string(core.PhaseBuild),
		string(core.PhaseAudit), string(core.PhaseShip),
	} {
		if g.appliesTo(p) {
			t.Errorf("must NOT apply to phase %q", p)
		}
	}
}
