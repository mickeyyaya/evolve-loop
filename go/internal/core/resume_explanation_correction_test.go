package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type resumedExplanationBuilder struct{ calls int }

func (r *resumedExplanationBuilder) Name() string { return string(PhaseBuild) }
func (r *resumedExplanationBuilder) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	r.calls++
	body := "## Explanation Documentation\n- Status: NOT_APPLICABLE\n- Reason: the resumed Build has no material source diff\n"
	if err := os.WriteFile(filepath.Join(req.Workspace, "build-report.md"), []byte(body), 0o644); err != nil {
		return PhaseResponse{}, err
	}
	return PhaseResponse{Phase: string(PhaseBuild), Verdict: VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

type rejectFirstResumedBuild struct{ calls int }

func (r *rejectFirstResumedBuild) Review(_ context.Context, in ReviewInput) ReviewResult {
	if in.Phase != string(PhaseBuild) {
		return ReviewResult{Approve: true}
	}
	r.calls++
	if r.calls == 1 {
		return ReviewResult{Approve: false, Retry: true, Reason: "exercise resumed correction"}
	}
	return ReviewResult{Approve: true}
}

type rejectTwiceResumedBuild struct{ calls int }

func (r *rejectTwiceResumedBuild) Review(_ context.Context, in ReviewInput) ReviewResult {
	if in.Phase != string(PhaseBuild) {
		return ReviewResult{Approve: true}
	}
	r.calls++
	if r.calls <= 2 {
		return ReviewResult{Approve: false, Retry: true, Reason: "exercise scaled resumed correction"}
	}
	return ReviewResult{Approve: true}
}

func TestRunCycleFromPhase_ResumedBuildUsesCorrectionAndLifecycleReview(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, ".evolve", "runs", "cycle-5")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "test"},
	} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".evolve/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", ".gitignore"}, {"commit", "-q", "-m", "base"}} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	cmd := exec.Command("git", "-C", root, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	base := string(out[:len(out)-1])
	binding := explanationdocs.CycleBinding{
		ProjectRoot: root, Worktree: root, Workspace: workspace, BaseSHA: base,
		Cycle: 5, RunID: "run-5", ContractVersion: explanationdocs.CurrentContractVersion,
	}
	activation := binding
	activation.Worktree, activation.BaseSHA = "", ""
	if err := explanationdocs.Activate(activation); err != nil {
		t.Fatal(err)
	}
	if err := explanationdocs.SealBuild(binding); err != nil {
		t.Fatal(err)
	}
	storage := &fakeStorage{cycleState: CycleState{
		CycleID: 5, RunID: "run-5", WorkspacePath: workspace, ActiveWorktree: root,
		WorktreeBaseSHA: base, ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
	}}
	builder := &resumedExplanationBuilder{}
	reviewer := &rejectFirstResumedBuild{}
	runners := buildRunners(nil)
	runners[PhaseBuild] = builder
	retries := policy.Policy{}.RetryConfig()
	retries.ContractCorrectionRetries = 1
	o := NewOrchestrator(storage, &fakeLedger{}, runners, WithReviewer(reviewer), WithRetryConfig(retries))
	if _, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: root}, &ResumePoint{Phase: string(PhaseBuild), CycleID: 5}); err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}
	if builder.calls != 2 || reviewer.calls != 2 {
		t.Fatalf("resumed Build calls builder=%d reviewer=%d, want 2/2", builder.calls, reviewer.calls)
	}
	if _, err := explanationdocs.LoadSnapshot(binding); err != nil {
		t.Fatalf("approved resumed Build was not sealed: %v", err)
	}
}

func TestReviewResumedDeliverable_LeakRecoveryFailureStopsBeforeReview(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "test"},
	} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	blockedWorktree := filepath.Join(root, "blocked-worktree")
	if err := os.WriteFile(blockedWorktree, []byte("not a directory\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "leak.txt"), []byte("escaped build output\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	reviewer := &recordingReviewer{default_: ReviewResult{Approve: true}}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	o.reviewer = reviewer
	_, err := o.reviewResumedDeliverable(
		context.Background(), root, 9,
		CycleState{
			CycleID: 9, RunID: "run-9", ActiveWorktree: blockedWorktree,
			WorkspacePath: t.TempDir(), ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
		},
		PhaseBuild, &fakeRunner{name: string(PhaseBuild)},
		PhaseRequest{}, PhaseResponse{Phase: string(PhaseBuild), Verdict: VerdictPASS},
		map[string]bool{"blocked-worktree": true},
	)
	if err == nil || !strings.Contains(err.Error(), "worktree-leak recovery failed") {
		t.Fatalf("resume accepted failed leak recovery: %v", err)
	}
	if len(reviewer.calls) != 0 {
		t.Fatalf("review ran after failed leak recovery: calls=%d", len(reviewer.calls))
	}
}

func TestReviewResumedDeliverable_LargeBuildUsesScaledCorrectionBudget(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "triage-report.md"), []byte("cycle_size_estimate: large\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reviewer := &rejectTwiceResumedBuild{}
	builder := &resumedExplanationBuilder{}
	retries := policy.Policy{}.RetryConfig()
	retries.ContractCorrectionRetries = 1 // large ×1.5 rounds to two corrections
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithRetryConfig(retries))
	o.reviewer = reviewer
	cs := CycleState{
		CycleID: 9, RunID: "run-9", WorkspacePath: workspace,
		CompletedPhases:                 []string{string(PhaseTriage)},
		ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
	}
	_, err := o.reviewResumedDeliverable(
		context.Background(), t.TempDir(), 9, cs, PhaseBuild, builder,
		PhaseRequest{Workspace: workspace},
		PhaseResponse{Phase: string(PhaseBuild), Verdict: VerdictPASS}, nil,
	)
	if err != nil {
		t.Fatalf("scaled resumed correction rejected: %v", err)
	}
	if reviewer.calls != 3 || builder.calls != 2 {
		t.Fatalf("reviewer/builder calls=%d/%d, want 3/2", reviewer.calls, builder.calls)
	}
}
