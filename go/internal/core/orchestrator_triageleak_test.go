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

type triageLeakRunner struct {
	name string
	root string
	leak string
}

func (r *triageLeakRunner) Name() string { return r.name }
func (r *triageLeakRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	if err := os.WriteFile(filepath.Join(r.root, r.leak), []byte("leaked scout-report draft\n"), 0o644); err != nil {
		return PhaseResponse{}, err
	}
	return PhaseResponse{Phase: r.name, Verdict: VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

// initLeakRecoverRepo creates a minimal real git repo (one committed file) so
// the orchestrator's default gitDirtyPaths exercises its production code path
// against real git, exactly like initAuditLeakRepo.
func initLeakRecoverRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("init\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Mirror production: the real repo's .gitignore ignores .evolve/* (runtime
	// state — runs/, leases, usage.json), so the orchestrator's own per-cycle
	// workspace writes never appear in `git status`. Without it the final
	// clean-tree assertion would trip on runtime residue, not the leak under test.
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".evolve/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("init", "-q")
	// Identity must live IN the repo, not in the helper's env: RunCycle's own
	// git children (the dossier-closeout commit) don't inherit these env vars,
	// and CI's ubuntu runners have no ambient identity git can auto-detect, so
	// the commit fails there.
	git("config", "user.name", "t")
	git("config", "user.email", "t@t")
	git("add", ".")
	git("commit", "-q", "-m", "init")
	return root
}

func TestOrchestrator_TriageLeakRecover(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := initLeakRecoverRepo(t)
	const leakName = "stray-triage-note.md"
	runners := buildRunners(nil)
	runners[PhaseTriage] = &triageLeakRunner{name: string(PhaseTriage), root: root, leak: leakName}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners, WithWorktreeProvisioner(&fakeWorktree{path: t.TempDir()}))

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})
	if err != nil {
		t.Fatalf("triage leak must be RELOCATED into the worktree, not abort the cycle: %v", err)
	}

	shipRan := false
	for _, p := range res.PhasesRun {
		if p == PhaseShip {
			shipRan = true
		}
	}
	if !shipRan {
		t.Errorf("ship never ran — cycle did not continue past the triage leak (phases=%v)", res.PhasesRun)
	}

	if _, statErr := os.Stat(filepath.Join(root, leakName)); !os.IsNotExist(statErr) {
		t.Errorf("leaked %s still present in the main tree — recovery must relocate it, not leave it (stat err=%v)", leakName, statErr)
	}

	cmd := exec.Command("git", "-C", root, "status", "--porcelain", "-uall")
	out, cerr := cmd.CombinedOutput()
	if cerr != nil {
		t.Fatalf("git status: %v\n%s", cerr, out)
	}
	if s := strings.TrimSpace(string(out)); s != "" {
		t.Fatalf("main tree not clean after recovery:\n%s", s)
	}
}
