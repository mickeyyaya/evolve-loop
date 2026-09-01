package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type remediationNoopBuilder struct{}

func (remediationNoopBuilder) Name() string { return string(PhaseBuild) }
func (remediationNoopBuilder) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	return PhaseResponse{Phase: string(PhaseBuild), Verdict: VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

type remediationHandoffGate struct {
	request PhaseRequest
}

func (r *remediationHandoffGate) Name() string { return "coverage-gate" }
func (r *remediationHandoffGate) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	r.request = req
	return PhaseResponse{Phase: r.Name(), Verdict: VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

type remediationResealingReviewer struct {
	t       *testing.T
	binding explanationdocs.CycleBinding
}

func (r remediationResealingReviewer) Review(_ context.Context, in ReviewInput) ReviewResult {
	if in.Phase != string(PhaseBuild) {
		return ReviewResult{Approve: true}
	}
	writeRemediationExplanation(r.t, r.binding, "remediated")
	if failures := explanationdocs.CheckBuild(context.Background(), r.binding); len(failures) != 0 {
		r.t.Fatalf("remediation CheckBuild: %v", failures)
	}
	if err := explanationdocs.SealResult(context.Background(), r.binding); err != nil {
		r.t.Fatalf("remediation SealResult: %v", err)
	}
	return ReviewResult{Approve: true}
}

func TestRemediation_GateRerunReceivesResealedExplanationHandoff(t *testing.T) {
	root := remediationGitRepo(t, true)
	worktree := remediationGitRepo(t, false)
	workspace := filepath.Join(root, ".evolve", "runs", "cycle-11")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	base := remediationGit(t, worktree, "rev-parse", "HEAD")
	binding := explanationdocs.CycleBinding{
		ProjectRoot: root, Worktree: worktree, Workspace: workspace, BaseSHA: base,
		Cycle: 11, RunID: "run-11", ContractVersion: explanationdocs.CurrentContractVersion,
	}
	activation := binding
	activation.Worktree, activation.BaseSHA = "", ""
	if err := explanationdocs.Activate(activation); err != nil {
		t.Fatal(err)
	}
	if err := explanationdocs.SealBuild(binding); err != nil {
		t.Fatal(err)
	}
	writeRemediationExplanation(t, binding, "initial")
	if failures := explanationdocs.CheckBuild(context.Background(), binding); len(failures) != 0 {
		t.Fatalf("initial CheckBuild: %v", failures)
	}
	if err := explanationdocs.SealResult(context.Background(), binding); err != nil {
		t.Fatal(err)
	}
	oldView, err := explanationdocs.LoadSnapshot(binding)
	if err != nil {
		t.Fatal(err)
	}

	gatePhase := Phase("coverage-gate")
	gate := &remediationHandoffGate{}
	runners := buildRunners(nil)
	runners[PhaseBuild] = remediationNoopBuilder{}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners,
		WithWorkflowConfig(policy.WorkflowConfig{RemediationRounds: 1, RemediablePhases: []string{string(gatePhase)}}))
	o.reviewer = remediationResealingReviewer{t: t, binding: binding}
	cr := &cycleRun{
		o: o, ctx: context.Background(), req: CycleRequest{ProjectRoot: root}, cycle: binding.Cycle,
		mainDirtyBaseline: map[string]bool{}, retryConfig: o.retryConfig, workflowConfig: o.workflowConfig,
		cs: CycleState{
			CycleID: binding.Cycle, RunID: binding.RunID, WorkspacePath: workspace,
			ActiveWorktree: worktree, WorktreeBaseSHA: base,
			ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
			CompletedPhases:                 []string{string(PhaseBuild)},
		},
	}
	dr := dispatchResult{
		resp: PhaseResponse{Phase: string(gatePhase), Verdict: VerdictFAIL}, attemptCount: 1,
		phaseWorktree: worktree, runner: gate,
		phaseReq: PhaseRequest{
			Cycle: binding.Cycle, RunID: binding.RunID, ProjectRoot: root, Worktree: worktree,
			Workspace: workspace, WorktreeBaseSHA: base,
			ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
			BuildExplanation:                oldView, BuildExplanationState: BuildExplanationAvailable,
		},
	}

	if action, err := cr.maybeRemediate(gatePhase, &dr); err != nil || action != loopNext {
		t.Fatalf("maybeRemediate action=%v err=%v", action, err)
	}
	newView, err := explanationdocs.LoadSnapshot(binding)
	if err != nil {
		t.Fatal(err)
	}
	if explanationdocs.SameView(oldView, newView) {
		t.Fatal("test premise: remediation did not replace the explanation snapshot")
	}
	if !explanationdocs.SameView(gate.request.BuildExplanation, newView) {
		t.Fatalf("gate re-run received stale Build explanation: got=%+v want=%+v", gate.request.BuildExplanation, newView)
	}
}

func remediationGitRepo(t *testing.T, ignoreEvolve bool) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "t@example.com"}, {"config", "user.name", "test"}} {
		remediationGit(t, dir, args...)
	}
	if ignoreEvolve {
		if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".evolve/\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	} else {
		if err := os.MkdirAll(filepath.Join(dir, "config"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config", "app.yaml"), []byte("enabled: false\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	remediationGit(t, dir, "add", "-A")
	remediationGit(t, dir, "commit", "-q", "-m", "base")
	return dir
}

func remediationGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeRemediationExplanation(t *testing.T, binding explanationdocs.CycleBinding, generation string) {
	t.Helper()
	document, err := explanationdocs.DocumentPath(binding.Cycle, binding.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binding.Worktree, "config", "app.yaml"), []byte("enabled: "+generation+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`# Build Explanation

## Build Binding
- Cycle: %d
- Base SHA: %s

## Summary
Records the %s implementation generation for remediation verification.

## Rationale
The explanation changes with the implementation so downstream gates receive the newly approved evidence.

## Changed Areas
- `+"`config/app.yaml`"+` — records the configuration behavior changed by remediation.

## Design Decisions
The existing configuration file remains the sole implementation surface.

## Verification
The remediation integration test compares the resealed host handoff.

## Compatibility
No public schema changes.

## Limitations
This is an isolated fixture.
`, binding.Cycle, binding.BaseSHA, generation)
	path := filepath.Join(binding.Worktree, filepath.FromSlash(document))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	report := "## Explanation Documentation\n- Status: REQUIRED\n- Document: " + document + "\n"
	if err := os.WriteFile(filepath.Join(binding.Workspace, "build-report.md"), []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
}
