package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

type rebaseRecoveryBuilder struct{ calls int }

func (*rebaseRecoveryBuilder) Name() string { return string(PhaseBuild) }
func (r *rebaseRecoveryBuilder) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	r.calls++
	return (&requiredExplanationBuilder{}).Run(ctx, req)
}

func runResumeGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestRunCycleFromPhase_RecoversSecondCrashAfterRebaseCheckpointThroughBuild(t *testing.T) {
	root := initTestRepo(t)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".evolve/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runResumeGit(t, root, "add", ".gitignore")
	runResumeGit(t, root, "commit", "-q", "-m", "ignore runtime")
	base := runResumeGit(t, root, "rev-parse", "HEAD")
	workspace := filepath.Join(root, ".evolve", "runs", "cycle-42")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	binding := explanationdocs.CycleBinding{
		ProjectRoot: root, Worktree: root, Workspace: workspace, BaseSHA: base,
		Cycle: 42, RunID: "run-42", ContractVersion: explanationdocs.CurrentContractVersion,
	}
	activation := binding
	activation.Worktree, activation.BaseSHA = "", ""
	if err := explanationdocs.Activate(activation); err != nil {
		t.Fatal(err)
	}
	if err := explanationdocs.SealBuild(binding); err != nil {
		t.Fatal(err)
	}
	if _, err := (&requiredExplanationBuilder{}).Run(context.Background(), PhaseRequest{
		ProjectRoot: root, Worktree: root, Workspace: workspace, WorktreeBaseSHA: base,
		Cycle: 42, RunID: "run-42", ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
	}); err != nil {
		t.Fatal(err)
	}
	if failures := explanationdocs.CheckBuild(context.Background(), binding); len(failures) != 0 {
		t.Fatal(failures)
	}
	if err := explanationdocs.SealResult(context.Background(), binding); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "peer.txt"), []byte("peer\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runResumeGit(t, root, "add", "peer.txt")
	runResumeGit(t, root, "commit", "-q", "-m", "peer base")
	newBase := runResumeGit(t, root, "rev-parse", "HEAD")
	if err := explanationdocs.RebaseBuild(context.Background(), binding, newBase); err != nil {
		t.Fatal(err)
	}

	storage := &fakeStorage{state: State{LastCycleNumber: 42}, cycleState: CycleState{
		CycleID: 42, RunID: "run-42", WorkspacePath: workspace, ActiveWorktree: root,
		// The first recovery already persisted newBase, then the host crashed
		// before the forced Build could replace the stale old-base snapshot.
		WorktreeBaseSHA: newBase, ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
	}}
	builder := &rebaseRecoveryBuilder{}
	runners := buildRunners(nil)
	runners[PhaseBuild] = builder
	o := NewOrchestrator(storage, &fakeLedger{}, runners)
	_, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: root}, &ResumePoint{
		CycleID: 42, Phase: string(PhaseAudit), WorktreePath: root,
		StatePath: filepath.Join(root, ".evolve", CycleStateFile),
	})
	if err != nil {
		t.Fatalf("resume rebased split: %v", err)
	}
	if builder.calls != 1 || storage.cycleState.WorktreeBaseSHA != newBase {
		t.Fatalf("recovery builder calls=%d base=%q, want 1/%q", builder.calls, storage.cycleState.WorktreeBaseSHA, newBase)
	}
}

