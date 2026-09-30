package triage

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
)

func TestTriageClassify_RoutesProtectedSurfaceTopNCard_BraceSyntax(t *testing.T) {
	if !guards.IsProtectedSurface("go/acs/regression/cycle1/predicates_test.go") {
		t.Fatal("pin moved: go/acs/regression/ no longer on ProtectedSurfaceManifest — update this test AND the routing rationale")
	}
	ws := t.TempDir()
	path := writeDecision(t, ws, `{"top_n":[{"id":"acs-regression-tamper"}]}`)
	artifact := "## top_n\n" +
		"- acs-regression-tamper: rewrite a regression predicate — priority=H, " +
		"files={go/acs/regression/cycle1/predicates_test.go;go/internal/foo/foo.go}, source=scout\n"

	verdict, diags, _ := hooks{}.Classify(artifact, core.PhaseRequest{Workspace: ws}, core.BridgeResponse{})

	if verdict != core.VerdictPASS {
		t.Fatalf("verdict = %s, want PASS: a card naming a protected path is routed, not refused", verdict)
	}
	if !diagsContain(diags, "acs-regression-tamper") || !diagsContain(diags, "go/acs/regression/cycle1/predicates_test.go") {
		t.Fatalf("diagnostics must cite the routed id AND path, got: %+v", diags)
	}
	if d := readDecision(t, path); len(d["top_n"].([]any)) != 0 || len(d["escalate_block"].([]any)) != 1 {
		t.Fatalf("the decision commits nothing and escalates the card: %v", d)
	}
}

func TestTriageClassify_RoutesProtectedSurfaceTopNCard_BareSyntax(t *testing.T) {
	if !guards.IsProtectedSurface("go/internal/guards/role.go") {
		t.Fatal("pin moved: go/internal/guards/role.go no longer on ProtectedSurfaceManifest — update this test AND the routing rationale")
	}
	ws := t.TempDir()
	writeDecision(t, ws, `{"top_n":[{"id":"role-gate-fix"}]}`)
	artifact := "## top_n\n" +
		"- role-gate-fix: touch the role gate — priority=H, " +
		"files=go/internal/guards/role.go;go/internal/foo/foo.go, evidence=x, source=scout\n"

	verdict, diags, _ := hooks{}.Classify(artifact, core.PhaseRequest{Workspace: ws}, core.BridgeResponse{})

	if verdict != core.VerdictPASS {
		t.Fatalf("verdict = %s, want PASS for a bare files= card naming a protected path (routed)", verdict)
	}
	if !diagsContain(diags, "role-gate-fix") || !diagsContain(diags, "go/internal/guards/role.go") {
		t.Fatalf("diagnostics must cite the routed id AND path, got: %+v", diags)
	}
}

func TestTriageClassify_RoutesAmongMultipleCards_NamesOffendingIdOnly(t *testing.T) {
	ws := t.TempDir()
	path := writeDecision(t, ws, `{"top_n":[{"id":"innocent-task"},{"id":"binaryguard-bypass"}]}`)
	artifact := "## top_n\n" +
		"- innocent-task: unrelated fix — priority=M, files={go/internal/foo/foo.go}, source=scout\n" +
		"- binaryguard-bypass: edit the binary guard — priority=H, files={go/internal/binaryguard/guard.go}, source=scout\n"

	verdict, diags, _ := hooks{}.Classify(artifact, core.PhaseRequest{Workspace: ws}, core.BridgeResponse{})

	if verdict != core.VerdictPASS {
		t.Fatalf("verdict = %s, want PASS: the innocent card is still committed", verdict)
	}
	if !diagsContain(diags, "binaryguard-bypass") || diagsContain(diags, "innocent-task") {
		t.Fatalf("diagnostics name the routed id and never the innocent sibling, got: %+v", diags)
	}
	d := readDecision(t, path)
	if topN := d["top_n"].([]any); len(topN) != 1 || topN[0].(map[string]any)["id"] != "innocent-task" {
		t.Fatalf("top_n keeps the innocent card only: %v", topN)
	}
}

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
