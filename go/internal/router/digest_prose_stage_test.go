package router

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

func stagedWorkspace(t *testing.T, policy string) string {
	t.Helper()
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-4")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if policy != "" {
		if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(policy), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return ws
}

func writeProseReports(t *testing.T, ws string) {
	t.Helper()
	writeFile(t, ws, "scout-report.md", "goal_type: bugfix\ndeliverable_kind: code\n\n## Selected Tasks\n### a\n")
	writeFile(t, ws, "triage-report.md", "cycle_size_estimate: medium\n")
}

func TestDigest_LegacyStageRecordsEachReportProseFallback(t *testing.T) {
	t.Parallel()
	ws := stagedWorkspace(t, "")
	writeProseReports(t, ws)
	sig, err := Digest(ws, []string{"scout", "triage"})
	if err != nil {
		t.Fatal(err)
	}
	if sig.Scout.GoalType != "bugfix" || sig.Triage.CycleSize != "medium" || len(sig.DigestDegraded) != 0 {
		t.Fatalf("legacy keeps the prose headers: %+v %+v %v", sig.Scout, sig.Triage, sig.DigestDegraded)
	}
	want := []ProseFallback{
		{Seat: "scout", Path: filepath.Join(ws, "scout-report.md"), Stage: phasecontract.FlowStageLegacy},
		{Seat: "triage", Path: filepath.Join(ws, "triage-report.md"), Stage: phasecontract.FlowStageLegacy},
	}
	if len(sig.ProseFallbacks) != 2 || sig.ProseFallbacks[0] != want[0] || sig.ProseFallbacks[1] != want[1] {
		t.Fatalf("fallbacks %+v, want %+v", sig.ProseFallbacks, want)
	}
}

func TestDigest_EnforceStageRefusesTheReportProse(t *testing.T) {
	t.Parallel()
	ws := stagedWorkspace(t, `{"isolation":{"i8":{"stage":"enforce"}}}`)
	writeProseReports(t, ws)
	sig, err := Digest(ws, []string{"scout", "triage"})
	if err != nil {
		t.Fatal(err)
	}
	if sig.Scout.Present || sig.Triage.Present {
		t.Fatalf("enforce must not read prose headers: %+v %+v", sig.Scout, sig.Triage)
	}
	if len(sig.ProseFallbacks) != 2 || !sig.ProseFallbacks[0].Refused || !sig.ProseFallbacks[1].Refused {
		t.Fatalf("fallbacks %+v", sig.ProseFallbacks)
	}
	if len(sig.DigestDegraded) != 2 || !strings.Contains(sig.DigestDegraded[0], "refuses the report-prose fallback") {
		t.Fatalf("an enforce refusal degrades the digest: %v", sig.DigestDegraded)
	}
}

func TestDigest_AHandoffOrAnAbsentReportIsNoProseFallback(t *testing.T) {
	t.Parallel()
	ws := stagedWorkspace(t, `{"isolation":{"i8":{"stage":"enforce"}}}`)
	writeFile(t, ws, "handoff-scout.json", `{"goal_type":"feature"}`)
	sig, err := Digest(ws, []string{"scout", "triage"})
	if err != nil {
		t.Fatal(err)
	}
	if len(sig.ProseFallbacks) != 0 || len(sig.DigestDegraded) != 0 {
		t.Fatalf("fallbacks %+v degraded %v", sig.ProseFallbacks, sig.DigestDegraded)
	}
}

func TestDigestFlowStage_IsLegacyOutsideACycleWorkspace(t *testing.T) {
	t.Parallel()
	if got := digestFlowStage(t.TempDir()); got != phasecontract.FlowStageLegacy {
		t.Fatalf("stage %q", got)
	}
	if got := digestFlowStage(stagedWorkspace(t, `{"isolation":{"i8":{"stage":"shadow"}}}`)); got != phasecontract.FlowStageShadow {
		t.Fatalf("stage %q", got)
	}
}
