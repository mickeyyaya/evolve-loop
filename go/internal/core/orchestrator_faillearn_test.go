package core_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// staticVerdictRunner succeeds at the transport level but returns a fixed
// (possibly non-canonical) verdict, modeling a retro that ran but produced garbage.
type staticVerdictRunner struct {
	name    string
	verdict string
}

func (r *staticVerdictRunner) Name() string { return r.name }
func (r *staticVerdictRunner) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	return core.PhaseResponse{Phase: r.name, Verdict: r.verdict, ArtifactsDir: req.Workspace}, nil
}

func runCycleWithFailingTriage(t *testing.T, retro core.PhaseRunner) string {
	t.Helper()
	root := t.TempDir()
	seedCycleStateFile(t, root)

	orch, _, _ := newTestOrchestrator(t, newRunners(map[core.Phase]core.PhaseRunner{
		core.PhaseTriage: &alwaysErrRunner{name: "triage"},
		core.PhaseRetro:  retro,
	}))
	_, err := orch.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: root,
		GoalHash:    "test-goal",
		Context:     map[string]string{"commit_message": "test commit"},
	})
	if err == nil {
		t.Fatal("triage hard failure must surface as a cycle error")
	}
	return root
}

func assertDeterministicArtifacts(t *testing.T, root string) {
	t.Helper()
	report := filepath.Join(root, ".evolve", "runs", "cycle-1", "retrospective-report.md")
	data, err := os.ReadFile(report)
	if err != nil {
		t.Fatalf("deterministic retrospective-report.md must exist after retro degradation: %v", err)
	}
	for _, want := range []string{"deterministic-fallback", "triage"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("report missing %q:\n%s", want, data)
		}
	}

	lessons, err := filepath.Glob(filepath.Join(root, ".evolve", "instincts", "lessons", "cycle-1-phase-*.yaml"))
	if err != nil || len(lessons) != 1 {
		t.Fatalf("want exactly 1 deterministic failure-lesson, got %v (err=%v)", lessons, err)
	}
}

func TestRecordFailureLearning_RetroRunnerError_WritesDeterministicArtifacts(t *testing.T) {
	t.Parallel()
	root := runCycleWithFailingTriage(t, &alwaysErrRunner{name: "retro"})
	assertDeterministicArtifacts(t, root)
}

func TestRecordFailureLearning_NonCanonicalVerdict_WritesDeterministicArtifacts(t *testing.T) {
	t.Parallel()
	root := runCycleWithFailingTriage(t, &staticVerdictRunner{name: "retro", verdict: "GIBBERISH"})
	assertDeterministicArtifacts(t, root)
}

func TestRecordFailureLearning_RetroRunnerError_FailedRecordStillPersisted(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	seedCycleStateFile(t, root)

	orch, st, _ := newTestOrchestrator(t, newRunners(map[core.Phase]core.PhaseRunner{
		core.PhaseTriage: &alwaysErrRunner{name: "triage"},
		core.PhaseRetro:  &alwaysErrRunner{name: "retro"},
	}))
	if _, err := orch.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: root,
		GoalHash:    "test-goal",
		Context:     map[string]string{"commit_message": "test commit"},
	}); err == nil {
		t.Fatal("triage hard failure must surface as a cycle error")
	}

	if len(st.state.FailedAt) != 1 {
		t.Fatalf("FailedAt = %+v, want exactly one cycle-mid-execution-fail record", st.state.FailedAt)
	}
	rec := st.state.FailedAt[0]
	if rec.Classification != "cycle-mid-execution-fail" {
		t.Errorf("classification = %q, want cycle-mid-execution-fail", rec.Classification)
	}
	if !rec.Retrospected {
		t.Error("retrospected = false; the deterministic artifact IS the retrospective for floor purposes")
	}
}

func TestRecordFailureLearning_RetroSucceeds_NoDeterministicFallback(t *testing.T) {
	t.Parallel()
	root := runCycleWithFailingTriage(t, &recordingRetroRunner{name: "retro"})

	report := filepath.Join(root, ".evolve", "runs", "cycle-1", "retrospective-report.md")
	if data, err := os.ReadFile(report); err == nil && strings.Contains(string(data), "deterministic-fallback") {
		t.Error("deterministic fallback written although the LLM retro succeeded")
	}
	lessons, _ := filepath.Glob(filepath.Join(root, ".evolve", "instincts", "lessons", "*.yaml"))
	if len(lessons) != 0 {
		t.Errorf("deterministic lessons %v written although the LLM retro succeeded", lessons)
	}
}

// reportingErrRunner writes a report carrying a v2 failure-block sentinel,
// then fails — the phase was healthy enough to self-report.
//
// See ADR-0039.
type reportingErrRunner struct{ name string }

func (r *reportingErrRunner) Name() string { return r.name }
func (r *reportingErrRunner) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	body := "## Triage\nFAIL\n" + phasecontract.RenderVerdictSentinelWithFailure("triage", "FAIL",
		&phasecontract.FailureBlock{
			Class:         "code-build-fail",
			Defects:       []string{"defect-alpha: walk drops clamp", "defect-beta: nil evidence map"},
			EvidencePaths: []string{"triage-report.md"},
		}) + "\n"
	_ = os.WriteFile(filepath.Join(req.Workspace, "triage-report.md"), []byte(body), 0o644)
	return core.PhaseResponse{Phase: r.name, Verdict: "FAIL", ArtifactsDir: req.Workspace},
		errStatic("triage exploded after self-report")
}

type errStatic string

func (e errStatic) Error() string { return string(e) }

func TestRecordFailureLearning_StructuredBlockFlowsIntoArtifacts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	seedCycleStateFile(t, root)
	orch, _, _ := newTestOrchestrator(t, newRunners(map[core.Phase]core.PhaseRunner{
		core.PhaseTriage: &reportingErrRunner{name: "triage"},
		core.PhaseRetro:  &alwaysErrRunner{name: "retro"},
	}))
	if _, err := orch.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: root,
		GoalHash:    "test-goal",
		Context:     map[string]string{"commit_message": "test commit"},
	}); err == nil {
		t.Fatal("triage hard failure must surface as a cycle error")
	}

	report, err := os.ReadFile(filepath.Join(root, ".evolve", "runs", "cycle-1", "retrospective-report.md"))
	if err != nil {
		t.Fatalf("deterministic retrospective must exist: %v", err)
	}
	for _, want := range []string{"defect-alpha", "defect-beta", "code-build-fail"} {
		if !strings.Contains(string(report), want) {
			t.Errorf("retrospective missing structured %q:\n%s", want, report)
		}
	}

	lessons, _ := filepath.Glob(filepath.Join(root, ".evolve", "instincts", "lessons", "cycle-1-phase-*.yaml"))
	if len(lessons) != 1 {
		t.Fatalf("want 1 lesson, got %v", lessons)
	}
	lesson, _ := os.ReadFile(lessons[0])
	for _, want := range []string{"defect-alpha", "code-build-fail"} {
		if !strings.Contains(string(lesson), want) {
			t.Errorf("lesson missing structured %q:\n%s", want, lesson)
		}
	}
}
