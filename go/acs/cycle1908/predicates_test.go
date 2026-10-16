//go:build acs

package cycle1908

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var evolveBuild struct {
	once      sync.Once
	dir, path string
	failure   string
}

func TestMain(m *testing.M) {
	code := m.Run()
	if evolveBuild.dir != "" {
		if err := os.RemoveAll(evolveBuild.dir); err != nil {
			fmt.Fprintf(os.Stderr, "cycle1908: remove %s: %v\n", evolveBuild.dir, err)
		}
	}
	os.Exit(code)
}

func evolveBin(t *testing.T) string {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	evolveBuild.once.Do(func() { evolveBuild.failure = buildEvolve(goDir) })
	if evolveBuild.failure != "" {
		t.Fatalf("go build ./cmd/evolve: %s", evolveBuild.failure)
	}
	return evolveBuild.path
}

func buildEvolve(goDir string) string {
	dir, err := os.MkdirTemp("", "cycle1908-evolve-")
	if err != nil {
		return err.Error()
	}
	evolveBuild.dir, evolveBuild.path = dir, filepath.Join(dir, "evolve")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", evolveBuild.path, "./cmd/evolve")
	cmd.Dir = goDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Sprintf("%v\n%s", err, out)
	}
	return ""
}

type result struct {
	stdout, stderr string
	code           int
}

func (r result) output() string { return r.stdout + r.stderr }

func (r result) String() string {
	return fmt.Sprintf("exit=%d\n--- stdout ---\n%s--- stderr ---\n%s", r.code, r.stdout, r.stderr)
}

func isolatedEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "EVOLVE_") || strings.HasPrefix(name, "GIT_") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0")
}

func sweep(t *testing.T, plane string, extra ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	args := append([]string{"dossier", "sweep", "--project-root", plane}, extra...)
	cmd := exec.CommandContext(ctx, evolveBin(t), args...)
	cmd.Dir, cmd.Env, cmd.WaitDelay = plane, isolatedEnv(), 5*time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	r := result{stdout: stdout.String(), stderr: stderr.String()}
	var exitErr *exec.ExitError
	switch {
	case ctx.Err() != nil:
		t.Fatalf("evolve %s did not return within its budget: %s", strings.Join(args, " "), r)
	case err == nil:
	case errors.As(err, &exitErr):
		r.code = exitErr.ExitCode()
	default:
		t.Fatalf("evolve %s: %v", strings.Join(args, " "), err)
	}
	return r
}

func newPlane(t *testing.T) *gittest.Repo {
	t.Helper()
	r := gittest.Fixture(t)
	r.Write("README.md", "plane\n")
	r.Git("add", "README.md")
	r.Git("commit", "-q", "-m", "base")
	return r
}

func writeCorpusPair(t *testing.T, root string, cycle int) {
	t.Helper()
	d := &dossier.Dossier{Cycle: cycle, Goal: "an orphaned closeout", FinalVerdict: dossier.VerdictPass,
		Phases: []dossier.PhaseRecord{{Name: "build", Verdict: dossier.VerdictPass}}}
	if err := dossier.Write(d, dossier.CyclesDir(root), false); err != nil {
		t.Fatal(err)
	}
}

func planeWithOrphans(t *testing.T) *gittest.Repo {
	t.Helper()
	r := newPlane(t)
	writeCorpusPair(t, r.Dir, 1705)
	writeCorpusPair(t, r.Dir, 1707)
	if err := os.Remove(filepath.Join(dossier.CyclesDir(r.Dir), "cycle-1707.json")); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestC1908_001_DryRunListsTheOrphanPairAndChangesNothing(t *testing.T) {
	r := planeWithOrphans(t)
	head := r.Git("rev-parse", "HEAD")
	status := r.Git("status", "--porcelain=v1", "--untracked-files=all")

	res := sweep(t, r.Dir, "--dry-run")

	if res.code != 0 {
		t.Fatalf("a dry run must exit 0: %s", res)
	}
	if !strings.Contains(res.stdout, "1705") {
		t.Errorf("the dry run must name cycle 1705: %s", res)
	}
	if r.Git("rev-parse", "HEAD") != head {
		t.Error("a dry run moved HEAD")
	}
	if got := r.Git("status", "--porcelain=v1", "--untracked-files=all"); got != status {
		t.Errorf("a dry run changed the tree: %q -> %q", status, got)
	}
}

func TestC1908_002_SweepCommitsTheCompletePairAndLeavesTheHalfPair(t *testing.T) {
	r := planeWithOrphans(t)
	base := r.Git("rev-parse", "HEAD")

	res := sweep(t, r.Dir)

	if res.code != 0 {
		t.Fatalf("sweep must exit 0: %s", res)
	}
	want := "dossier: cycle-1705 closeout\n\nknowledge-base/cycles/cycle-1705.json\nknowledge-base/cycles/cycle-1705.md"
	if got := r.Git("show", "--name-only", "--format=%s", "HEAD"); got != want {
		t.Fatalf("HEAD = %q, want the cycle-1705 closeout\n%s", got, res)
	}
	if r.Git("rev-parse", "HEAD~1") != base {
		t.Error("the closeout must be one commit on the base")
	}
	if r.Git("ls-files", "knowledge-base/cycles/cycle-1707.md") != "" {
		t.Error("the half pair was committed")
	}
	if !strings.Contains(res.output(), "1707") {
		t.Errorf("the report must name the skipped half pair 1707: %s", res)
	}
}

func TestC1908_003_ALiveRunLeaseHoldsTheSweepWithExit1(t *testing.T) {
	r := planeWithOrphans(t)
	runDir := filepath.Join(r.Dir, ".evolve", "runs", "cycle-1706")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "run.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: "run-1706", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
	head := r.Git("rev-parse", "HEAD")

	res := sweep(t, r.Dir)

	if res.code != 1 {
		t.Fatalf("a sweep held by a live run must exit 1: %s", res)
	}
	if !strings.Contains(res.output(), "cycle-1706") {
		t.Errorf("the hold must name the live run: %s", res)
	}
	if r.Git("rev-parse", "HEAD") != head {
		t.Error("a held sweep committed")
	}
}

func TestC1908_004_ACleanCorpusExits0AndAStrayArgumentExits10(t *testing.T) {
	r := newPlane(t)
	head := r.Git("rev-parse", "HEAD")

	if res := sweep(t, r.Dir); res.code != 0 || strings.Contains(res.output(), "unknown subcommand") {
		t.Fatalf("nothing to sweep must exit 0 through a known verb: %s", res)
	}
	if r.Git("rev-parse", "HEAD") != head {
		t.Error("an empty sweep moved HEAD")
	}
	if res := sweep(t, r.Dir, "stray"); res.code != 10 || strings.Contains(res.output(), "unknown subcommand") {
		t.Fatalf("a stray argument is a usage error (exit 10) of the sweep verb: %s", res)
	}
}

// acs-predicate: config-check
func TestC1908_005_TheUsageAndRuntimeReferenceDocumentSweep(t *testing.T) {
	cmd := exec.Command(evolveBin(t), "dossier")
	cmd.Env = isolatedEnv()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	_ = cmd.Run()
	if !strings.Contains(stderr.String(), "sweep") {
		t.Errorf("the dossier usage must name sweep: %q", stderr.String())
	}
	ref := filepath.Join(acsassert.RepoRoot(t), "docs", "operations", "runtime-reference.md")
	if !acsassert.FileContains(t, ref, "dossier sweep") {
		t.Error("runtime-reference.md must document `evolve dossier sweep`")
	}
}