func TestRunCycleFromPhase_RejectsCycleAndVersionRewriteThatHidesHostActivation(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, ".evolve", "runs", "cycle-42")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	binding := explanationdocs.CycleBinding{
		ProjectRoot: root, Workspace: workspace, Cycle: 42, RunID: "run-42",
		ContractVersion: explanationdocs.CurrentContractVersion,
	}
	if err := explanationdocs.Activate(binding); err != nil {
		t.Fatal(err)
	}

	statePath := filepath.Join(workspace, CycleStateFile)
	state := `{"cycle_id":99,"checkpoint":{"enabled":true,"resumeFromPhase":"audit","gitHead":"unknown"}}`
	if err := os.WriteFile(statePath, []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(ipcenv.CycleStateFileKey, statePath)
	point, err := LoadResumeState(context.Background(), root, filepath.Join(root, ".evolve"), ResumeOptions{})
	if err != nil {
		t.Fatalf("LoadResumeState: %v", err)
	}

	storage := &fakeStorage{cycleState: CycleState{
		CycleID: 99, WorkspacePath: workspace, RunID: "run-42",
		ExplanationDocumentationVersion: 0,
	}}
	o := NewOrchestrator(storage, &fakeLedger{}, buildRunners(nil))
	_, err = o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: root}, point)
	if err == nil || !strings.Contains(err.Error(), "resume identity mismatch") {
		t.Fatalf("tampered fleet resume was not rejected: %v", err)
	}
}

func TestRequireResumeExplanationIdentity_RejectsClearedSealedFields(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, ".evolve", "runs", "cycle-42")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	binding := explanationdocs.CycleBinding{
		ProjectRoot: root, Worktree: t.TempDir(), Workspace: workspace,
		BaseSHA: strings.Repeat("a", 40), Cycle: 42, RunID: "run-42",
		ContractVersion: explanationdocs.CurrentContractVersion,
	}
	activation := binding
	activation.Worktree, activation.BaseSHA = "", ""
	if err := explanationdocs.Activate(activation); err != nil {
		t.Fatal(err)
	}
	if err := explanationdocs.SealBuild(binding); err != nil {
		t.Fatal(err)
	}

	err := requireResumeExplanationIdentity(root, workspace, 42, CycleState{
		CycleID: 42, RunID: "run-42", WorkspacePath: workspace,
		ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
		// Deliberately clear the mutable projection of the sealed host fields.
		ActiveWorktree: "", WorktreeBaseSHA: "",
	}, 42)
	if err == nil || !strings.Contains(err.Error(), "worktree/base") {
		t.Fatalf("cleared sealed resume identity was accepted: %v", err)
	}
}

func TestRequireResumeExplanationIdentity_RejectsLegacyDowngradeAgainstAuthoritativeCycle(t *testing.T) {
	root := t.TempDir()
	originalWorkspace := filepath.Join(root, ".evolve", "runs", "cycle-42")
	if err := os.MkdirAll(originalWorkspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := explanationdocs.Activate(explanationdocs.CycleBinding{
		ProjectRoot: root, Workspace: originalWorkspace, Cycle: 42, RunID: "run-42",
		ContractVersion: explanationdocs.CurrentContractVersion,
	}); err != nil {
		t.Fatal(err)
	}
	tamperedWorkspace := filepath.Join(root, ".evolve", "runs", "cycle-99")
	if err := os.MkdirAll(tamperedWorkspace, 0o755); err != nil {
		t.Fatal(err)
	}
	err := requireResumeExplanationIdentity(root, tamperedWorkspace, 42, CycleState{
		CycleID: 99, WorkspacePath: tamperedWorkspace,
		ExplanationDocumentationVersion: 0,
	}, 99)
	if err == nil || !strings.Contains(err.Error(), "identity mismatch") {
		t.Fatalf("global resume accepted cycle/workspace/version downgrade: %v", err)
	}
}

func TestRunCycleFromPhase_GlobalCheckpointUsesHostLastCycleIdentity(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, ".evolve", "runs", "cycle-99")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	storage := &fakeStorage{
		state: State{LastCycleNumber: 42},
		cycleState: CycleState{
			CycleID: 99, WorkspacePath: workspace,
			ExplanationDocumentationVersion: 0,
		},
	}
	o := NewOrchestrator(storage, &fakeLedger{}, buildRunners(nil))
	_, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: root}, &ResumePoint{
		CycleID: 99, Phase: string(PhaseAudit),
		StatePath: filepath.Join(root, ".evolve", CycleStateFile),
	})
	if err == nil || !strings.Contains(err.Error(), "authoritative host cycle") {
		t.Fatalf("global resume trusted rewritten cycle-state identity: %v", err)
	}
}
