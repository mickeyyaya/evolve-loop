package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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
	if !strings.Contains(round[CtxKeyAuditRepairFindings], "- "+audit.document+":8 states 9 lines") {
		t.Errorf("the re-author was not handed the failure block's defects:\n%s", round[CtxKeyAuditRepairFindings])
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

type reauthorDyingBuilder struct {
	calls     int
	workspace string
}

func (b *reauthorDyingBuilder) Name() string { return string(PhaseBuild) }

func (b *reauthorDyingBuilder) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	b.calls++
	b.workspace = req.Workspace
	if req.Context[CtxKeyExplanationReauthor] != "" {
		return PhaseResponse{}, errors.New("re-author build: bridge exited 81 (submit_wedged)")
	}
	return PhaseResponse{Phase: string(PhaseBuild), Verdict: VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

func TestRunCycle_AReauthorBuildThatDiesIsDigestedFromItsOwnFailure(t *testing.T) {
	runners := buildRunners(map[Phase]string{PhaseRetro: VerdictFAIL})
	runners[PhaseAudit] = &explanationAuditRunner{t: t, defects: func(document string) []string {
		return []string{document + ":8 states 9 lines; the ratchet scanner measures 8"}
	}}
	builder := &reauthorDyingBuilder{}
	runners[PhaseBuild] = builder
	o := NewOrchestrator(&fakeStorage{state: State{}}, &fakeLedger{}, runners)

	_, _ = o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})

	if builder.calls < 2 {
		t.Fatalf("build dispatched %d time(s); the re-author round never ran", builder.calls)
	}
	raw, err := os.ReadFile(filepath.Join(builder.workspace, "failure-digest.json"))
	if err != nil {
		t.Fatalf("no failure digest for the dead re-author: %v", err)
	}
	var digest FailureDigest
	if err := json.Unmarshal(raw, &digest); err != nil {
		t.Fatalf("failure digest: %v", err)
	}
	if !strings.HasPrefix(digest.Fingerprint, string(PhaseBuild)+"|") {
		t.Fatalf("fingerprint %q names another phase's failure; the re-author Build died and must be digested as build", digest.Fingerprint)
	}
}
