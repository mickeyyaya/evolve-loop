package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

const briefLowFinding = "\n### L1 (LOW) — a comment restates the code\nb\n"

func writeAuditReportWithDefects(t *testing.T, ws, findings string, defects ...string) {
	t.Helper()
	sentinel := phasecontract.RenderVerdictSentinelWithFailure("audit", "FAIL",
		&phasecontract.FailureBlock{Class: "code-audit-fail", Defects: defects})
	writeBriefFixture(t, ws, phasecontract.ArtifactFilename(string(PhaseAudit)), findings+"\n"+sentinel+"\n")
}

func downgradeAudit(t *testing.T, cs *CycleState, reasons ...string) {
	t.Helper()
	diags := make([]Diagnostic, len(reasons))
	for i, reason := range reasons {
		diags[i] = Diagnostic{Severity: cyclestate.SeverityError, Message: reason}
	}
	persistFloorFailReasons(cs, PhaseAudit, diags)
}

func composeQuietly(t *testing.T, cs CycleState) (string, string) {
	t.Helper()
	var brief string
	stderr := captureStderr(t, func() { brief = composeRepairBrief(cs) })
	return brief, stderr
}

func TestComposeRepairBrief_AnAgentGradedFailBriefsItsFailureBlockDefects(t *testing.T) {
	ws := t.TempDir()
	defects := []string{
		"go/internal/decisionsample/sample.go:14 exports Pick with no production caller",
		"go/internal/decisionsample/sample_test.go:30 asserts only that Pick returns non-nil",
	}
	writeAuditReportWithDefects(t, ws, briefRound1, defects...)

	brief, stderr := composeQuietly(t, CycleState{WorkspacePath: ws, AuditRepairActive: true, AuditRepairAttempts: 1, AuditDispatches: 1})

	if strings.Contains(stderr, "unreadable") {
		t.Fatalf("an agent-graded FAIL writes no gate record; its absence must not WARN:\n%s", stderr)
	}
	findings := strings.Index(brief, "auditor findings (audit round 1")
	for _, d := range defects {
		at := strings.Index(brief, "- "+d)
		if at < 0 || findings < 0 || at > findings {
			t.Fatalf("the brief must carry the failure block's defect %q ahead of the findings table:\n%s", d, brief)
		}
	}
	if !strings.Contains(brief, "code-audit-fail") {
		t.Errorf("the defects section must name the auditor's declared class:\n%s", brief)
	}
}

func TestComposeRepairBrief_ADefectTheBriefedFindingsCarryIsNotRepeated(t *testing.T) {
	ws := t.TempDir()
	writeAuditReportWithDefects(t, ws, briefRound1+briefLowFinding,
		"H1 (HIGH) — caller-proof hard floor violated: decisionsample exports have no callers",
		"M1: explanation names an area not in the diff",
		"L1 a comment restates the code",
		"go/internal/decisionsample/sample.go:14 exports Pick with no production caller",
	)

	brief, _ := composeQuietly(t, CycleState{WorkspacePath: ws, AuditRepairActive: true, AuditRepairAttempts: 1, AuditDispatches: 1})

	for _, title := range []string{"caller-proof hard floor violated", "explanation names an area not in the diff"} {
		if n := strings.Count(brief, title); n != 1 {
			t.Errorf("%q appears %d times; a defect restating a briefed finding must be dropped:\n%s", title, n, brief)
		}
	}
	if !strings.Contains(brief, "- L1 a comment restates the code") {
		t.Errorf("a LOW finding is not briefed, so the defect restating it must stay:\n%s", brief)
	}
	if !strings.Contains(brief, "- go/internal/decisionsample/sample.go:14 exports Pick") {
		t.Errorf("a defect no finding carries must stay:\n%s", brief)
	}
}

func TestComposeRepairBrief_AGateRecordKeepsTodaysBriefOverTheFailureBlock(t *testing.T) {
	ws := t.TempDir()
	writeAuditReportWithDefects(t, ws, briefRound1, "go/internal/decisionsample/sample.go:14 exports Pick with no production caller")
	cs := CycleState{WorkspacePath: ws, AuditRepairActive: true, AuditRepairAttempts: 1, AuditDispatches: 1}
	downgradeAudit(t, &cs, "EGPS: acs-verdict.json ship_eligible=false")

	got := composeRepairBrief(cs)

	want := "failed phase: audit\n- EGPS: acs-verdict.json ship_eligible=false\n\n" +
		"auditor findings (audit round 1 — fix THESE; the gate reasons above are their symptoms):\n" +
		"- H1 (HIGH) — caller-proof hard floor violated: decisionsample exports have no callers\n" +
		"- M1 (MEDIUM) — explanation names an area not in the diff"
	if got != want {
		t.Fatalf("a runner-downgraded FAIL's brief must be today's, byte for byte (gate reasons, then findings; no failure block):\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestRepairRoundDispatch_AnAgentGradedFailBriefsTheDefectsWithoutAWarn(t *testing.T) {
	runners := buildRunners(map[Phase]string{PhaseRetro: VerdictFAIL})
	ar := &findingsAuditRunner{t: t}
	runners[PhaseAudit] = ar
	o := NewOrchestrator(&fakeStorage{state: State{LastCycleNumber: 0}}, &fakeLedger{}, runners)

	stderr := captureStderr(t, func() {
		if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()}); err != nil {
			t.Errorf("RunCycle: %v", err)
		}
	})

	for _, line := range strings.Split(stderr, "\n") {
		if strings.Contains(line, "findings artifact") {
			t.Fatalf("the repair round WARNed about the absent gate record:\n%s", line)
		}
	}
	for _, p := range []Phase{PhaseTDD, PhaseBuild} {
		reqs := runners[p].(*fakeRunner).requests
		if len(reqs) < 2 {
			t.Fatalf("%s dispatched %d time(s); the repair round never re-dispatched it", p, len(reqs))
		}
		if brief := reqs[len(reqs)-1].Context[CtxKeyAuditRepairFindings]; !strings.Contains(brief, "- H1 caller-proof hard floor violated\n") {
			t.Errorf("the repair-round %s was not briefed with the failure block's defect:\n%s", p, brief)
		}
	}
}

