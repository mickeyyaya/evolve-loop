package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func gitOut(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func initRepoWithCommit(t *testing.T, root string) {
	t.Helper()
	gitOut(t, root, "init", "-q", "-b", "main")
	gitOut(t, root, "config", "user.email", "t@example.com")
	gitOut(t, root, "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, root, "add", "README.md")
	gitOut(t, root, "commit", "-q", "-m", "seed")
}

// simulateCaptureStderr redirects os.Stderr around fn (the orchestrator's
// host WARNs go there) and returns what was written. Not parallel-safe.
func simulateCaptureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	done := make(chan string)
	go func() {
		var b strings.Builder
		buf := make([]byte, 64*1024)
		for {
			n, rerr := r.Read(buf)
			b.Write(buf[:n])
			if rerr != nil {
				break
			}
		}
		done <- b.String()
	}()
	fn()
	os.Stderr = orig
	_ = w.Close()
	out := <-done
	_ = r.Close()
	return out
}

// statusWithoutWalkRecords drops the walk's own untracked records, so the
// comparison sees only the operator's tree; a staged or modified path still counts.
func statusWithoutWalkRecords(status string) string {
	var kept []string
	for _, line := range strings.Split(status, "\n") {
		if strings.HasPrefix(line, "?? ") && (strings.HasSuffix(line, " .evolve/") || strings.HasSuffix(line, " knowledge-base/")) {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// The seed repo holds a Go module, an unformatted tracked file and an
// uncommitted edit, so the host's gofmt -w and leak-recovery checkout would show.
func TestWireSimulateOrchestrator_NeverMutatesGit(t *testing.T) {
	root := t.TempDir()
	initRepoWithCommit(t, root)
	const unformatted = "package seed\n\nfunc   Ugly( ) int {\n\treturn   1\n}\n"
	mustSeed := func(rel, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, rel), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustSeed("go/go.mod", "module example.com/seed\n\ngo 1.24\n")
	mustSeed("go/seed.go", unformatted)
	gitOut(t, root, "add", "go")
	gitOut(t, root, "commit", "-q", "-m", "seed module with an unformatted file")
	// Leak recovery on an in-place root would `git checkout` this edit away.
	mustSeed("README.md", "seed\noperator's uncommitted edit\n")
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	before := gitOut(t, root, "rev-list", "--count", "HEAD")
	statusBefore := gitOut(t, root, "status", "--porcelain")

	var res core.CycleResult
	stderr := simulateCaptureStderr(t, func() {
		d := wireSimulateOrchestrator(root, evolveDir, os.Stderr)
		res, _ = d.Orchestrator.RunCycle(context.Background(), core.CycleRequest{ProjectRoot: root, GoalHash: "simulate-never-mutates-git"})
		d.Signals.Flush()
	})

	if after := gitOut(t, root, "rev-list", "--count", "HEAD"); after != before {
		t.Errorf("a simulate walk committed into the repo: %s → %s commits\n%s", before, after, gitOut(t, root, "log", "--oneline", "-3"))
	}
	if branches := gitOut(t, root, "branch", "--list", "cycle-*"); branches != "" {
		t.Errorf("a simulate walk created cycle branches: %q", branches)
	}
	if wts := gitOut(t, root, "worktree", "list"); strings.Count(wts, "\n") != 0 {
		t.Errorf("a simulate walk registered git worktrees:\n%s", wts)
	}
	if _, serr := os.Stat(filepath.Join(evolveDir, "worktrees")); serr == nil {
		t.Error("a simulate walk must not provision into .evolve/worktrees (that is the real provisioner's home)")
	}
	if got, _ := os.ReadFile(filepath.Join(root, "go", "seed.go")); string(got) != unformatted {
		t.Errorf("the host's gofmt normalizer rewrote the operator's file:\n%s", got)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "README.md")); !strings.Contains(string(got), "operator's uncommitted edit") {
		t.Errorf("the host's leak recovery reverted the operator's uncommitted edit:\n%s", got)
	}
	if statusAfter := statusWithoutWalkRecords(gitOut(t, root, "status", "--porcelain")); statusAfter != statusBefore {
		t.Errorf("the tree's porcelain status changed:\nbefore: %q\nafter:  %q", statusBefore, statusAfter)
	}
	if !strings.Contains(stderr, "the active worktree is the project root") {
		t.Errorf("the in-place root is announced once, with what it disables, got:\n%s", stderr)
	}
	if _, serr := os.Stat(filepath.Join(root, "knowledge-base", "cycles")); serr != nil {
		t.Errorf("the dossier file (uncommitted) is still written: %v", serr)
	}
	if res.Cycle < 1 {
		t.Errorf("a real cycle number: %+v", res)
	}
}

func TestSimulatePhase_WritesTheContractedReportStub(t *testing.T) {
	ws := filepath.Join(t.TempDir(), "runs", "cycle-3")
	resp, err := (&simulatePhase{name: core.PhaseBuild}).Run(context.Background(), core.PhaseRequest{Workspace: ws, Cycle: 3})
	if err != nil || resp.Verdict != core.VerdictPASS {
		t.Fatalf("resp = %+v err = %v", resp, err)
	}
	data, err := os.ReadFile(filepath.Join(ws, "build-report.md"))
	if err != nil {
		t.Fatalf("the build deliverable stub: %v", err)
	}
	if !strings.Contains(string(data), `evolve-verdict: {"phase":"build","verdict":"PASS"`) || !strings.Contains(string(data), "## Explanation Documentation\n- Status: NOT_APPLICABLE") {
		t.Errorf("the build stub carries the PASS sentinel and a NOT_APPLICABLE explanation declaration: %s", data)
	}
	if _, err := (&simulatePhase{name: core.PhaseScout}).Run(context.Background(), core.PhaseRequest{Workspace: ws, Cycle: 3}); err != nil {
		t.Fatal(err)
	}
	if scout, _ := os.ReadFile(filepath.Join(ws, "scout-report.md")); strings.Contains(string(scout), "Explanation Documentation") {
		t.Error("only the build stub declares explanation documentation")
	}
}

func TestCampaignBeforeWave_SimulateRunsNoLiveProbes(t *testing.T) {
	if hook := campaignBeforeWave(true, t.TempDir(), os.Stderr); hook != nil {
		t.Error("--simulate must install no live-probe hook")
	}
	if hook := campaignBeforeWave(false, t.TempDir(), os.Stderr); hook == nil {
		t.Error("a real run keeps the probes")
	}
}
