package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func rereviewWorkflow(stage policy.AuditRereviewStage) policy.WorkflowConfig {
	wf := policy.Policy{}.WorkflowConfig()
	wf.Convergence.AuditRereview = stage
	return wf
}

func (f *scopeFixture) roundTwoReady(t *testing.T) {
	t.Helper()
	f.build(t)
	writeFile(t, filepath.Join(f.ws, "audit-report.md"), round1Report)
	f.o.emitPhaseBindings(context.Background(), scopeCycle, f.repo, f.cycleState(1), PhaseAudit, VerdictFAIL)
	writeFile(t, filepath.Join(f.wt, "a.go"), strings.Replace(numberedLines("old", 30), "old 12\n", "fix 12\n", 1))
}

func TestScopeReachesAuditOnDispatchAndResume(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"dispatch", "resume"} {
		t.Run(path, func(t *testing.T) {
			f := newScopeFixture(t)
			f.roundTwoReady(t)
			st := &fakeStorage{state: State{LastCycleNumber: scopeCycle}, cycleState: CycleState{
				CycleID: scopeCycle, RunID: "run", WorkspacePath: f.ws, ActiveWorktree: f.wt, WorktreeBaseSHA: gitHead(f.repo),
				AuditDispatches: 1, CompletedPhases: []string{"scout", "triage", "build", "audit"},
			}}
			o := NewOrchestrator(st, f.led, buildRunners(nil), WithWorkflowConfig(rereviewWorkflow(policy.AuditRereviewEnforce)))
			req := CycleRequest{ProjectRoot: f.repo, GoalHash: "goal"}
			ctx := context.Background()
			if path == "dispatch" {
				cr := &cycleRun{o: o, ctx: ctx, req: req, cs: st.cycleState, cycle: scopeCycle, ctxSnap: map[string]string{}, retryConfig: o.retryConfig, workflowConfig: o.workflowConfig}
				if _, _, err := cr.dispatch(PhaseAudit); err != nil {
					t.Fatalf("dispatch: %v", err)
				}
			} else if _, err := o.RunCycleFromPhase(ctx, req, &ResumePoint{Phase: string(PhaseAudit), CycleID: scopeCycle}); err != nil {
				t.Fatalf("resume: %v", err)
			}

			got := o.runners[PhaseAudit].(*fakeRunner).requests[0]
			if got.AuditRound != 2 || got.AuditScope == nil {
				t.Fatalf("audit request round %d scope %+v, want round 2 with the repair scope", got.AuditRound, got.AuditScope)
			}
			if !got.AuditScope.Enforce || len(got.AuditScope.Hunks) != 1 || got.AuditScope.Hunks[0].From != 12 {
				t.Fatalf("scope = %+v, want one hunk at line 12 and enforce", got.AuditScope)
			}
			if _, err := os.Stat(filepath.Join(f.ws, phasecontract.RoundArchiveFilename("audit-scope.json", 2))); err != nil {
				t.Fatalf("the scope is not persisted for the post-audit decision: %v", err)
			}
			for _, phase := range []Phase{PhaseShip} {
				if r, ok := o.runners[phase].(*fakeRunner); ok && len(r.requests) > 0 && r.requests[0].AuditScope != nil {
					t.Errorf("%s request carries an audit scope", phase)
				}
			}
		})
	}
}

func TestScopeIsRound1WhenUnavailableAndSignalsIt(t *testing.T) {
	t.Parallel()
	f := newScopeFixture(t)
	center, events := captureSignals()
	f.o = NewOrchestrator(nil, f.led, nil, WithSignalCenter(center))
	cs := f.cycleState(2)

	if got := f.o.auditScopeFor(context.Background(), PhaseAudit, cs, f.repo); got != nil {
		t.Fatalf("scope = %+v from an empty ledger", got)
	}
	if got := f.o.auditScopeFor(context.Background(), PhaseBuild, cs, f.repo); got != nil {
		t.Fatalf("a non-audit phase got a scope: %+v", got)
	}
	cs.AuditDispatches = 1
	if got := f.o.auditScopeFor(context.Background(), PhaseAudit, cs, f.repo); got != nil {
		t.Fatalf("round 1 got a scope: %+v", got)
	}
	if len(*events) != 1 || (*events)[0].Code != CodeAuditRereviewNoScope || (*events)[0].Fields["round"] != "2" || (*events)[0].Fields["reason"] == "" {
		t.Fatalf("signals = %+v, want exactly one AUDIT_REREVIEW_NO_SCOPE for round 2 naming the reason", *events)
	}
}

