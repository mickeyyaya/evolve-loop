//go:build acs

package cycle1693

import (
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cmdEvolvePkg = "./cmd/evolve"
	frozenFile   = "go/cmd/evolve/cmd_loop_iteration_state_coherence_test.go"
)

var (
	positiveTests = []string{
		"TestPrepareIteration_StaleCanonicalState",
		"TestPrepareIteration_StaleCanonicalState/sigkilled_lane_fresh_heartbeat_dead_owner",
		"TestPrepareIteration_StaleCanonicalState/leaseless_record",
		"TestPrepareIteration_StaleCanonicalState/killed_mid_retro",
	}
	negativeTests = []string{
		"TestPrepareIteration_ResumableCycleUntouched",
		"TestPrepareIteration_ResumableCycleUntouched/dead_owner_lease=true",
		"TestPrepareIteration_ResumableCycleUntouched/dead_owner_lease=false",
		"TestPrepareIteration_OwnInFlightCycleUntouched",
		"TestPrepareIteration_CompletedCycleRecordUntouched",
		"TestPrepareIteration_TerminalOrFreshRecordUntouched",
		"TestPrepareIteration_TerminalOrFreshRecordUntouched/fresh_tree",
		"TestPrepareIteration_TerminalOrFreshRecordUntouched/already_aborted",
		"TestPrepareIteration_TerminalOrFreshRecordUntouched/end_marker",
	}
	wiringTests = []string{
		"TestPrepareIteration_WiredBeforeEveryIteration",
		"TestPrepareIteration_WiredBeforeEveryIteration/sequential_runLoop_entrypoint",
		"TestPrepareIteration_WiredBeforeEveryIteration/fleet_wave_config",
		"TestPrepareIteration_WiredBeforeEveryIteration/fleet_pool_config",
	}
	errorTests = []string{
		"TestPrepareIteration_CoherenceReadErrorSurfacesWithoutWrite",
		"TestPrepareIteration_ReconcileWriteErrorSurfaces",
	}
)

var (
	suiteOnce sync.Once
	suiteOut  string
	suiteCode int
	suiteErr  error
)

func frozenSuite(t *testing.T) string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	suiteOnce.Do(func() {
		var tops []string
		seen := map[string]bool{}
		for _, group := range [][]string{positiveTests, negativeTests, wiringTests, errorTests} {
			for _, n := range group {
				top := strings.SplitN(n, "/", 2)[0]
				if !seen[top] {
					seen[top] = true
					tops = append(tops, top)
				}
			}
		}
		pattern := "^(" + strings.Join(tops, "|") + ")$"
		stdout, stderr, code, err := acsassert.SubprocessOutput(
			"go", "-C", filepath.Join(root, "go"), "test", "-race", "-count=1", "-v", "-run", pattern, cmdEvolvePkg)
		suiteOut, suiteCode, suiteErr = stdout+stderr, code, err
	})
	if suiteErr != nil && suiteCode == -1 {
		t.Fatalf("could not run go test: %v", suiteErr)
	}
	return suiteOut
}

func requirePass(t *testing.T, out string, names []string) {
	t.Helper()
	for _, n := range names {
		if strings.Contains(out, "--- FAIL: "+n+" (") {
			t.Errorf("RED: %s FAILED", n)
			continue
		}
		if !strings.Contains(out, "--- PASS: "+n+" (") {
			t.Errorf("RED: %s did not PASS (missing, renamed, skipped or not compiled)", n)
		}
	}
	if t.Failed() {
		t.Logf("frozen suite output (exit=%d):\n%s", suiteCode, tail(out, 120))
	}
}

func tail(s string, lines int) string {
	parts := strings.Split(s, "\n")
	if len(parts) > lines {
		parts = parts[len(parts)-lines:]
	}
	return strings.Join(parts, "\n")
}

func TestC1693_001_StaleCanonicalStateReconciledWithWarn(t *testing.T) {
	requirePass(t, frozenSuite(t), positiveTests)
}

func TestC1693_002_ResumableAndLiveRecordsUntouched(t *testing.T) {
	requirePass(t, frozenSuite(t), negativeTests)
}