func TestComposeRepairBrief_TheDedupeCoversExactlyTheBriefedFindings(t *testing.T) {
	var issues strings.Builder
	issues.WriteString("# Audit Report\n\n## Verdict\n\n**FAIL**\n\n## Issues\n")
	for i := 1; i <= maxBriefFindings+1; i++ {
		fmt.Fprintf(&issues, "\n### H%d (HIGH) — defect number %d is unaddressed\nb\n", i, i)
	}
	ws := t.TempDir()
	writeAuditReportWithDefects(t, ws, issues.String(),
		"defect number 1 is unaddressed",
		"h2 DEFECT NUMBER 2 IS UNADDRESSED",
		fmt.Sprintf("H%d defect number %d is unaddressed", maxBriefFindings+1, maxBriefFindings+1),
	)

	brief, _ := composeQuietly(t, CycleState{WorkspacePath: ws, AuditRepairActive: true, AuditRepairAttempts: 1, AuditDispatches: 1})

	for _, restated := range []string{"- defect number 1 is unaddressed", "- h2 DEFECT NUMBER 2"} {
		if strings.Contains(brief, restated) {
			t.Errorf("%q restates a briefed finding (bare title; id and title in another case) and must be dropped:\n%s", restated, brief)
		}
	}
	beyond := fmt.Sprintf("- H%d defect number %d is unaddressed", maxBriefFindings+1, maxBriefFindings+1)
	if !strings.Contains(brief, beyond) {
		t.Errorf("finding %d is past the brief's %d, so the defect restating it must stay:\n%s", maxBriefFindings+1, maxBriefFindings, brief)
	}
	if !strings.Contains(brief, "- … 1 more finding(s) in audit-report.md") || strings.Contains(brief, fmt.Sprintf("H%d (HIGH)", maxBriefFindings+1)) {
		t.Errorf("the brief lists %d findings and counts the rest:\n%s", maxBriefFindings, brief)
	}
}

func TestComposeRepairBrief_AFailureBlockEveryDefectOfWhichIsBriefedAddsNoSection(t *testing.T) {
	ws := t.TempDir()
	writeAuditReportWithDefects(t, ws, briefRound1, "M1: explanation names an area not in the diff")

	brief, _ := composeQuietly(t, CycleState{WorkspacePath: ws, AuditRepairActive: true, AuditRepairAttempts: 1, AuditDispatches: 1})

	if strings.Contains(brief, "audit defects") || !strings.HasPrefix(brief, "auditor findings (audit round 1") {
		t.Fatalf("with every defect already briefed, the brief must open on the findings with no empty defects header:\n%s", brief)
	}
}

func TestComposeRepairBrief_TheRunnersReasonsNotTheWorkspaceFileDecideTheGateSlot(t *testing.T) {
	defect := "go/internal/decisionsample/sample.go:14 exports Pick with no production caller"
	stale, unreadable, unwritten := t.TempDir(), t.TempDir(), t.TempDir()
	writeAuditFailReason(t, stale, "build", agentGradedRouterReason("build"))
	if err := os.Mkdir(filepath.Join(unreadable, "audit-fail-reason.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, ws := range []string{stale, unreadable, unwritten} {
		writeAuditReportWithDefects(t, ws, briefRound1, defect)
	}
	cs := CycleState{AuditRepairActive: true, AuditRepairAttempts: 1, AuditDispatches: 1}

	for name, ws := range map[string]string{"another phase's leftover record": stale, "a record that cannot be read": unreadable} {
		cs.WorkspacePath = ws
		brief, stderr := composeQuietly(t, cs)
		if strings.Contains(stderr, "unreadable") || !strings.Contains(brief, "- "+defect) || strings.Contains(brief, "failed phase") {
			t.Errorf("%s with no runner diagnosis must brief the failure block, quietly:\nstderr=%q\n%s", name, stderr, brief)
		}
	}
	cs.WorkspacePath, cs.AuditFailReasons = unwritten, []string{"EGPS: acs-verdict.json ship_eligible=false"}
	if brief, _ := composeQuietly(t, cs); !strings.HasPrefix(brief, "failed phase: audit\n- EGPS: acs-verdict.json ship_eligible=false\n\n") || strings.Contains(brief, defect) {
		t.Errorf("the runner's reasons lead the brief even when their best-effort record was never written:\n%s", brief)
	}
}

func TestComposeRepairBrief_TheFindingsHeaderNamesGateReasonsOnlyWhenTheyLead(t *testing.T) {
	ws := t.TempDir()
	writeAuditReportWithDefects(t, ws, briefRound1, "go/internal/decisionsample/sample.go:14 exports Pick with no production caller")
	cs := CycleState{WorkspacePath: ws, AuditRepairActive: true, AuditRepairAttempts: 1, AuditDispatches: 1}

	for name, brief := range map[string]string{
		"the failure block's defects": composeRepairBrief(cs),
		"the standing findings":       auditorFindingsBrief(ws, 1),
	} {
		if !strings.Contains(brief, "auditor findings (audit round 1 — fix THESE):") || strings.Contains(brief, "gate reasons") {
			t.Errorf("with %s the header must not call anything above it gate reasons or symptoms:\n%s", name, brief)
		}
	}
}