func rereviewRound2(t *testing.T, stage policy.AuditRereviewStage, report string) (*scopeFixture, *[]signalcenter.Event) {
	t.Helper()
	f := newScopeFixture(t)
	f.roundTwoReady(t)
	cs := f.cycleState(1)
	f.o.workflowConfig = rereviewWorkflow(stage)
	center, events := captureSignals()
	f.o.signals = center
	rename := func() {
		if err := os.Rename(filepath.Join(f.ws, "audit-report.md"), filepath.Join(f.ws, phasecontract.RoundArchiveFilename("audit-report.md", 1))); err != nil {
			t.Fatal(err)
		}
	}
	rename()
	_ = cs
	if scope := f.o.auditScopeFor(context.Background(), PhaseAudit, f.cycleState(2), f.repo); scope == nil {
		t.Fatal("round 2 has no scope")
	}
	writeFile(t, filepath.Join(f.ws, "audit-report.md"), report)
	*events = nil
	return f, events
}

const lateReport = `## Verdict
FAIL

## Issues
### H1 (HIGH) — the loop swallows the error at a.go:3
### M2 (MEDIUM) — a fresh defect in untouched code at a.go:20
### M3 (MEDIUM) — reworded copy of the old defect at a.go:3
### H4 (HIGH) — a fresh serious defect in untouched code at a.go:25
### M5 (MEDIUM) — a defect inside the repair at a.go:12
### M6 (MEDIUM) — a second fresh defect in untouched code at a.go:21
`

func TestShadowRecordsWhatEnforceWouldDoAndFilesNothing(t *testing.T) {
	t.Parallel()
	f, events := rereviewRound2(t, policy.AuditRereviewShadow, lateReport)

	f.o.emitPhaseBindings(context.Background(), scopeCycle, f.repo, f.cycleState(2), PhaseAudit, VerdictFAIL)

	var shadow []signalcenter.Event
	for _, e := range *events {
		if e.Code == CodeAuditRereviewShadow {
			shadow = append(shadow, e)
		}
	}
	if len(shadow) != 1 {
		t.Fatalf("AUDIT_REREVIEW_SHADOW events = %d, want 1: %+v", len(shadow), *events)
	}
	want := map[string]string{"stage": "shadow", "round": "2", "hunks": "1", "late": "3", "would_file": "2", "would_block": "1", "reworded": "1", "filed": "0", "verdict": "FAIL"}
	for k, v := range want {
		if shadow[0].Fields[k] != v {
			t.Errorf("fields[%s] = %q, want %q (all: %v)", k, shadow[0].Fields[k], v, shadow[0].Fields)
		}
	}
	if _, err := os.Stat(filepath.Join(f.ws, AuditLateFindingsFile)); err == nil {
		t.Fatal("shadow wrote a late-findings note")
	}
}

const filedReport = `## Verdict
FAIL

## Issues
### H4 (HIGH) — a fresh serious defect in untouched code at a.go:25

## Filed (late)
### M2 (MEDIUM) — a fresh defect in untouched code at a.go:20
### M5 (MEDIUM) — a defect inside the repair at a.go:12
`

func TestStageActorAtEnforce(t *testing.T) {
	t.Parallel()
	f, events := rereviewRound2(t, policy.AuditRereviewEnforce, filedReport)

	f.o.emitPhaseBindings(context.Background(), scopeCycle, f.repo, f.cycleState(2), PhaseAudit, VerdictFAIL)
	f.o.emitPhaseBindings(context.Background(), scopeCycle, f.repo, f.cycleState(2), PhaseAudit, VerdictFAIL)

	inbox := filepath.Join(f.repo, ".evolve", "inbox")
	items, err := filepath.Glob(filepath.Join(inbox, "*.json"))
	if err != nil || len(items) != 1 {
		t.Fatalf("inbox items = %v (%v), want exactly one deduplicated follow-up for the one valid filed finding", items, err)
	}
	raw, _ := os.ReadFile(items[0])
	var item struct{ ID, Title, Summary string }
	if err := json.Unmarshal(raw, &item); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(item.ID, "audit-late-") || !strings.Contains(item.Title, "a fresh defect in untouched code") || !strings.Contains(item.Summary, "a.go:20") {
		t.Fatalf("follow-up = %+v", item)
	}
	note, err := os.ReadFile(filepath.Join(f.ws, AuditLateFindingsFile))
	if err != nil || !strings.Contains(string(note), "a.go:20") || strings.Contains(string(note), "a.go:12") {
		t.Fatalf("late-findings note = %s (%v), want only the valid entry", note, err)
	}
	last := (*events)[len(*events)-1]
	if last.Code != CodeAuditRereviewShadow || last.Fields["stage"] != "enforce" || last.Fields["filed"] != "1" {
		t.Fatalf("last signal = %+v", last)
	}
}

