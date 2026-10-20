package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const tSkillsDrift1841 = "skill projection drift: 3 artifact(s) stale vs their SSOTs — CI TestSkills_NoDrift would FAIL. Run `evolve skills generate`."

func writeBuildHandoff(t *testing.T, dir, verdict, class string, defects ...string) {
	t.Helper()
	sent := map[string]any{"phase": "build", "verdict": verdict, "schema_version": 2}
	if class != "" {
		sent["failure"] = map[string]any{"class": class, "defects": defects}
	}
	raw, err := json.Marshal(sent)
	if err != nil {
		t.Fatal(err)
	}
	body := "## Changes\n- none\n<!-- evolve-verdict: " + string(raw) + " -->\n"
	if err := os.WriteFile(filepath.Join(dir, "build-report.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func cycle1841(t *testing.T, buildVerdict, buildClass string) CycleState {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "audit-report.md"), []byte("## Verdict\nPASS\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\",\"schema_version\":1} -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeBuildHandoff(t, dir, buildVerdict, buildClass, "the skills-drift gate reds on a generator the worktree cannot run; no source change repairs it")
	return CycleState{CycleID: 1841, RunID: "run-1841", WorkspacePath: dir, AuditRepairAttempts: 1,
		AuditFailReasons: []string{"verdict-conflict: auditor narrative=PASS but 1 deterministic gate(s) forced FAIL [skills-drift] — the gate outranks the narrative", tSkillsDrift1841}}
}

const tGofmt1841 = "gofmt: 1 file(s) unformatted: go/internal/x/x.go"

func auditedTrees(trees ...string) *fakeLedger {
	l := &fakeLedger{entries: []LedgerEntry{{Role: "builder", Kind: "agent_subprocess", RunID: "run-1841", WorktreeTreeSHA: "b0"}}}
	for _, tree := range trees {
		l.entries = append(l.entries, LedgerEntry{Role: "auditor", Kind: "agent_subprocess", RunID: "run-1841", WorktreeTreeSHA: tree})
	}
	l.entries = append(l.entries, LedgerEntry{Role: "auditor", Kind: "agent_subprocess", RunID: "run-other", WorktreeTreeSHA: "zz"})
	return l
}

func TestDecideAfterAuditFail_ABuildersSystemClassNeverHaltsTheLoop(t *testing.T) {
	t.Parallel()
	for name, reasons := range map[string][]string{
		"an unchanged skills-drift reading (cycle 1841)":  {"verdict-conflict: auditor narrative=PASS but 1 deterministic gate(s) forced FAIL [skills-drift]", tSkillsDrift1841},
		"a gofmt reading the remedy round left unchanged": {"verdict-conflict: auditor narrative=PASS but 1 deterministic gate(s) forced FAIL [gofmt]", tGofmt1841},
	} {
		o := floorOrchestrator(fixedNextStrategy{next: "end"})
		o.ledger = auditedTrees("t1", "t1")
		cs := cycle1841(t, "WARN", "infrastructure-systemic")
		cs.AuditFailReasons = reasons

		next, reason, sig := o.decideAfterAuditFail(cs)
		consumeAuditRepairGrant(&cs, reason)

		if sig != nil || next != PhaseRetro || o.applyFailureDecisionFloor(cs, VerdictFAIL) != nil {
			t.Errorf("%s: next = %s sig = %+v reason %q; want no halt from the builder's word, only a decline to retro", name, next, sig, reason)
		}
		d, defers := o.auditDeclineDefers(cs)
		if !defers || d.fixType != policy.FixTypePipelineRepair || d.source != "build" || d.category != policy.CategoryInfraSystemic {
			t.Errorf("%s: defer = %+v %t; want a pipeline-repair defer of the item that costs one lane", name, d, defers)
		}
	}
}

func TestDecideAfterAuditFail_AChangedAuditedTreeKeepsTheRepairRound(t *testing.T) {
	t.Parallel()
	o := floorOrchestrator(fixedNextStrategy{next: "end"})
	o.ledger = auditedTrees("t1", "t2")
	cs := cycle1841(t, "WARN", "infrastructure-systemic")

	next, reason, sig := o.decideAfterAuditFail(cs)

	d := o.failureDossier(cs, VerdictFAIL)
	if sig != nil || next != PhaseBuild || d.BuildDeclared != nil {
		t.Errorf("next = %s sig = %+v reason %q build_declared %+v; want the gate remediation round: the remedy round moved the tree, so the claim is not corroborated", next, sig, reason, d.BuildDeclared)
	}
}

func TestAuditedTreeUnchanged_ComparesTheLastTwoAuditsOfTheRun(t *testing.T) {
	t.Parallel()
	o := floorOrchestrator(nil)
	for name, c := range map[string]struct {
		ledger Ledger
		want   bool
	}{
		"two equal trees":       {auditedTrees("t0", "t1", "t1"), true},
		"two different trees":   {auditedTrees("t1", "t2"), false},
		"one audit":             {auditedTrees("t1"), false},
		"an empty tree":         {auditedTrees("", ""), false},
		"a ledger with no iter": {&iterlessLedger{}, false},
		"no ledger":             {nil, false},
	} {
		o.ledger = c.ledger
		if got := o.auditedTreeUnchanged("run-1841"); got != c.want {
			t.Errorf("%s: auditedTreeUnchanged = %t, want %t", name, got, c.want)
		}
	}
}

type iterlessLedger struct{ fakeLedger }

func (*iterlessLedger) Iter(context.Context) (LedgerIterator, error) {
	return nil, errors.New("no iterator")
}

func TestFailureDossier_TodaysDossierHoldsWithoutTheShape(t *testing.T) {
	t.Parallel()
	o := floorOrchestrator(nil)
	o.ledger = auditedTrees("t1", "t1")
	noClass := cycle1841(t, "WARN", "")
	taskClass := cycle1841(t, "WARN", "code-build-fail")
	auditFail := cycle1841(t, "WARN", "infrastructure-systemic")
	writeAuditWithFailure(t, auditFail.WorkspacePath, "FAIL", "code-audit-fail", "H1 the change is wrong")
	auditFail.AuditFailReasons = []string{"H1 the change is wrong"}
	passingBuild := cycle1841(t, "PASS", "infrastructure-systemic")
	noReport := cycle1841(t, "WARN", "infrastructure-systemic")
	if err := os.Remove(filepath.Join(noReport.WorkspacePath, "build-report.md")); err != nil {
		t.Fatal(err)
	}
	beforeTheRemedy := cycle1841(t, "WARN", "infrastructure-systemic")
	beforeTheRemedy.AuditRepairAttempts = 0

	for name, cs := range map[string]CycleState{"a builder WARN with no class": noClass, "a task-level builder class": taskClass, "a FAIL narrative of the auditor": auditFail, "a claim before the gate remedy round ran (the cycle-1828 shape)": beforeTheRemedy, "a PASS build that carries a block": passingBuild, "no build report": noReport} {
		d := o.failureDossier(cs, VerdictFAIL)
		got, _ := json.Marshal(d)
		if d.FloorCandidate != "" || d.BuildDeclared != nil {
			t.Errorf("%s: dossier %s, want today's dossier with no floor and no build_declared", name, got)
		}
	}
}

func TestFailureDossier_ANonFloorSystemClassIsRecordedAndIsNoFloor(t *testing.T) {
	t.Parallel()
	o := floorOrchestrator(nil)
	o.ledger = auditedTrees("t1", "t1")
	cs := cycle1841(t, "FAIL", "exit-transport-hang")

	d := o.failureDossier(cs, VerdictFAIL)

	if d.FloorCandidate != "" || d.BuildDeclared == nil || d.BuildDeclared.Class != policy.CategoryTransportHang {
		t.Errorf("dossier = %+v (build %+v), want transport-hang recorded as build_declared and no floor candidate", d, d.BuildDeclared)
	}
}
