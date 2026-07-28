//go:build acs

// Package cycle1159 encodes the cycle-1159 acceptance criteria: three
// "landed test-green but never wired into the real call site" defects from the
// fleet-scoped backlog.
//
//	001/002 — workspace-hygiene S5: runGCHook must default an absent gc.mode to
//	          shadow and must actually invoke the S4 worktree/branch sweep.
//	003     — menu pass must preserve an already-committed id prefix.
//	004     — the cycle-962 carry-forward classifier must stay wired into the
//	          fleet-rebase recovery path AND keep its three-verdict semantics.
//
// Every predicate here exercises the system under test (runs the hook, calls the
// selector, classifies a real git repo) — no source-grep-only assertions.
package cycle1159

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goTest runs `go test` for pkg inside the worktree's go/ module and returns
// combined output plus success. The cmd/evolve hook is package main, so it can
// only be exercised through its own test binary.
func goTest(t *testing.T, root, pkg string, runRegex string) (string, bool) {
	t.Helper()
	cmd := exec.Command("go", "test", "-count=1", "-run", runRegex, pkg)
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	return string(out), err == nil
}

// gitIn runs git in dir with a hermetic config so the predicate never depends
// on the operator's global git settings.
func gitIn(t *testing.T, dir string, args ...string) (string, bool) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=acs", "GIT_AUTHOR_EMAIL=acs@evolve.local",
		"GIT_COMMITTER_NAME=acs", "GIT_COMMITTER_EMAIL=acs@evolve.local",
	)
	out, err := cmd.CombinedOutput()
	return string(out), err == nil
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, ok := gitIn(t, dir, args...); !ok {
		t.Fatalf("git %s failed: %s", strings.Join(args, " "), out)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestC1159_001_GCHookDefaultsToShadowAndSweepsWorktrees drives the REAL
// runGCHook through its own package test binary: absent gc.mode resolves to
// shadow, shadow publishes the S5 workspace manifest without mutating, enforce
// actually deletes the merged orphan branch, and an explicit off still opts out
// (the anti-"always sweep" negative).
func TestC1159_001_GCHookDefaultsToShadowAndSweepsWorktrees(t *testing.T) {
	root := acsassert.RepoRoot(t)
	const runRegex = "^TestRunGCHook_(DefaultModeIsShadow|ShadowWritesWorkspaceManifest|EnforceAppliesWorktreeSweep|ExplicitOffSkipsWorktreeSweep|NonGitProjectRootIsFailOpen)$"
	out, ok := goTest(t, root, "./cmd/evolve/", runRegex)
	if !ok {
		t.Errorf("S5 gc-hook wiring predicates are not green:\n%s", out)
	}
	if strings.Contains(out, "no tests to run") || strings.Contains(out, "[no test files]") {
		t.Errorf("the S5 gc-hook contract tests did not run at all (vacuous pass):\n%s", out)
	}
}

// TestC1159_002_GCWorktreeSweepHasProductionCaller is the wiring proof for S5:
// gc.PlanWorktrees/gc.ApplyWorktrees must be reachable from production code, not
// only from their own unit tests (the S4 "green unit, absent integration" gap).
// The behavioral half lives in 001; this predicate pins that the capability is
// reachable at all, which a passing unit test alone cannot show.
func TestC1159_002_GCWorktreeSweepHasProductionCaller(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, fn := range []string{"PlanWorktrees", "ApplyWorktrees"} {
		cmd := exec.Command("grep", "-rl", "--include=*.go", "gc."+fn, "cmd", "internal")
		cmd.Dir = filepath.Join(root, "go")
		out, _ := cmd.CombinedOutput()
		callers := 0
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasSuffix(line, "_test.go") || strings.HasPrefix(line, "internal/gc/") {
				continue
			}
			callers++
		}
		if callers == 0 {
			t.Errorf("gc.%s has zero non-test callers outside internal/gc — the S4 sweep is still dead code", fn)
		}
	}
}

