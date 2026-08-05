package triage

// protected_surface_admission_test.go — RED contract for the second,
// commit-time admission route (F4, docs/operations/batch-integrity-review-
// 2026-08-04.md; inbox item triage-protected-surface-admission).
//
// console_routed_prompt_test.go already pins route #1: guards.IsProtectedSurface
// screens items sourced from .evolve/inbox before they are OFFERED in the
// prompt. That screen never runs for a top_n card the LLM writes from the
// fleet-todo/scout route — a card whose `files={...}` segment names a
// protected path sails through hooks.Classify today (Classify only checks
// non-empty artifact + "## top_n" heading + >=1 list item). Cycles
// 1257/1259/1263 burned on exactly this shape. These tests pin the second,
// independent commit-time check: Classify itself must FAIL a committed
// artifact whose top_n `files=` references guards.IsProtectedSurface.
import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
)

// Negative: a top_n card naming a protected file via the brace-delimited
// `files={a;b;c}` encoding (the format inboxbatch.RenderMarkdown emits, per
// scout-report.md's Key Findings) must FAIL Classify, and the diagnostic
// must name both the offending task id and the offending path so the
// operator can act without re-deriving it from the artifact.
func TestTriageClassify_RejectsProtectedSurfaceTopNCard_BraceSyntax(t *testing.T) {
	if !guards.IsProtectedSurface("go/acs/regression/cycle1/predicates_test.go") {
		t.Fatal("pin moved: go/acs/regression/ no longer on ProtectedSurfaceManifest — update this test AND the routing rationale")
	}
	artifact := "## top_n\n" +
		"- acs-regression-tamper: rewrite a regression predicate — priority=H, " +
		"files={go/acs/regression/cycle1/predicates_test.go;go/internal/foo/foo.go}, source=scout\n"

	verdict, diags, _ := hooks{}.Classify(artifact, core.PhaseRequest{}, core.BridgeResponse{})

	if verdict != core.VerdictFAIL {
		t.Fatalf("verdict = %s, want FAIL for a top_n card naming a protected path", verdict)
	}
	if !diagsContain(diags, "acs-regression-tamper") || !diagsContain(diags, "go/acs/regression/cycle1/predicates_test.go") {
		t.Fatalf("diagnostics must cite the offending id AND path, got: %+v", diags)
	}
}

// Same defect, the bare (unbraced) `files=a;b` encoding real cycle-1312
// output actually used (.evolve/runs/cycle-1312/triage-report.md) — the
// admission check must not silently no-op on the brace-less variant.
func TestTriageClassify_RejectsProtectedSurfaceTopNCard_BareSyntax(t *testing.T) {
	if !guards.IsProtectedSurface("go/internal/guards/role.go") {
		t.Fatal("pin moved: go/internal/guards/role.go no longer on ProtectedSurfaceManifest — update this test AND the routing rationale")
	}
	artifact := "## top_n\n" +
		"- role-gate-fix: touch the role gate — priority=H, " +
		"files=go/internal/guards/role.go;go/internal/foo/foo.go, evidence=x, source=scout\n"

	verdict, diags, _ := hooks{}.Classify(artifact, core.PhaseRequest{}, core.BridgeResponse{})

	if verdict != core.VerdictFAIL {
		t.Fatalf("verdict = %s, want FAIL for a bare files= card naming a protected path", verdict)
	}
	if !diagsContain(diags, "role-gate-fix") || !diagsContain(diags, "go/internal/guards/role.go") {
		t.Fatalf("diagnostics must cite the offending id AND path, got: %+v", diags)
	}
}

// Semantic: among several top_n cards, only the one actually naming a
// protected path must be identified — the diagnostic must not misattribute
// the rejection to an innocent sibling card.
func TestTriageClassify_RejectsAmongMultipleCards_NamesOffendingIdOnly(t *testing.T) {
	artifact := "## top_n\n" +
		"- innocent-task: unrelated fix — priority=M, files={go/internal/foo/foo.go}, source=scout\n" +
		"- binaryguard-bypass: edit the binary guard — priority=H, files={go/internal/binaryguard/guard.go}, source=scout\n"

	verdict, diags, _ := hooks{}.Classify(artifact, core.PhaseRequest{}, core.BridgeResponse{})

	if verdict != core.VerdictFAIL {
		t.Fatalf("verdict = %s, want FAIL when any card in the batch names a protected path", verdict)
	}
	if !diagsContain(diags, "binaryguard-bypass") {
		t.Fatalf("diagnostics must name the actually-offending id binaryguard-bypass, got: %+v", diags)
	}
	if diagsContain(diags, "innocent-task") {
		t.Fatalf("diagnostics must not misattribute the rejection to the innocent sibling card, got: %+v", diags)
	}
}

// Positive/regression: a top_n card whose files= segment names only
// ordinary, non-manifest paths must still PASS — the new admission check
// must not regress the byte-identical PASS behavior EvaluateClassify already
// gives a well-formed artifact (scout-report.md Acceptance Criteria: "A
// non-protected top_n card is unaffected").
func TestTriageClassify_AllowsNonProtectedTopNCard(t *testing.T) {
	artifact := "## top_n\n" +
		"- add-widget: add a widget — priority=M, files={go/internal/widget/widget.go;go/internal/widget/widget_test.go}, source=scout\n"

	verdict, diags, next := hooks{}.Classify(artifact, core.PhaseRequest{}, core.BridgeResponse{})

	if verdict != core.VerdictPASS {
		t.Fatalf("verdict = %s (diags=%+v), want PASS for a non-protected top_n card", verdict, diags)
	}
	if diags != nil {
		t.Fatalf("diags = %+v, want nil on PASS", diags)
	}
	if next != string(core.PhaseTDD) {
		t.Fatalf("nextPhase = %q, want %q", next, string(core.PhaseTDD))
	}
}

// Edge: a top_n card with no files= segment at all (e.g. a purely narrative
// line) must be unaffected by the new check — nothing to intersect means no
// rejection basis exists.
func TestTriageClassify_NoFilesSegmentIsUnaffected(t *testing.T) {
	artifact := "## top_n\n" +
		"- narrative-only: a card with no files= segment at all\n"

	verdict, _, _ := hooks{}.Classify(artifact, core.PhaseRequest{}, core.BridgeResponse{})

	if verdict != core.VerdictPASS {
		t.Fatalf("verdict = %s, want PASS when no card carries a files= segment", verdict)
	}
}

func diagsContain(diags []core.Diagnostic, substr string) bool {
	for _, d := range diags {
		if strings.Contains(d.Message, substr) {
			return true
		}
	}
	return false
}
