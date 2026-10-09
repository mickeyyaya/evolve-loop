package audit

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func closureReport(body string) string {
	return "# Audit Report\n\n## Verdict\n**PASS**\n\n" + body + "\n\n" +
		`<!-- evolve-verdict: {"phase":"audit","verdict":"PASS","schema_version":1} -->` + "\n"
}

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

func TestC1285_403_OrdinaryReportUnaffected(t *testing.T) {
	verdict, diags := classifyClosure(t,
		"## Findings\n\nAll acceptance criteria are met. The build is clean and the "+
			"predicate suite is green; no defects were carried in from an earlier cycle.")

	if verdict != core.VerdictPASS {
		t.Errorf("verdict = %q, want PASS — an ordinary report must be unperturbed by the closure gate.\ndiagnostics:\n%s", verdict, diags)
	}
}

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
