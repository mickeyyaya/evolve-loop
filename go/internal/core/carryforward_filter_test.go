package core

// carryforward_filter_test.go — fast-tier coverage for the deterministic
// carry-forward candidate screen (cycle 962). These are white-box tests: core
// cannot import test/fixtures (that package imports core), so they drive the
// package gitRunner seam directly through a SCRIPTED fake that answers each git
// subcommand independently — the single-canned-response gitRec is too blunt for
// functions that branch on distinct exit codes across is-ancestor / cherry /
// merge-tree.
//
// The assertions probe INTENT, not surface: (1) a genuine 3-way conflict is
// (false, nil) and NEVER an error, and (2) the supersession screen runs BEFORE
// the merge dry-run so an already-landed candidate is rejected without the
// merge-tree that would mask it as "clean".

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// seamGit is a per-subcommand recording git seam. respond maps a git
// invocation (by args) to (stdout, exitCode); a nil/zero response is (”, 0).
type seamGit struct {
	respond func(args []string) (stdout string, exit int)
	calls   [][]string
}

func (s *seamGit) run(_ context.Context, _ /*name*/, _ /*dir*/ string, args, _ []string, _ io.Reader, out, _ io.Writer) (int, error) {
	s.calls = append(s.calls, append([]string(nil), args...))
	stdout, exit := "", 0
	if s.respond != nil {
		stdout, exit = s.respond(args)
	}
	if out != nil && stdout != "" {
		_, _ = out.Write([]byte(stdout))
	}
	return exit, nil
}

// useSeamGit swaps the package gitRunner seam for the scripted fake.
func useSeamGit(t *testing.T, s *seamGit) {
	t.Helper()
	orig := gitRunner
	gitRunner = sysexec.RunFunc(s.run)
	t.Cleanup(func() { gitRunner = orig })
}

// calledWith reports whether any recorded call's args start with the prefix.
func (s *seamGit) calledWith(prefix ...string) bool {
	for _, c := range s.calls {
		if has(c, prefix...) {
			return true
		}
	}
	return false
}

func TestCarryforwardCandidateLandable_CleanMergeIsLandable(t *testing.T) {
	s := &seamGit{respond: func(args []string) (string, int) {
		switch {
		case has(args, "merge-base", "--is-ancestor"):
			return "", 1 // not an ancestor → not fast-forward-superseded
		case has(args, "cherry"):
			return "+ deadbeef\n", 0 // one commit not on base → not a patch-id duplicate
		case has(args, "merge-tree", "--write-tree"):
			return "treesha\n", 0 // clean 3-way merge
		}
		return "", 0
	}}
	useSeamGit(t, s)

	ok, err := CarryforwardCandidateLandable(context.Background(), "/wt", "cycle-9", "main")
	if err != nil {
		t.Fatalf("landable = err %v, want (true, nil)", err)
	}
	if !ok {
		t.Error("landable = false, want true for a clean, not-yet-landed candidate")
	}
	if !s.calledWith("merge-tree", "--write-tree") {
		t.Error("merge dry-run never ran — landability was decided without the 3-way check")
	}
}

// The load-bearing contract: a REAL merge conflict is (false, nil), never an
// error. A caller distinguishing "not landable" from "git broke" depends on it.
func TestCarryforwardCandidateLandable_ConflictIsFalseNotError(t *testing.T) {
	s := &seamGit{respond: func(args []string) (string, int) {
		switch {
		case has(args, "merge-base", "--is-ancestor"):
			return "", 1
		case has(args, "cherry"):
			return "+ deadbeef\n", 0
		case has(args, "merge-tree", "--write-tree"):
			return "<<<<<<< conflict\n", 1 // exit 1 == genuine 3-way conflict
		}
		return "", 0
	}}
	useSeamGit(t, s)

	ok, err := CarryforwardCandidateLandable(context.Background(), "/wt", "cycle-9", "main")
	if err != nil {
		t.Fatalf("conflict returned err %v, want (false, nil) — a conflict is a verdict, not a failure", err)
	}
	if ok {
		t.Error("landable = true on a conflicting candidate, want false")
	}
}

// Order matters: an already-landed (ancestor) candidate is rejected by the
// supersession screen BEFORE the merge dry-run — merge-tree of an
// already-absorbed change would read "clean" and mask the duplicate.
func TestCarryforwardCandidateLandable_SupersededShortCircuitsBeforeMerge(t *testing.T) {
	s := &seamGit{respond: func(args []string) (string, int) {
		if has(args, "merge-base", "--is-ancestor") {
			return "", 0 // ref IS an ancestor of base → already landed
		}
		return "", 0
	}}
	useSeamGit(t, s)

	ok, err := CarryforwardCandidateLandable(context.Background(), "/wt", "cycle-9", "main")
	if err != nil || ok {
		t.Fatalf("landable = (%v, %v), want (false, nil) for a superseded ancestor", ok, err)
	}
	if s.calledWith("merge-tree", "--write-tree") {
		t.Error("merge dry-run ran on a superseded candidate — the supersession screen must short-circuit first")
	}
}

// Patch-id supersession: not an ancestor, but `git cherry` shows every commit
// already on base (all `-`, no `+`) → superseded, no merge attempted.
func TestCarryforwardCandidateLandable_PatchIdDuplicateSuperseded(t *testing.T) {
	s := &seamGit{respond: func(args []string) (string, int) {
		switch {
		case has(args, "merge-base", "--is-ancestor"):
			return "", 1
		case has(args, "cherry"):
			return "- aaa111\n- bbb222\n", 0 // all equivalent on base, none new
		}
		return "", 0
	}}
	useSeamGit(t, s)

	ok, err := CarryforwardCandidateLandable(context.Background(), "/wt", "cycle-9", "main")
	if err != nil || ok {
		t.Fatalf("landable = (%v, %v), want (false, nil) for a patch-id duplicate", ok, err)
	}
	if s.calledWith("merge-tree", "--write-tree") {
		t.Error("merge dry-run ran on a patch-id-superseded candidate — should short-circuit")
	}
}

// has reports whether args begins with the given subcommand tokens.
func has(args []string, prefix ...string) bool {
	if len(args) < len(prefix) {
		return false
	}
	return strings.Join(args[:len(prefix)], " ") == strings.Join(prefix, " ")
}
