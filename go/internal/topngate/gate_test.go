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
