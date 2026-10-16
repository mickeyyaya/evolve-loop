//go:build acs

package cycle1879

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	subprocessHangGuard = 8 * time.Minute
	laneCycle           = 4242
	siblingCycle        = 4243
	unrelatedCycle      = 4244
	sharedGoalHash      = "goal-wave-fixture"
	otherGoalHash       = "goal-other-wave"
	laneItemID          = "lane-item"
	siblingItemID       = "sibling-item"
	unrelatedItemID     = "unrelated-item"
	declaredPackage     = "go/internal/alpha"
	declaredDoc         = "docs/alpha.md"
	inScopeFile         = "go/internal/alpha/alpha.go"
	closureFile         = "go/acs/cycle4242/predicates_test.go"
	undeclaredFile      = "go/internal/stray/stray.go"
	siblingPackage      = "go/internal/beta"
	siblingFile         = "go/internal/beta/beta.go"
	unrelatedPackage    = "go/internal/gamma"
	unrelatedFile       = "go/internal/gamma/gamma.go"
	exitClean           = 0
	exitOutOfScope      = 1
	exitIO              = 2
	exitUsage           = 10
)

func TestMain(m *testing.M) {
	code := m.Run()
	if evolveBuild.dir != "" {
		_ = os.RemoveAll(evolveBuild.dir)
	}
	os.Exit(code)
}

type builtEvolve struct {
	once     sync.Once
	dir, bin string
	err      error
}

var evolveBuild builtEvolve

func evolveBin(t *testing.T) string {
	t.Helper()
	evolveBuild.once.Do(func() {
		evolveBuild.dir, evolveBuild.err = os.MkdirTemp("", "evolve-acs-1879-")
		if evolveBuild.err != nil {
			return
		}
		evolveBuild.bin = filepath.Join(evolveBuild.dir, "evolve")
		cmd := exec.Command("go", "build", "-o", evolveBuild.bin, "./cmd/evolve")
		cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
		if out, err := cmd.CombinedOutput(); err != nil {
			evolveBuild.err = fmt.Errorf("go build ./cmd/evolve: %w\n%s", err, out)
		}
	})
	if evolveBuild.err != nil {
		t.Fatal(evolveBuild.err)
	}
	return evolveBuild.bin
}

type result struct {
	stdout, stderr string
	code           int
}

func (r result) String() string {
	return fmt.Sprintf("exit=%d\n--- stdout ---\n%s\n--- stderr ---\n%s", r.code, r.stdout, r.stderr)
}

