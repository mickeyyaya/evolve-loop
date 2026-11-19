//go:build acs

package cycle2528

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/acsverdict"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

type hub struct {
	root    string
	plane   string
	decoy   string
	cycleWT string
	binary  string
}

func buildHub(t *testing.T) hub {
	t.Helper()
	moduleDir := acsassert.GoModuleDir(t)
	binary := filepath.Join(t.TempDir(), "evolve")
	build := exec.Command("go", "build", "-o", binary, "./cmd/evolve")
	build.Dir = moduleDir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build ./cmd/evolve: %v\n%s", err, acsassert.OutputTail(string(out), 30))
	}
	src := gittest.Fixture(t)
	src.Write("f.txt", "x")
	src.Git("add", ".")
	src.Git("commit", "-m", "init")
	root := t.TempDir()
	bare := src.CloneBare(src.Dir, filepath.Join(root, ".repo.git"))
	h := hub{
		root:    root,
		plane:   filepath.Join(root, "plane"),
		decoy:   filepath.Join(root, "decoy"),
		cycleWT: filepath.Join(root, "plane", ".evolve", "worktrees", "cycle-wt"),
		binary:  binary,
	}
	for _, dir := range []string{h.plane, h.decoy, h.cycleWT} {
		bare.Git("worktree", "add", "--detach", dir)
	}
	if err := os.MkdirAll(filepath.Join(h.cycleWT, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	return h
}

func (h hub) holdCycle(t *testing.T, worktree, cycle string) {
	t.Helper()
	dir := filepath.Join(worktree, ".evolve", "runs", "cycle-"+cycle)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cycle-state.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (h hub) acsSuite(t *testing.T, cycle string) (stdout, stderr string, code int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, h.binary, "acs", "suite", "--cycle", cycle)
	cmd.Dir = h.cycleWT
	cmd.WaitDelay = 5 * time.Second
	cmd.Env = slices.DeleteFunc(os.Environ(), func(kv string) bool { return strings.HasPrefix(kv, "EVOLVE_") })
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	code = 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("run evolve acs suite: %v", err)
	}
	return out.String(), errb.String(), code
}

func (h hub) verdict(t *testing.T, cycle string) acssuite.Verdict {
	t.Helper()
	path, err := acsverdict.Path(filepath.Join(h.cycleWT, ".evolve"), atoi(t, cycle))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read verdict: %v", err)
	}
	var v acssuite.Verdict
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("parse verdict %s: %v", path, err)
	}
	return v
}

func atoi(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func sameDir(a, b string) bool {
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	if errA != nil || errB != nil {
		return a == b
	}
	return ra == rb
}

func TestC2528_001_SuiteFromAHubCycleWorktreeResolvesThePlane(t *testing.T) {
	h := buildHub(t)
	h.holdCycle(t, h.plane, "7")
	stdout, stderr, code := h.acsSuite(t, "7")
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (the fixture has no Go predicate tree)\nstdout=%q\nstderr=%q", code, stdout, stderr)
	}
	v := h.verdict(t, "7")
	if sameDir(v.ProjectRoot, h.root) {
		t.Errorf("project_root = %q, the hub: the suite read the policy of the git common dir's parent", v.ProjectRoot)
	}
	if !sameDir(v.ProjectRoot, h.plane) {
		t.Errorf("project_root = %q, want the plane %q that holds cycle 7", v.ProjectRoot, h.plane)
	}
}

func TestC2528_002_SuiteReadsThePlanePolicyJSON(t *testing.T) {
	h := buildHub(t)
	h.holdCycle(t, h.plane, "7")
	policy := `{"acs":{"predicate_env":["EVOLVE_PROJECT_ROOT"]}}`
	if err := os.WriteFile(filepath.Join(h.plane, ".evolve", "policy.json"), []byte(policy), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := h.acsSuite(t, "7")
	if code != 2 {
		t.Fatalf("exit = %d, want 2\nstderr=%q", code, stderr)
	}
	v := h.verdict(t, "7")
	if !slices.ContainsFunc(v.Warnings, func(w string) bool { return strings.Contains(w, "acs.predicate_env names EVOLVE_PROJECT_ROOT") }) {
		t.Errorf("warnings = %v, want the refusal that only the plane's policy.json produces", v.Warnings)
	}
}

func TestC2528_003_SuitePicksTheWorktreeThatHoldsThisCyclesState(t *testing.T) {
	h := buildHub(t)
	h.holdCycle(t, h.plane, "7")
	h.holdCycle(t, h.decoy, "8")
	if _, stderr, code := h.acsSuite(t, "8"); code != 2 {
		t.Fatalf("cycle 8 exit = %d, want 2\nstderr=%q", code, stderr)
	}
	if v := h.verdict(t, "8"); !sameDir(v.ProjectRoot, h.decoy) {
		t.Errorf("cycle 8 project_root = %q, want the worktree %q that holds cycle 8", v.ProjectRoot, h.decoy)
	}
}

func TestC2528_004_SuiteWithoutAnyCycleStateFailsNamingCycleAndRoots(t *testing.T) {
	h := buildHub(t)
	h.holdCycle(t, h.plane, "7")
	stdout, stderr, code := h.acsSuite(t, "9")
	if code == 0 || code == 2 {
		t.Fatalf("exit = %d, want a hard failure (1 or 10): no worktree holds cycle 9\nstdout=%q\nstderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "cycle 9") && !strings.Contains(stderr, "cycle-9") {
		t.Errorf("stderr = %q, want it to name cycle 9", stderr)
	}
	resolved, err := filepath.EvalSymlinks(h.plane)
	if err != nil {
		resolved = h.plane
	}
	if !strings.Contains(stderr, h.plane) && !strings.Contains(stderr, resolved) {
		t.Errorf("stderr = %q, want it to name the checked root %q", stderr, h.plane)
	}
	if strings.Contains(stdout, "[acs suite]") {
		t.Errorf("stdout = %q, want no suite summary from a guessed root", stdout)
	}
	path, pathErr := acsverdict.Path(filepath.Join(h.cycleWT, ".evolve"), 9)
	if pathErr != nil {
		t.Fatal(pathErr)
	}
	if _, statErr := os.Stat(path); statErr == nil {
		t.Errorf("verdict %s exists, want none for a refused run", path)
	}
}
