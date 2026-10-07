package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/codereview"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/defectledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const oneHighFindingReport = "## Scope\nlightweight.\n## Findings\n### CR1 (HIGH) — the guard is inverted\n- Dimension: correctness\n- Location: go/x.go:3\n- Scenario: a nil map panics\n- Evidence: probe panics\n- Fix: return early on nil\n## Verdict\nFAIL\n"

func reviewHarness(t *testing.T, completed ...string) (*cycleRun, *[]signalcenter.Event) {
	t.Helper()
	center := signalcenter.New()
	got := &[]signalcenter.Event{}
	center.Subscribe(func(e signalcenter.Event) { *got = append(*got, e) })
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, phasecontract.ArtifactFilename(codereview.PhaseName)), []byte(oneHighFindingReport), 0o644); err != nil {
		t.Fatal(err)
	}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(center))
	return &cycleRun{
		o: o, ctx: context.Background(), req: CycleRequest{ProjectRoot: t.TempDir()}, cycle: 21,
		cs: CycleState{CycleID: 21, WorkspacePath: ws, CompletedPhases: completed}, current: PhaseBuild, envSnap: map[string]string{},
	}, got
}

func reviewEvents(events []signalcenter.Event) []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range events {
		if e.Code == codereview.CodeFindings {
			out = append(out, e)
		}
	}
	return out
}

func complete(t *testing.T, cr *cycleRun, phase Phase, verdict string) {
	t.Helper()
	if _, err := cr.recordAndBranch(phase, dispatchResult{resp: PhaseResponse{Verdict: verdict}, attemptCount: 1}); err != nil {
		t.Fatalf("recordAndBranch(%s): %v", phase, err)
	}
}

func TestCodeReviewCompletion_RecordsItsFindingsAsShadowRowsAndSignalsThem(t *testing.T) {
	cr, events := reviewHarness(t, "scout", "build")
	complete(t, cr, Phase(codereview.PhaseName), VerdictPASS)

	doc, _, err := defectledger.Read(cr.cs.WorkspacePath)
	if err != nil || len(doc.Entries) != 1 {
		t.Fatalf("ledger = %+v (err %v), want the one finding recorded", doc, err)
	}
	row := doc.Entries[0]
	if row.Source != codereview.PhaseName || row.Round != 1 || row.Severity != "HIGH" || row.Dimension != "correctness" || row.Status != defectledger.StatusDeferred {
		t.Errorf("row = %+v, want a round-1 HIGH correctness code-review row, DEFERRED in shadow", row)
	}
	got := reviewEvents(*events)
	if len(got) != 1 || got[0].Cycle != 21 || got[0].Fields["round"] != "1" || got[0].Fields["would_repair"] != "true" || got[0].Fields["stage"] != "shadow" {
		t.Fatalf("REVIEW_FINDINGS events = %+v, want one for cycle 21, round 1, would_repair in shadow", got)
	}
}

func TestCodeReviewCompletion_ASecondDispatchIsRoundTwo(t *testing.T) {
	cr, events := reviewHarness(t, "scout", "build", codereview.PhaseName, "build")
	complete(t, cr, Phase(codereview.PhaseName), VerdictWARN)
	got := reviewEvents(*events)
	if len(got) != 1 || got[0].Fields["round"] != "2" {
		t.Fatalf("REVIEW_FINDINGS = %+v, want round 2: the delta re-review is the cycle's second code-review dispatch", got)
	}
}

func TestCodeReviewCompletion_AFailedReviewOrAnotherPhaseRecordsNothing(t *testing.T) {
	cases := map[string]struct {
		phase   Phase
		verdict string
	}{
		"failed review":     {Phase(codereview.PhaseName), VerdictFAIL},
		"skipped review":    {Phase(codereview.PhaseName), VerdictSKIPPED},
		"another evaluator": {Phase("adversarial-review"), VerdictPASS},
		"a writer":          {Phase("test-amplification"), VerdictPASS},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cr, events := reviewHarness(t, "scout", "build")
			complete(t, cr, tc.phase, tc.verdict)
			if got := reviewEvents(*events); len(got) != 0 {
				t.Errorf("REVIEW_FINDINGS = %+v, want none", got)
			}
			if _, err := os.Stat(filepath.Join(cr.cs.WorkspacePath, defectledger.LedgerFile)); !os.IsNotExist(err) {
				t.Errorf("a ledger was written (stat err %v), want none", err)
			}
		})
	}
}

func TestDispatchEvaluateBatch_RecordsTheCodeReviewsFindingsLikeTheSerialPath(t *testing.T) {
	var active, peak int32
	runners := buildRunners(nil)
	runners[Phase(codereview.PhaseName)] = &concurrentRunner{name: codereview.PhaseName, active: &active, maxActive: &peak}
	runners[Phase("evaluator")] = &concurrentRunner{name: "evaluator", active: &active, maxActive: &peak}
	cr := newBatchCycleRun(t, runners, 2)
	if err := os.WriteFile(filepath.Join(cr.cs.WorkspacePath, phasecontract.ArtifactFilename(codereview.PhaseName)), []byte(oneHighFindingReport), 0o644); err != nil {
		t.Fatal(err)
	}

	if act, err := cr.dispatchEvaluateBatch([]Phase{Phase(codereview.PhaseName), "evaluator"}); err != nil || act != loopNext {
		t.Fatalf("act=%v err=%v, want loopNext", act, err)
	}

	doc, _, err := defectledger.Read(cr.cs.WorkspacePath)
	if err != nil || len(doc.Entries) != 1 || doc.Entries[0].Source != codereview.PhaseName || doc.Entries[0].Round != 1 {
		t.Fatalf("ledger = %+v (err %v), want the batched review's finding recorded as round 1", doc, err)
	}
}
