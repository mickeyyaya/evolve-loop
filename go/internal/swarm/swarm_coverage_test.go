package swarm

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type failingProvisioner struct {
	integrationErr error
	workerErr      error
	cleaned        []string
}

func (p *failingProvisioner) CreateIntegration(context.Context, string, int) (string, error) {
	if p.integrationErr != nil {
		return "", p.integrationErr
	}
	return "/tmp/integration", nil
}

func (p *failingProvisioner) CreateWorker(_ context.Context, _ string, _ int, workerID, _ string) (string, error) {
	if p.workerErr != nil && workerID == "w1" {
		return "", p.workerErr
	}
	return "/tmp/" + workerID, nil
}

func (p *failingProvisioner) Cleanup(_ context.Context, _ string, wt string) error {
	p.cleaned = append(p.cleaned, wt)
	return nil
}

func TestSwarm_CoverageResultAndValidationEdges(t *testing.T) {
	t.Run("TotalCostUSD", func(t *testing.T) {
		got := (SwarmResult{Workers: []WorkerResult{
			{CostUSD: 0.25}, {CostUSD: 1.50}, {CostUSD: 0},
		}}).TotalCostUSD()
		if got != 1.75 {
			t.Fatalf("TotalCostUSD = %.2f, want 1.75", got)
		}
	})

	t.Run("reader invalid dag collapses", func(t *testing.T) {
		got := Validate(SwarmPlan{Mode: ModeReader, Partitionable: true, Workers: []WorkerSpec{
			{WorkerID: "w0", DependsOn: []string{"missing"}},
			{WorkerID: "w1"},
		}})
		if !got.Collapse || !strings.Contains(got.Reason, "invalid reader DAG") {
			t.Fatalf("Validate reader invalid dag = %+v", got)
		}
	})

	t.Run("fallback without rationale", func(t *testing.T) {
		got := Validate(SwarmPlan{Mode: ModeWriter, Partitionable: false})
		if !got.Collapse || got.Reason != "planner declared non-partitionable" {
			t.Fatalf("fallback reason = %+v", got)
		}
	})

	t.Run("normalize empty path", func(t *testing.T) {
		if got := normalizePath("   "); got != "" {
			t.Fatalf("normalizePath blank = %q", got)
		}
	})

	t.Run("detectConflicts skips blanks and sorts multiple conflicts", func(t *testing.T) {
		got := detectConflicts([]WorkerSpec{
			{WorkerID: "w1", TargetFiles: []string{" b.go ", "", "a.go"}},
			{WorkerID: "w0", TargetFiles: []string{"./B.go", "a.go", "   "}},
		})
		if len(got) != 2 {
			t.Fatalf("conflicts = %+v", got)
		}
		if got[0].File != "a.go" || got[1].File != "b.go" {
			t.Fatalf("conflicts not sorted by file: %+v", got)
		}
		if got[0].Workers[0] != "w0" || got[0].Workers[1] != "w1" {
			t.Fatalf("workers not sorted: %+v", got[0].Workers)
		}
	})
}

func TestSwarm_CoverageManifestErrorBranches(t *testing.T) {
	t.Run("LoadManifest read error", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "dir")
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, _, _, _, err := LoadManifest(dir); err == nil || !strings.Contains(err.Error(), "read swarm manifest") {
			t.Fatalf("LoadManifest directory error = %v", err)
		}
	})

	t.Run("LoadManifest parse error", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "sessions.json")
		if err := os.WriteFile(path, []byte("{bad json"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, _, _, _, err := LoadManifest(path); err == nil || !strings.Contains(err.Error(), "parse swarm manifest") {
			t.Fatalf("LoadManifest parse error = %v", err)
		}
	})

	t.Run("Register persist mkdir error", func(t *testing.T) {
		parentFile := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(parentFile, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		reg := NewSessionRegistry(filepath.Join(parentFile, "sessions.json"), 1, "build", 1)
		if err := reg.Register(handle("w0")); err == nil || !strings.Contains(err.Error(), "swarm manifest dir") {
			t.Fatalf("Register mkdir error = %v", err)
		}
	})

	t.Run("in-memory registry skips persistence", func(t *testing.T) {
		reg := NewSessionRegistry("", 1, "build", 1)
		if err := reg.Register(handle("w0")); err != nil {
			t.Fatalf("in-memory Register = %v", err)
		}
	})

	t.Run("Register persist rename error", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "sessions.json")
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatal(err)
		}
		reg := NewSessionRegistry(path, 1, "build", 1)
		if err := reg.Register(handle("w0")); err == nil || !strings.Contains(err.Error(), "swarm manifest rename") {
			t.Fatalf("Register rename error = %v", err)
		}
	})
}

