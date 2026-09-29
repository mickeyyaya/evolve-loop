package core

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

type explanationAuditRunner struct {
	t        *testing.T
	runs     int
	document string
	defects  func(document string) []string
}

func (r *explanationAuditRunner) Name() string { return string(PhaseAudit) }

func (r *explanationAuditRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	r.runs++
	if r.runs > 1 {
		return PhaseResponse{Phase: string(PhaseAudit), Verdict: VerdictPASS, ArtifactsDir: req.Workspace}, nil
	}
	document, err := explanationdocs.DocumentPath(req.Cycle, req.RunID)
	if err != nil {
		return PhaseResponse{}, err
	}
	r.document = document
	failure := &phasecontract.FailureBlock{Class: "code-audit-fail", Defects: r.defects(document)}
	report := "# Audit Report\n\n" + phasecontract.RenderVerdictSentinelWithFailure("audit", "FAIL", failure) + "\n"
	writeBriefFixture(r.t, req.Workspace, phasecontract.ArtifactFilename(string(PhaseAudit)), report)
	return PhaseResponse{Phase: string(PhaseAudit), Verdict: VerdictFAIL, ArtifactsDir: req.Workspace}, nil
}

func runExplanationRepairCycle(t *testing.T, defects func(string) []string) (*explanationAuditRunner, *fakeRunner, *fakeRunner) {
	t.Helper()
	runners := buildRunners(map[Phase]string{PhaseRetro: VerdictFAIL})
	audit := &explanationAuditRunner{t: t, defects: defects}
	runners[PhaseAudit] = audit
	o := NewOrchestrator(&fakeStorage{state: State{}}, &fakeLedger{}, runners)
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if audit.runs != 2 {
		t.Fatalf("audit ran %d time(s), want 2: one FAIL, then the repair round's re-audit", audit.runs)
	}
	return audit, runners[PhaseTDD].(*fakeRunner), runners[PhaseBuild].(*fakeRunner)
}

func TestRunCycle_AnExplanationOnlyAuditFailReDispatchesOnlyTheBuildReauthor(t *testing.T) {
	audit, tdd, build := runExplanationRepairCycle(t, func(document string) []string {
		return []string{document + ":8 states 9 lines; the ratchet scanner measures 8", document + ":35 claims the probe pins the order"}
	})

	if len(tdd.requests) != 1 {
		t.Fatalf("tdd dispatched %d time(s), want 1: the doc-only correction must not restart at tdd", len(tdd.requests))
	}
	if len(build.requests) != 2 {
		t.Fatalf("build dispatched %d time(s), want 2: the correction round is Build's explanation re-author", len(build.requests))
	}
	round := build.requests[1].Context
	if round[CtxKeyExplanationReauthor] != audit.document {
		t.Errorf("the re-author dispatch was not scoped to %q: %q", audit.document, round[CtxKeyExplanationReauthor])
	}
	if !strings.Contains(round[CtxKeyAuditRepairFindings], explanationNeedsCorrection+": "+audit.document+":8") {
		t.Errorf("the re-author was not handed the recorded findings:\n%s", round[CtxKeyAuditRepairFindings])
	}
}

func TestRunCycle_AMixedAuditFailStillRestartsAtTDD(t *testing.T) {
	_, tdd, build := runExplanationRepairCycle(t, func(document string) []string {
		return []string{document + ":8 states 9 lines", "H1: go/internal/cyclesimulator/characterization_test.go:263 builds a raw git repo"}
	})

	if len(tdd.requests) != 2 || len(build.requests) != 2 {
		t.Fatalf("tdd %d, build %d dispatches, want 2 and 2: a FAIL with a code defect keeps the full repair round", len(tdd.requests), len(build.requests))
	}
	if _, scoped := build.requests[1].Context[CtxKeyExplanationReauthor]; scoped {
		t.Error("a mixed repair round was scoped to the explanation document")
	}
}
