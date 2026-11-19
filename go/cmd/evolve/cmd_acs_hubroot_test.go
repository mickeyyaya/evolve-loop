package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

type hubLayout struct {
	hub     string
	plane   string
	decoy   string
	cycleWT string
}

func newHubLayout(t *testing.T) hubLayout {
	t.Helper()
	requireGit(t)
	src := gittest.Fixture(t)
	src.Write("f.txt", "x")
	src.Git("add", ".")
	src.Git("commit", "-m", "init")
	hub := t.TempDir()
	bare := src.CloneBare(src.Dir, filepath.Join(hub, ".repo.git"))
	layout := hubLayout{
		hub:     hub,
		plane:   filepath.Join(hub, "plane"),
		decoy:   filepath.Join(hub, "decoy"),
		cycleWT: filepath.Join(hub, "plane", ".evolve", "worktrees", "cycle-wt"),
	}
	bare.Git("worktree", "add", "--detach", layout.plane)
	bare.Git("worktree", "add", "--detach", layout.decoy)
	bare.Git("worktree", "add", "--detach", layout.cycleWT)
	if err := os.MkdirAll(filepath.Join(layout.cycleWT, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	return layout
}

func (h hubLayout) cycleEvolveDir() string { return filepath.Join(h.cycleWT, ".evolve") }

func writeKernelCycleState(t *testing.T, projectRoot string, cycle string) {
	t.Helper()
	dir := filepath.Join(projectRoot, ".evolve", "runs", "cycle-"+cycle)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cycle-state.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func sameDir(t *testing.T, a, b string) bool {
	t.Helper()
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	if errA != nil || errB != nil {
		return a == b
	}
	return ra == rb
}

func TestSuiteProjectRoot_HubLayoutResolvesThePlane(t *testing.T) {
	t.Parallel()
	h := newHubLayout(t)
	writeKernelCycleState(t, h.plane, "7")

	got := suiteProjectRoot(h.cycleEvolveDir(), 7, h.cycleWT)
	if sameDir(t, got, h.hub) {
		t.Fatalf("suiteProjectRoot = %q, the hub: the git common dir's parent is not the plane that holds cycle 7", got)
	}
	if !sameDir(t, got, h.plane) {
		t.Errorf("suiteProjectRoot = %q, want the plane %q that holds runs/cycle-7/cycle-state.json", got, h.plane)
	}
}

func TestSuiteProjectRoot_HubLayoutPicksTheWorktreeHoldingThisCyclesState(t *testing.T) {
	t.Parallel()
	h := newHubLayout(t)
	writeKernelCycleState(t, h.plane, "7")
	writeKernelCycleState(t, h.decoy, "8")

	if got := suiteProjectRoot(h.cycleEvolveDir(), 7, h.cycleWT); !sameDir(t, got, h.plane) {
		t.Errorf("cycle 7: suiteProjectRoot = %q, want the plane %q", got, h.plane)
	}
	if got := suiteProjectRoot(h.cycleEvolveDir(), 8, h.cycleWT); !sameDir(t, got, h.decoy) {
		t.Errorf("cycle 8: suiteProjectRoot = %q, want the worktree %q that holds cycle 8", got, h.decoy)
	}
}

func TestSuiteProjectRoot_HubLayoutIgnoresARunDirWithoutKernelState(t *testing.T) {
	t.Parallel()
	h := newHubLayout(t)
	if err := os.MkdirAll(filepath.Join(h.decoy, ".evolve", "runs", "cycle-7"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeKernelCycleState(t, h.plane, "7")

	if got := suiteProjectRoot(h.cycleEvolveDir(), 7, h.cycleWT); !sameDir(t, got, h.plane) {
		t.Errorf("suiteProjectRoot = %q, want the plane %q: a bare runs/cycle-7 minted by acs run is not the kernel state", got, h.plane)
	}
}

func TestSuiteProjectRoot_HubLayoutWithoutACycleKeepsTheGitDerivation(t *testing.T) {
	t.Parallel()
	h := newHubLayout(t)
	writeKernelCycleState(t, h.plane, "7")

	want := mainProjectRoot(h.cycleWT)
	if got := suiteProjectRoot(h.cycleEvolveDir(), 0, h.cycleWT); got != want {
		t.Errorf("suiteProjectRoot with no cycle = %q, want today's git derivation %q", got, want)
	}
}

func TestRunACSSuite_HubLayoutWithoutCycleStateFailsNamingCycleAndRoots(t *testing.T) {
	t.Parallel()
	h := newHubLayout(t)
	writeKernelCycleState(t, h.plane, "7")

	code, stdout, stderr := runACSSuiteCLI(t, "--cycle", "9", "--root", h.cycleWT, "--evolve-dir", h.cycleEvolveDir())
	if code == 0 || code == 2 {
		t.Fatalf("exit = %d, want a hard failure (1 or 10): no worktree holds cycle 9, so no policy may be read from a guessed root (stdout=%q stderr=%q)", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "cycle 9") && !strings.Contains(stderr, "cycle-9") {
		t.Errorf("stderr = %q, want it to name cycle 9", stderr)
	}
	checked := []string{h.plane, h.decoy}
	for _, root := range checked {
		resolved, err := filepath.EvalSymlinks(root)
		if err != nil {
			resolved = root
		}
		if !strings.Contains(stderr, root) && !strings.Contains(stderr, resolved) {
			t.Errorf("stderr = %q, want it to name the checked root %q", stderr, root)
		}
	}
	if strings.Contains(stdout, "[acs suite]") {
		t.Errorf("stdout = %q, want no suite summary: the suite must not run against a guessed root", stdout)
	}
	assertNoSuiteVerdict(t, suiteVerdictPath(h.cycleEvolveDir(), "9"))
}

func TestRunACSSuite_HubLayoutReadsThePlanePolicy(t *testing.T) {
	t.Parallel()
	h := newHubLayout(t)
	writeKernelCycleState(t, h.plane, "7")
	policy := `{"acs":{"predicate_env":["EVOLVE_PROJECT_ROOT"]}}`
	if err := os.WriteFile(filepath.Join(h.plane, ".evolve", "policy.json"), []byte(policy), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runACSSuiteCLI(t, "--cycle", "7", "--root", h.cycleWT, "--evolve-dir", h.cycleEvolveDir())
	if code != 2 {
		t.Fatalf("exit = %d, want 2: the fixture worktree has no Go predicate tree (stdout=%q stderr=%q)", code, stdout, stderr)
	}
	v := readSuiteVerdict(t, suiteVerdictPath(h.cycleEvolveDir(), "7"))
	if !sameDir(t, v.ProjectRoot, h.plane) {
		t.Errorf("verdict project_root = %q, want the plane %q (the hub is %q)", v.ProjectRoot, h.plane, h.hub)
	}
	if !slices.ContainsFunc(v.Warnings, func(w string) bool { return strings.Contains(w, "acs.predicate_env names EVOLVE_PROJECT_ROOT") }) {
		t.Errorf("warnings = %v, want the refusal that only the plane's policy.json can produce", v.Warnings)
	}
}
