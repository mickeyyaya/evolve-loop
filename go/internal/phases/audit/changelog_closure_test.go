package audit

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// Two levels, both required: closureClaimOffenders(text) is the content rule,
// line-scoped; hooks.Classify is the wiring. A gate reachable only from a unit
// test is dead code, so every acceptance case below reaches the rule through
// the real audit verdict seam.
//
// Detection rule pinned by this contract (case-insensitive, per line):
//
//	claim   := line contains "verified closed"
//	           OR (line contains "closed" AND line references cycle-<digits>)
//	cited   := THE SAME line contains "defect-dispositions.json"
//	           or "defect-ledger.json"
//	offender := claim AND NOT cited

// closureReport renders a narrative-PASS audit artifact whose evolve-verdict
// sentinel declares PASS, with body as the bookkeeping prose. Same artifact
// shape extractAuditVerdict already parses, so the gate reads real production
// input rather than a test-only side channel.
func closureReport(body string) string {
	return "# Audit Report\n\n## Verdict\n**PASS**\n\n" + body + "\n\n" +
		`<!-- evolve-verdict: {"phase":"audit","verdict":"PASS","schema_version":1} -->` + "\n"
}

// closureDiags flattens diagnostics so a criterion can assert the gap is NAMED
// rather than merely that some diagnostic exists.
func closureDiags(diags []core.Diagnostic) string {
	var b strings.Builder
	for _, d := range diags {
		b.WriteString(d.Severity)
		b.WriteString(": ")
		b.WriteString(d.Message)
		b.WriteString("\n")
	}
	return b.String()
}

// classifyClosure drives the REAL Classify seam over a report body and returns
// the verdict plus the flattened diagnostics.
func classifyClosure(t *testing.T, body string) (string, string) {
	t.Helper()
	ws := t.TempDir()
	yes := true
	writeACSVerdictShip(t, ws, 0, &yes)
	verdict, diags, _ := hooks{}.Classify(
		closureReport(body),
		core.PhaseRequest{Cycle: 1285, Workspace: ws, ProjectRoot: t.TempDir()},
		core.BridgeResponse{},
	)
	return verdict, closureDiags(diags)
}

// TestC1285_401_ClassifyBlocksUncitedClosureClaim: a prior cycle's CRITICAL
// declared closed with nothing to check the claim against must not be able to
// ride out on PASS.
func TestC1285_401_ClassifyBlocksUncitedClosureClaim(t *testing.T) {
	verdict, diags := classifyClosure(t,
		"## Bookkeeping\n\nThe CRITICAL defect raised by cycle-1272 is verified closed.")

	if verdict == core.VerdictPASS {
		t.Errorf("verdict = PASS; an unevidenced closure claim must not PASS — this is the 1272 laundering shape.\ndiagnostics:\n%s", diags)
	}
	if !strings.Contains(diags, "defect-dispositions.json") {
		t.Errorf("diagnostics must name the artifact the claim has to cite, so the operator knows the remedy; got:\n%s", diags)
	}
	if !strings.Contains(strings.ToLower(diags), "verified closed") {
		t.Errorf("diagnostics must quote the offending claim, not merely report a count — an unnamed offender is unactionable; got:\n%s", diags)
	}
}

// TestC1285_402_ClassifyAllowsCitedClosureClaim — the POSITIVE half. A claim
// that points at the per-defect disposition record is exactly the behavior the
// gate exists to require, so it must ship cleanly. A gate that blocks both
// shapes is a gate nobody can satisfy.
func TestC1285_402_ClassifyAllowsCitedClosureClaim(t *testing.T) {
	verdict, diags := classifyClosure(t,
		"## Bookkeeping\n\nThe CRITICAL defect raised by cycle-1272 is verified closed "+
			"(per .evolve/runs/cycle-1272/defect-dispositions.json, entry d1 FIXED).")

	if verdict != core.VerdictPASS {
		t.Errorf("verdict = %q, want PASS — a cited closure claim is the compliant shape.\ndiagnostics:\n%s", verdict, diags)
	}
	if strings.Contains(diags, "closure claim") {
		t.Errorf("a cited claim must raise no closure diagnostic; got:\n%s", diags)
	}
}

// TestC1285_403_OrdinaryReportUnaffected — no-false-positive floor. The
// overwhelming majority of audit reports make no closure claim at all; the gate
// must be invisible to them or it blocks the green path.
func TestC1285_403_OrdinaryReportUnaffected(t *testing.T) {
	verdict, diags := classifyClosure(t,
		"## Findings\n\nAll acceptance criteria are met. The build is clean and the "+
			"predicate suite is green; no defects were carried in from an earlier cycle.")

	if verdict != core.VerdictPASS {
		t.Errorf("verdict = %q, want PASS — an ordinary report must be unperturbed by the closure gate.\ndiagnostics:\n%s", verdict, diags)
	}
}

// TestC1285_404_ClosureOffendersAreLineScoped pins the content rule directly,
// including the loophole that matters most: a citation somewhere ELSE in the
// document does not vouch for an uncited claim line.
func TestC1285_404_ClosureOffendersAreLineScoped(t *testing.T) {
	cases := []struct {
		name string
		text string
		want int
	}{
		{
			name: "uncited verified-closed claim",
			text: "cycle-1272's CRITICAL is verified closed.",
			want: 1,
		},
		{
			name: "cited on the same line",
			text: "cycle-1272's CRITICAL is verified closed — see .evolve/runs/cycle-1272/defect-dispositions.json.",
			want: 0,
		},
		{
			name: "citation on a DIFFERENT line does not vouch",
			text: "We read .evolve/runs/cycle-1272/defect-dispositions.json.\n" +
				"cycle-1272's CRITICAL is verified closed.",
			want: 1,
		},
		{
			name: "closed + cycle reference is a claim even without the exact phrase",
			text: "The cycle-1255 stale-worktree defect is closed.",
			want: 1,
		},
		{
			name: "closed with no cycle reference is not a closure claim",
			text: "The file handle is closed in the deferred cleanup.",
			want: 0,
		},
		{
			name: "ledger artifact also counts as a citation",
			text: "cycle-1272's CRITICAL is verified closed (defect-ledger.json entry d1).",
			want: 0,
		},
		{
			name: "case-insensitive",
			text: "Cycle-1272's CRITICAL is VERIFIED CLOSED.",
			want: 1,
		},
		{
			name: "empty text",
			text: "",
			want: 0,
		},
		{
			name: "two uncited claims are both named",
			text: "cycle-1255's D1 is verified closed.\ncycle-1268's D2 is verified closed.",
			want: 2,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := closureClaimOffenders(tc.text)
			if len(got) != tc.want {
				t.Errorf("closureClaimOffenders() returned %d offender(s), want %d: %v", len(got), tc.want, got)
			}
		})
	}
}