func TestC1693_003_WiredAtIterationTopForEveryPath(t *testing.T) {
	out := frozenSuite(t)
	requirePass(t, out, wiringTests)
	if strings.Contains(out, "WARNING: DATA RACE") {
		t.Errorf("RED: the race detector reported a data race in the frozen suite:\n%s", tail(out, 120))
	}
}

func TestC1693_004_ScopeConfinedToIterationTopFiles(t *testing.T) {
	root := acsassert.RepoRoot(t)
	allowed := map[string]bool{
		"go/cmd/evolve/cmd_loop_window.go":         true,
		"go/cmd/evolve/cmd_loop_blockerbreaker.go": true,
		"go/cmd/evolve/cmd_loop_control.go":        true,
	}
	protected := []string{
		"go/internal/loopwave/",
		"go/cmd/evolve/cmd_loop_wave.go",
		"go/cmd/evolve/cmd_loop_chain.go",
		"go/internal/core/cyclerun.go",
		"go/internal/core/cyclerun_dispatch.go",
	}

	changed := changedPaths(t, root)
	landed := 0
	for _, p := range changed {
		for _, prot := range protected {
			if p == prot || (strings.HasSuffix(prot, "/") && strings.HasPrefix(p, prot)) {
				t.Errorf("RED: protected control-plane surface touched: %s", p)
			}
		}
		if !strings.HasPrefix(p, "go/") || !strings.HasSuffix(p, ".go") {
			continue
		}
		if strings.HasSuffix(p, "_test.go") {
			if !strings.HasPrefix(p, "go/cmd/evolve/") && !strings.HasPrefix(p, "go/acs/cycle1693/") {
				t.Errorf("RED: test file outside the lane's scope changed: %s", p)
			}
			continue
		}
		if !allowed[p] {
			t.Errorf("RED: production file outside go/cmd/evolve/cmd_loop_{window,blockerbreaker,control}.go changed: %s", p)
			continue
		}
		landed++
	}
	if landed == 0 {
		t.Errorf("RED: no change to cmd_loop_window.go / cmd_loop_blockerbreaker.go / cmd_loop_control.go — the fix has not landed in the iteration-top surface (changed: %v)", changed)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", frozenFile); code != 0 {
		t.Errorf("RED: %s is not git-tracked — the frozen contract would be dropped at ship", frozenFile)
	}
}

func TestC1693_005_CoherenceErrorsSurfaceLoudly(t *testing.T) {
	requirePass(t, frozenSuite(t), errorTests)
}

func changedPaths(t *testing.T, root string) []string {
	t.Helper()
	seen := map[string]bool{}
	add := func(p string) {
		if p = strings.TrimSpace(p); p != "" {
			seen[p] = true
		}
	}

	var bases []string
	for _, ref := range []string{"main", "origin/main"} {
		if mb, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "merge-base", "HEAD", ref); code == 0 {
			bases = append(bases, strings.TrimSpace(mb))
		}
	}
	if len(bases) == 0 {
		t.Fatalf("no merge-base with main or origin/main in %s", root)
	}
	base := bases[0]
	for _, cand := range bases[1:] {
		if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "merge-base", "--is-ancestor", base, cand); code == 0 {
			base = cand
		}
	}
	out, errOut, code, _ := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", base, "HEAD")
	if code != 0 {
		t.Fatalf("git diff %s HEAD: exit %d: %s", base, code, errOut)
	}
	for _, line := range strings.Split(out, "\n") {
		add(line)
	}

	status, errOut, code, _ := acsassert.SubprocessOutput("git", "-C", root, "status", "--porcelain", "--untracked-files=all")
	if code != 0 {
		t.Fatalf("git status: exit %d: %s", code, errOut)
	}
	for _, line := range strings.Split(status, "\n") {
		if len(line) < 4 {
			continue
		}
		p := line[3:]
		if i := strings.Index(p, " -> "); i >= 0 {
			p = p[i+4:]
		}
		add(p)
	}

	paths := make([]string, 0, len(seen))
	for p := range seen {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}