func TestSwarm_CoverageDispatchProvisioningErrors(t *testing.T) {
	t.Run("integration provision failure", func(t *testing.T) {
		prov := &failingProvisioner{integrationErr: errors.New("no integration")}
		_, err := Dispatch(context.Background(), twoWriterPlan(),
			DispatchRequest{ProjectRoot: t.TempDir(), Cycle: 282, Workspace: t.TempDir()},
			Deps{Launcher: &fakeLauncher{}, Provisioner: prov})
		if err == nil || !strings.Contains(err.Error(), "provision integration branch") {
			t.Fatalf("Dispatch integration error = %v", err)
		}
	})

	t.Run("worker provision failure cleans prior worker", func(t *testing.T) {
		prov := &failingProvisioner{workerErr: errors.New("no worker")}
		_, err := Dispatch(context.Background(), twoWriterPlan(),
			DispatchRequest{ProjectRoot: t.TempDir(), Cycle: 282, Workspace: t.TempDir()},
			Deps{Launcher: &fakeLauncher{}, Provisioner: prov})
		if err == nil || !strings.Contains(err.Error(), "provision worker w1") {
			t.Fatalf("Dispatch worker error = %v", err)
		}
		if len(prov.cleaned) != 1 || prov.cleaned[0] != "/tmp/w0" {
			t.Fatalf("cleanup after worker provision failure = %v", prov.cleaned)
		}
	})
}

