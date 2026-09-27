//go:build integration

package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type tddLeakRunner struct {
	name  string
	onRun func()
}

func (r *tddLeakRunner) Name() string { return r.name }
func (r *tddLeakRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	if r.onRun != nil {
		r.onRun()
	}
	return PhaseResponse{Phase: r.name, Verdict: VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

func TestTDDLeakRecover(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	root := initAuditLeakRepo(t)
	runners := buildRunners(nil)

	runners[PhaseTDD] = &tddLeakRunner{
		name: string(PhaseTDD),
		onRun: func() {
			err := os.WriteFile(filepath.Join(root, "docs", "note.md"), []byte("tdd-leaked-modification\n"), 0o644)
			if err != nil {
				t.Errorf("write docs/note.md: %v", err)
			}
		},
	}

	st := &fakeStorage{}
	led := &fakeLedger{}

	// Real gitWorktree provisioner (not fakeWorktree) so recovery runs against an actual git worktree.
	o := NewOrchestrator(st, led, runners, WithWorktreeProvisioner(gitWorktree{}))

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})
	if err != nil {
		t.Fatalf("cycle failed: %v", err)
	}

	if got := auditLeakReadFile(t, filepath.Join(root, "docs", "note.md")); got != auditLeakNoteV1 {
		t.Errorf("docs/note.md in main = %q, want committed %q", got, auditLeakNoteV1)
	}

	shipRan := false
	for _, p := range res.PhasesRun {
		if p == PhaseShip {
			shipRan = true
		}
	}
	if !shipRan {
		t.Errorf("ship never ran — cycle did not continue (phases=%v)", res.PhasesRun)
	}
}

func TestDiscardGitignore(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := initAuditLeakRepo(t)
	ctx := context.Background()

	if isGitignored(ctx, root, "go/evolve") {
		t.Error("go/evolve is tracked and should not be gitignored")
	}

	err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("ignored_file.txt\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("git", "-C", root, "add", ".gitignore")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add gitignore: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "-C", root, "commit", "-m", "add gitignore")
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit gitignore: %v\n%s", err, out)
	}

	err = os.WriteFile(filepath.Join(root, "ignored_file.txt"), []byte("ignored content\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	if !isGitignored(ctx, root, "ignored_file.txt") {
		t.Error("ignored_file.txt should be gitignored")
	}

	err = discardMainLeak(ctx, root, "ignored_file.txt")
	if err == nil {
		t.Error("discardMainLeak on untracked gitignored path should return error")
	} else if !strings.Contains(err.Error(), "git checkout HEAD") {
		t.Errorf("expected git checkout HEAD error; got: %v", err)
	}
}