func TestEnforceFilingFailureKeepsTheFindingInTheNoteAndSignals(t *testing.T) {
	t.Parallel()
	f, events := rereviewRound2(t, policy.AuditRereviewEnforce, filedReport)
	if err := os.MkdirAll(filepath.Join(f.repo, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(f.repo, ".evolve", "inbox"), "a file where the inbox directory belongs\n")

	f.o.emitPhaseBindings(context.Background(), scopeCycle, f.repo, f.cycleState(2), PhaseAudit, VerdictFAIL)

	note, err := os.ReadFile(filepath.Join(f.ws, AuditLateFindingsFile))
	if err != nil || !strings.Contains(string(note), "a.go:20") || !strings.Contains(string(note), "failed") {
		t.Fatalf("note = %s (%v), want the finding kept with the failure", note, err)
	}
	warned := false
	for _, e := range *events {
		warned = warned || (e.Code == CodeAuditRereviewFileFailed && e.Severity == signalcenter.SeverityWarn && strings.Contains(e.Reason, "not filed") && e.Fields["reason"] != "")
	}
	if !warned {
		t.Fatalf("no warning signal for the failed filing: %+v", *events)
	}
}

func TestAuditWorktreeTree_IsTheTreeTheBindingRecords(t *testing.T) {
	t.Parallel()
	f := newScopeFixture(t)
	f.build(t)
	writeFile(t, filepath.Join(f.wt, "a.go"), numberedLines("changed", 30))
	req := PhaseRequest{ProjectRoot: f.repo, Worktree: f.wt, Workspace: f.ws}

	tree := AuditWorktreeTree(context.Background(), req)

	writeFile(t, filepath.Join(f.ws, "audit-report.md"), round1Report)
	f.o.emitPhaseBindings(context.Background(), scopeCycle, f.repo, f.cycleState(1), PhaseAudit, VerdictFAIL)
	if tree == "" || tree != f.led.entries[0].WorktreeTreeSHA {
		t.Fatalf("tree %q, binding %q", tree, f.led.entries[0].WorktreeTreeSHA)
	}
	if got := AuditWorktreeTree(context.Background(), PhaseRequest{ProjectRoot: f.repo, Worktree: f.repo, Workspace: f.ws}); got != "" {
		t.Fatalf("an in-place worktree has tree %q", got)
	}
}

func TestEnforceLateNoteThatCannotBeWrittenIsSignalled(t *testing.T) {
	t.Parallel()
	f, events := rereviewRound2(t, policy.AuditRereviewEnforce, filedReport)
	if err := os.MkdirAll(filepath.Join(f.ws, AuditLateFindingsFile, "blocker"), 0o755); err != nil {
		t.Fatal(err)
	}

	f.o.emitPhaseBindings(context.Background(), scopeCycle, f.repo, f.cycleState(2), PhaseAudit, VerdictFAIL)

	warned := false
	for _, e := range *events {
		warned = warned || (e.Severity == signalcenter.SeverityWarn && strings.Contains(e.Reason, "landing note is not written"))
	}
	if !warned {
		t.Fatalf("no warning for the unwritten note: %+v", *events)
	}
}

func TestShadowStageComputesAScopeThatIsNotEnforced(t *testing.T) {
	t.Parallel()
	f, _ := rereviewRound2(t, policy.AuditRereviewShadow, lateReport)

	scope := f.o.auditScopeFor(context.Background(), PhaseAudit, f.cycleState(2), f.repo)

	if scope == nil || scope.Enforce {
		t.Fatalf("shadow scope = %+v, want a recorded scope with Enforce false", scope)
	}
	f.o.workflowConfig = rereviewWorkflow(policy.AuditRereviewEnforce)
	if on := f.o.auditScopeFor(context.Background(), PhaseAudit, f.cycleState(2), f.repo); on == nil || !on.Enforce {
		t.Fatalf("enforce scope = %+v, want Enforce true", on)
	}
}

func TestShadowFilesNothingEvenWhenTheReportHasFiledEntries(t *testing.T) {
	t.Parallel()
	f, events := rereviewRound2(t, policy.AuditRereviewShadow, filedReport)

	f.o.emitPhaseBindings(context.Background(), scopeCycle, f.repo, f.cycleState(2), PhaseAudit, VerdictFAIL)

	items, err := filepath.Glob(filepath.Join(f.repo, ".evolve", "inbox", "*.json"))
	if err != nil || len(items) != 0 {
		t.Fatalf("shadow minted inbox items %v (%v)", items, err)
	}
	if _, err := os.Stat(filepath.Join(f.ws, AuditLateFindingsFile)); err == nil {
		t.Fatal("shadow wrote a late-findings note")
	}
	for _, e := range *events {
		if e.Code == CodeAuditRereviewShadow && e.Fields["filed"] != "0" {
			t.Fatalf("shadow reports filed=%q", e.Fields["filed"])
		}
	}
}

func TestCountRereview_SplitsLateFindingsByTheBarWithAsymmetricCounts(t *testing.T) {
	t.Parallel()
	scope := &AuditRoundScope{Hunks: []convergence.Hunk{{File: "a.go", From: 10, To: 12}}}

	got := countRereview(scope, []reportdoc.Finding{
		{Title: "x", Severity: "MEDIUM", Location: "a.go:20"},
		{Title: "y", Severity: "LOW", Location: "a.go:21"},
		{Title: "z", Severity: "HIGH", Location: "a.go:25"},
	})

	if got.late != 3 || got.wouldFile != 2 || got.wouldBlock != 1 {
		t.Fatalf("counts = %+v, want late 3, would_file 2 (below the bar), would_block 1 (at the bar)", got)
	}
}