func runEvolve(t *testing.T, dir string, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), subprocessHangGuard)
	defer cancel()
	cmd := exec.CommandContext(ctx, evolveBin(t), args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "EVOLVE_PROJECT_ROOT="+dir, "GIT_TERMINAL_PROMPT=0")
	cmd.WaitDelay = 5 * time.Second
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("evolve %v outlived the hang guard: %v\n%s\n%s", args, ctx.Err(), stdout.String(), stderr.String())
	}
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("run evolve %v: %v", args, err)
		}
		code = exitErr.ExitCode()
	}
	return result{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

func scopeDelta(t *testing.T, projectRoot string, extra ...string) result {
	t.Helper()
	args := append([]string{"cycle", "scope-delta", "--project-root", projectRoot}, extra...)
	return runEvolve(t, projectRoot, args...)
}

type laneFixture struct {
	cycle    int
	goalHash string
	itemID   string
	files    []string
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	body, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func declareLane(t *testing.T, projectRoot, worktree, baseSHA string, lane laneFixture) {
	t.Helper()
	runDir := filepath.Join(projectRoot, ".evolve", "runs", fmt.Sprintf("cycle-%d", lane.cycle))
	writeJSON(t, filepath.Join(runDir, "cycle-state.json"), map[string]any{
		"cycle_id":          lane.cycle,
		"goal_hash":         lane.goalHash,
		"active_worktree":   worktree,
		"worktree_base_sha": baseSHA,
		"pre_cycle_head":    baseSHA,
		"workspace_path":    runDir,
		"phase":             "build",
		"completed_phases":  []string{"scout", "triage", "tdd"},
	})
	writeJSON(t, filepath.Join(runDir, "lane-scope.json"), map[string]any{
		"todo_ids":  []string{lane.itemID},
		"goal_hash": lane.goalHash,
	})
	record := filepath.Join(projectRoot, ".evolve", "inbox", "processing", fmt.Sprintf("cycle-%d", lane.cycle), "2026-10-01T00-00-00Z-"+lane.itemID+".json")
	writeJSON(t, record, map[string]any{
		"id":         lane.itemID,
		"kind":       "feature",
		"title":      "fixture item " + lane.itemID,
		"files":      lane.files,
		"acceptance": []string{"fixture acceptance"},
		"weight":     0.5,
	})
}

type waveFixture struct {
	projectRoot string
	worktree    *gittest.Repo
	baseSHA     string
}

func newWave(t *testing.T) waveFixture {
	t.Helper()
	projectRoot := t.TempDir()
	worktree := gittest.Fixture(t)
	worktree.Write("README.md", "base\n")
	worktree.Git("add", "-A")
	worktree.Git("commit", "-q", "-m", "base")
	baseSHA := worktree.Git("rev-parse", "HEAD")
	declareLane(t, projectRoot, worktree.Dir, baseSHA, laneFixture{laneCycle, sharedGoalHash, laneItemID, []string{declaredPackage, declaredDoc}})
	siblingWorktree := t.TempDir()
	declareLane(t, projectRoot, siblingWorktree, baseSHA, laneFixture{siblingCycle, sharedGoalHash, siblingItemID, []string{siblingPackage}})
	declareLane(t, projectRoot, siblingWorktree, baseSHA, laneFixture{unrelatedCycle, otherGoalHash, unrelatedItemID, []string{unrelatedPackage}})
	return waveFixture{projectRoot: projectRoot, worktree: worktree, baseSHA: baseSHA}
}

func (w waveFixture) change(t *testing.T, rels ...string) {
	t.Helper()
	for _, rel := range rels {
		w.worktree.Write(rel, "package fixture\n")
	}
	w.worktree.Git("add", "-A")
	w.worktree.Git("commit", "-q", "-m", "lane change")
}

type scopeDeltaEntry struct {
	Path        string `json:"path"`
	Class       string `json:"class"`
	Disposition string `json:"disposition"`
}

type scopeDeltaSummary struct {
	ByClass map[string]int `json:"by_class"`
	Kept    int            `json:"kept"`
	Carved  int            `json:"carved"`
	Refused int            `json:"refused"`
}

type scopeDeltaReport struct {
	Cycle   int               `json:"cycle"`
	Summary scopeDeltaSummary `json:"summary"`
	Entries []scopeDeltaEntry `json:"entries"`
}

func decodeReport(t *testing.T, r result) scopeDeltaReport {
	t.Helper()
	var report scopeDeltaReport
	dec := json.NewDecoder(strings.NewReader(r.stdout))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&report); err != nil {
		t.Fatalf("RED: --json stdout is not one {cycle, summary, entries} object: %v\n%s", err, r)
	}
	return report
}

func entryFor(report scopeDeltaReport, path string) (scopeDeltaEntry, bool) {
	for _, e := range report.Entries {
		if e.Path == path {
			return e, true
		}
	}
	return scopeDeltaEntry{}, false
}

func lineWith(text string, needles ...string) bool {
	for _, line := range strings.Split(text, "\n") {
		if containsAll(line, needles) {
			return true
		}
	}
	return false
}

func containsAll(line string, needles []string) bool {
	for _, n := range needles {
		if !strings.Contains(line, n) {
			return false
		}
	}
	return true
}
