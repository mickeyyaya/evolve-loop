package core

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

const briefLowFinding = "\n### L1 (LOW) — a comment restates the code\nb\n"

func writeAuditReportWithDefects(t *testing.T, ws, findings string, defects ...string) {
	t.Helper()
	sentinel := phasecontract.RenderVerdictSentinelWithFailure("audit", "FAIL",
		&phasecontract.FailureBlock{Class: "code-audit-fail", Defects: defects})
	writeBriefFixture(t, ws, phasecontract.ArtifactFilename(string(PhaseAudit)), findings+"\n"+sentinel+"\n")
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
	withBlock, plain := t.TempDir(), t.TempDir()
	for _, ws := range []string{withBlock, plain} {
		writeAuditFailReason(t, ws, "audit", "EGPS: acs-verdict.json ship_eligible=false")
	}
	writeAuditReportWithDefects(t, withBlock, briefRound1, "go/internal/decisionsample/sample.go:14 exports Pick with no production caller")
	writeBriefFixture(t, plain, phasecontract.ArtifactFilename(string(PhaseAudit)), briefRound1)
	cs := CycleState{AuditRepairActive: true, AuditRepairAttempts: 1, AuditDispatches: 1}

	cs.WorkspacePath = withBlock
	got := composeRepairBrief(cs)
	cs.WorkspacePath = plain
	want := composeRepairBrief(cs)

	if got != want {
		t.Fatalf("a runner-downgraded FAIL's brief must be today's (gate reasons, then findings):\ngot:\n%s\nwant:\n%s", got, want)
	}
	if !strings.HasPrefix(got, "failed phase: audit\n- EGPS: acs-verdict.json ship_eligible=false") {
		t.Fatalf("the gate reasons must lead the brief:\n%s", got)
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
}

func TestComposeRepairBrief_AFailureBlockEveryDefectOfWhichIsBriefedAddsNoSection(t *testing.T) {
	ws := t.TempDir()
	writeAuditReportWithDefects(t, ws, briefRound1, "M1: explanation names an area not in the diff")

	brief, _ := composeQuietly(t, CycleState{WorkspacePath: ws, AuditRepairActive: true, AuditRepairAttempts: 1, AuditDispatches: 1})

	if strings.Contains(brief, "audit defects") || !strings.HasPrefix(brief, "auditor findings (audit round 1") {
		t.Fatalf("with every defect already briefed, the brief must open on the findings with no empty defects header:\n%s", brief)
	}
}
