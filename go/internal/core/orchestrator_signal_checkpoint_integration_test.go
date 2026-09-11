//go:build integration

package core_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestRunCycle_InterruptPreservesIntegrationBranchForResume(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	t.Setenv("GIT_CONFIG_COUNT", "0")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	w := fixtures.NewWorkspace(t).
		WithFiles(map[string]string{".gitignore": ".evolve/\n.test-worktrees/\n"}).
		WithGitInit().
		Build()
	root := w.Root
	seedCycleStateFile(t, root)
	interruptGitOutput(t, root, "add", ".gitignore")
	interruptGitOutput(t, root, "commit", "-q", "-m", "test base")
	baselineHEAD := interruptGitOutput(t, root, "rev-parse", "HEAD")

	ctx, cancel := context.WithCancel(context.Background())
	runners := newRunners(map[core.Phase]core.PhaseRunner{
		core.PhaseBuild: &cancelingSignalRunner{name: string(core.PhaseBuild), cancel: cancel},
	})
	orch, _, _ := newTestOrchestrator(t, runners)

	_, err := orch.RunCycle(ctx, core.CycleRequest{
		ProjectRoot: root,
		GoalHash:    "interrupt-preserves-integration-history",
		Context:     map[string]string{"commit_message": "test interrupt preserves integration history"},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunCycle error = %v, want context.Canceled", err)
	}

	if got := interruptGitOutput(t, root, "rev-parse", "HEAD"); got != baselineHEAD {
		t.Errorf("integration HEAD = %s, want unchanged %s", got, baselineHEAD)
	}
	if got := interruptGitOutput(t, root, "status", "--porcelain"); got != "" {
		t.Errorf("integration tree is dirty after resumable interrupt: %s", got)
	}
	for _, ext := range []string{"json", "md"} {
		path := filepath.Join(root, "knowledge-base", "cycles", "cycle-1."+ext)
		if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
			t.Errorf("terminal dossier %s exists after resumable interrupt; stat error = %v", path, statErr)
		}
	}
	workspace := filepath.Join(root, ".evolve", "runs", "cycle-1")
	for _, artifact := range []string{"phase-timing.json", "abnormal-events.jsonl"} {
		path := filepath.Join(workspace, artifact)
		if _, statErr := os.Stat(path); statErr != nil {
			t.Errorf("diagnostic artifact %s missing after interrupt: %v", path, statErr)
		}
	}

	resume, err := core.LoadResumeState(context.Background(), root, w.EvolveDir, core.ResumeOptions{})
	if err != nil {
		t.Fatalf("LoadResumeState: %v", err)
	}
	if resume.Phase != string(core.PhaseBuild) {
		t.Errorf("resume phase = %q, want interrupted phase %q", resume.Phase, core.PhaseBuild)
	}
	if !containsStr(resume.CompletedPhases, string(core.PhaseTDD)) {
		t.Errorf("completed phases = %v, want completed TDD retained", resume.CompletedPhases)
	}
	if containsStr(resume.CompletedPhases, string(core.PhaseBuild)) {
		t.Errorf("completed phases = %v, interrupted Build must remain incomplete", resume.CompletedPhases)
	}
}

func interruptGitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	gitArgs := []string{"-C", root, "-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null"}
	cmd := exec.CommandContext(ctx, "git", append(gitArgs, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