func TestSwarm_CoverageGitProvisionerEdges(t *testing.T) {
	t.Run("worktree base mkdir failure", func(t *testing.T) {
		root := gitInit(t)
		baseFile := filepath.Join(t.TempDir(), "base-file")
		if err := os.WriteFile(baseFile, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("EVOLVE_WORKTREE_BASE", filepath.Join(baseFile, "child"))
		p := NewGitWorkerProvisioner(nil)
		if _, err := p.CreateIntegration(context.Background(), root, 282); err == nil || !strings.Contains(err.Error(), "worktree base") {
			t.Fatalf("CreateIntegration mkdir error = %v", err)
		}
	})

	t.Run("default worktree base and empty integration branch base", func(t *testing.T) {
		root := gitInit(t)
		t.Setenv("EVOLVE_WORKTREE_BASE", "")
		p := NewGitWorkerProvisioner(nil)
		wt, err := p.CreateWorker(context.Background(), root, 282, "wbase", "")
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(root, ".evolve", "worktrees", "cycle-282-wbase")
		if wt != want {
			t.Fatalf("CreateWorker default base = %q, want %q", wt, want)
		}
	})

	t.Run("addWorktree reports invalid base", func(t *testing.T) {
		root := gitInit(t)
		t.Setenv("EVOLVE_WORKTREE_BASE", filepath.Join(root, ".evolve", "worktrees"))
		p := NewGitWorkerProvisioner(nil)
		_, err := p.CreateWorker(context.Background(), root, 282, "wbadbase", "missing-base")
		if err == nil || !strings.Contains(err.Error(), "git worktree add") {
			t.Fatalf("CreateWorker invalid base error = %v", err)
		}
	})

	t.Run("stale directory is removed before add", func(t *testing.T) {
		root := gitInit(t)
		base := filepath.Join(root, ".evolve", "worktrees")
		t.Setenv("EVOLVE_WORKTREE_BASE", base)
		stale := filepath.Join(base, "cycle-282-integration")
		if err := os.MkdirAll(stale, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(stale, "junk"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		p := NewGitWorkerProvisioner(nil)
		wt, err := p.CreateIntegration(context.Background(), root, 282)
		if err != nil {
			t.Fatal(err)
		}
		if wt != stale {
			t.Fatalf("CreateIntegration path = %q, want stale path %q", wt, stale)
		}
		if out, err := exec.Command("git", "-C", wt, "rev-parse", "--git-dir").CombinedOutput(); err != nil {
			t.Fatalf("stale dir was not replaced with a git worktree: %v\n%s", err, out)
		}
	})

	t.Run("cleanup surfaces git failure as warning only", func(t *testing.T) {
		wt := filepath.Join(t.TempDir(), "orphan")
		if err := os.MkdirAll(wt, 0o755); err != nil {
			t.Fatal(err)
		}
		p := NewGitWorkerProvisioner(nil)
		if err := p.Cleanup(context.Background(), t.TempDir(), wt); err != nil {
			t.Fatalf("Cleanup should remain best-effort, got %v", err)
		}
		if _, err := os.Stat(wt); !os.IsNotExist(err) {
			t.Fatalf("Cleanup should remove orphan dir, stat err=%v", err)
		}
	})
}

func TestSwarm_CoverageExecGitMerger(t *testing.T) {
	t.Run("merge success", func(t *testing.T) {
		root := gitInit(t)
		runGit(t, root, "checkout", "-q", "-b", "worker")
		if err := os.WriteFile(filepath.Join(root, "worker.txt"), []byte("worker\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit(t, root, "add", "-A")
		runGit(t, root, "commit", "-q", "-m", "worker")
		runGit(t, root, "checkout", "-q", "main")

		if err := (ExecGitMerger{IntegrationWorktree: root}).Merge(context.Background(), "main", "worker"); err != nil {
			t.Fatalf("Merge success path: %v", err)
		}
		if _, err := os.Stat(filepath.Join(root, "worker.txt")); err != nil {
			t.Fatalf("merged file missing: %v", err)
		}
	})

	t.Run("merge conflict aborts", func(t *testing.T) {
		root := gitInit(t)
		if err := os.WriteFile(filepath.Join(root, "conflict.txt"), []byte("main\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit(t, root, "add", "-A")
		runGit(t, root, "commit", "-q", "-m", "main-conflict")
		runGit(t, root, "checkout", "-q", "-b", "worker-conflict", "HEAD~1")
		if err := os.WriteFile(filepath.Join(root, "conflict.txt"), []byte("worker\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit(t, root, "add", "-A")
		runGit(t, root, "commit", "-q", "-m", "worker-conflict")
		runGit(t, root, "checkout", "-q", "main")

		err := (ExecGitMerger{IntegrationWorktree: root}).Merge(context.Background(), "main", "worker-conflict")
		if err == nil || !errors.Is(err, ErrMergeConflict) {
			t.Fatalf("Merge conflict error = %v", err)
		}
		out, gerr := exec.Command("git", "-C", root, "status", "--porcelain").Output()
		if gerr != nil {
			t.Fatal(gerr)
		}
		if strings.TrimSpace(string(out)) != "" {
			t.Fatalf("merge conflict should be aborted cleanly, status:\n%s", out)
		}
	})
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestSwarm_CoverageMoreBehavior(t *testing.T) {
	t.Run("worker prompt includes owned files and acceptance", func(t *testing.T) {
		got := workerPrompt(WorkerSpec{
			Scope:       "own parser",
			TargetFiles: []string{"go/internal/swarm/parse.go"},
			Acceptance:  []string{"go test ./internal/swarm"},
		})
		for _, want := range []string{"own parser", "go/internal/swarm/parse.go", "go test ./internal/swarm"} {
			if !strings.Contains(got, want) {
				t.Fatalf("workerPrompt missing %q:\n%s", want, got)
			}
		}
	})

	t.Run("ExecSessionKiller returns first error", func(t *testing.T) {
		k := ExecSessionKiller{
			KillGroup: func(int) error { return errors.New("pgid boom") },
			KillTmux:  func(context.Context, string) error { return errors.New("tmux boom") },
		}
		err := k.Kill(context.Background(), SessionHandle{WorkerID: "w0", PGID: 2, TmuxSession: "s"})
		if err == nil || !strings.Contains(err.Error(), "kill pgid 2") {
			t.Fatalf("Kill first error = %v", err)
		}
	})

	t.Run("ExecSessionKiller returns tmux error when group skipped", func(t *testing.T) {
		k := ExecSessionKiller{
			KillTmux: func(context.Context, string) error { return errors.New("tmux boom") },
		}
		err := k.Kill(context.Background(), SessionHandle{WorkerID: "w0", PGID: 0, TmuxSession: "s"})
		if err == nil || !strings.Contains(err.Error(), "kill tmux") {
			t.Fatalf("Kill tmux-only error = %v", err)
		}
	})

	t.Run("Dispatch default concurrency", func(t *testing.T) {
		fk := &fakeLauncher{}
		plan := SwarmPlan{Mode: ModeReader, Partitionable: true, TaskID: "r", Workers: []WorkerSpec{
			{WorkerID: "w0"}, {WorkerID: "w1"},
		}}
		res, err := Dispatch(context.Background(), plan, DispatchRequest{Workspace: t.TempDir()},
			Deps{Launcher: fk, Concurrency: 0})
		if err != nil {
			t.Fatal(err)
		}
		if !res.AllOK() || len(fk.launched) != 2 {
			t.Fatalf("default concurrency dispatch = res %+v launches %v", res, fk.launched)
		}
	})

	t.Run("RunMergeTrain negative retries disables resolver", func(t *testing.T) {
		m := &scriptMerger{failBranch: map[string]bool{"cycle-1-w0": true}}
		called := false
		rep := RunMergeTrain(context.Background(), "integ", []string{"w0"}, branchMap("w0"),
			MergeTrainDeps{
				Merger:     m,
				MaxRetries: -1,
				Resolver: func(context.Context, string, string) error {
					called = true
					return nil
				},
			})
		if rep.AllMerged || called {
			t.Fatalf("negative MaxRetries should fail without resolver call: rep=%+v called=%v", rep, called)
		}
	})

	t.Run("RunMergeTrain resolver failure records reason", func(t *testing.T) {
		m := &scriptMerger{failBranch: map[string]bool{"cycle-1-w0": true}}
		rep := RunMergeTrain(context.Background(), "integ", []string{"w0"}, branchMap("w0"),
			MergeTrainDeps{
				Merger: m,
				Resolver: func(context.Context, string, string) error {
					return errors.New("author unavailable")
				},
			})
		if rep.AllMerged || !strings.Contains(rep.Outcomes[0].Reason, "conflict resolution failed") {
			t.Fatalf("resolver failure outcome = %+v", rep.Outcomes)
		}
	})
}