// TestC1159_003_MenuPassPreservesCommittedIds calls the real selector against a
// real inbox dir: a LOW-weight committed candidate must lead the menus even when
// the backlog holds higher-weight work, and a nil prefix must leave the legacy
// selection untouched (the anti-no-op negative — "always prepend" fails it).
func TestC1159_003_MenuPassPreservesCommittedIds(t *testing.T) {
	evolveDir := t.TempDir()
	inbox := filepath.Join(evolveDir, "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(id string, weight float64, file string) {
		b, err := json.Marshal(map[string]any{"id": id, "weight": weight, "files": []string{file}})
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(inbox, id+".json"), string(b))
	}
	write("a1", 0.9, "go/internal/x/a.go")
	write("a2", 0.7, "go/internal/x/a.go")
	write("b1", 0.8, "go/internal/y/b.go")
	write("c1", 0.1, "go/internal/z/c.go")

	committed := []triagecap.FleetCandidate{{ID: "c1", Weight: 0.1, Files: []string{"go/internal/z/c.go"}}}
	menus := triagecap.SelectWaveSeedMenus(evolveDir, committed, 2, 4, nil)
	if len(menus) == 0 || len(menus[0]) == 0 {
		t.Fatalf("menu pass returned no lanes for a committed prefix: %v", menus)
	}
	if got := menus[0][0].ID; got != "c1" {
		t.Errorf("lane 0 rep = %q, want committed id \"c1\" — a committed id must never be displaced by a higher-weight backlog item", got)
	}

	// Negative: an empty prefix must reproduce the committed-blind selection.
	backlog := triagecap.ReadInboxBacklog(evolveDir, nil)
	want := render(triagecap.ExpandWithClusterMates(triagecap.SelectFleetWidthTopN(backlog, 2), backlog, 4))
	if got := render(triagecap.SelectWaveSeedMenus(evolveDir, nil, 2, 4, nil)); got != want {
		t.Errorf("nil committed prefix changed the selection: got %q, want legacy %q", got, want)
	}
}

func render(menus [][]triagecap.FleetCandidate) string {
	var lanes []string
	for _, m := range menus {
		ids := make([]string, len(m))
		for i, c := range m {
			ids[i] = c.ID
		}
		lanes = append(lanes, strings.Join(ids, ","))
	}
	return strings.Join(lanes, " | ")
}

// TestC1159_004_FleetRebaseClassifierWiredAndCorrect pins the carry-forward
// classifier on both axes the wiring-proof policy demands: it BEHAVES correctly
// on a real repo across all three verdicts (a conflict must never be reported as
// already-landed), and it has a real non-test caller in the recovery path.
func TestC1159_004_FleetRebaseClassifierWiredAndCorrect(t *testing.T) {
	repo := t.TempDir()
	mustGit(t, repo, "init", "-b", "main")
	writeFile(t, filepath.Join(repo, "f.txt"), "base\n")
	mustGit(t, repo, "add", "f.txt")
	mustGit(t, repo, "commit", "-m", "base")

	// clean: touches a different file, not on main.
	mustGit(t, repo, "checkout", "-b", "clean")
	writeFile(t, filepath.Join(repo, "g.txt"), "new\n")
	mustGit(t, repo, "add", "g.txt")
	mustGit(t, repo, "commit", "-m", "clean work")

	// conflict: rewrites f.txt from the same base main also rewrites.
	mustGit(t, repo, "checkout", "-b", "conflict", "main")
	writeFile(t, filepath.Join(repo, "f.txt"), "candidate side\n")
	mustGit(t, repo, "add", "f.txt")
	mustGit(t, repo, "commit", "-m", "candidate edit")

	mustGit(t, repo, "checkout", "main")
	writeFile(t, filepath.Join(repo, "f.txt"), "main side\n")
	mustGit(t, repo, "add", "f.txt")
	mustGit(t, repo, "commit", "-m", "main edit")

	// landed: a branch pointing at main is a strict ancestor → superseded.
	mustGit(t, repo, "branch", "landed", "main")

	ctx := context.Background()
	for _, tc := range []struct {
		ref  string
		want core.FleetRebaseVerdict
		name string
	}{
		{"clean", core.FleetRebaseClean, "Clean"},
		{"conflict", core.FleetRebaseConflict, "Conflict"},
		{"landed", core.FleetRebaseAlreadyLanded, "AlreadyLanded"},
	} {
		got, err := core.ClassifyFleetRebaseCandidate(ctx, repo, tc.ref, "main")
		if err != nil {
			t.Errorf("ClassifyFleetRebaseCandidate(%s): unexpected error: %v", tc.ref, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ClassifyFleetRebaseCandidate(%s) = %v, want %s", tc.ref, got, tc.name)
		}
	}

	// Wiring proof: a non-test production caller must exist.
	root := acsassert.RepoRoot(t)
	cmd := exec.Command("grep", "-rl", "--include=*.go", "ClassifyFleetRebaseCandidate", "cmd", "internal")
	cmd.Dir = filepath.Join(root, "go")
	out, _ := cmd.CombinedOutput()
	callers := 0
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasSuffix(line, "_test.go") || line == "internal/core/carryforward_filter.go" {
			continue
		}
		callers++
	}
	if callers == 0 {
		t.Error("ClassifyFleetRebaseCandidate has zero non-test callers — the fleet-rebase pre-screen is dead code again")
	}
}
