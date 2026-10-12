package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards/treediff"
)

func postReviewScriptedGuardE4(steps ...[]string) *treediff.Guard {
	calls := 0
	return treediff.New(func(context.Context, string) ([]string, error) {
		i := calls
		calls++
		if i >= len(steps) {
			i = len(steps) - 1
		}
		if steps[i] == nil {
			return nil, errors.New("porcelain unreadable")
		}
		return steps[i], nil
	})
}

func postReviewRepoE4(t *testing.T, tracked map[string]string) *gittest.Repo {
	t.Helper()
	repo := gittest.Fixture(t)
	for rel, body := range tracked {
		writeFile(t, filepath.Join(repo.Dir, rel), body)
		repo.Git("add", "--", rel)
	}
	repo.Git("commit", "-q", "-m", "seed")
	return repo
}

func postReviewRunE4(t *testing.T, root string, guard *treediff.Guard) (*cycleRun, *dispatchResult) {
	t.Helper()
	cr := dispatchRunE4(t, &fakeLedger{}, &countingPassRunnerE4{name: "scout"}, PhaseScout, 1)
	cr.req.ProjectRoot = root
	dr := &dispatchResult{resp: PhaseResponse{Phase: "scout", Verdict: VerdictPASS}, attemptCount: 1, treeGuard: guard, phaseWorktree: "/wt"}
	return cr, dr
}

func postReviewExecutableAtE4(t *testing.T, path string, err error) {
	t.Helper()
	orig := postReviewExecutable
	postReviewExecutable = func() (string, error) { return path, err }
	t.Cleanup(func() { postReviewExecutable = orig })
}

func TestApplyPostReviewGuards_ARebuiltBinaryInsideTheMainTreeIsDiscardedAndThePhaseContinues(t *testing.T) {
	repo := postReviewRepoE4(t, map[string]string{"bin/tool": "release build\n"})
	binary := filepath.Join(repo.Dir, "bin", "tool")
	writeFile(t, binary, "rebuilt churn\n")
	postReviewExecutableAtE4(t, binary, nil)
	cr, dr := postReviewRunE4(t, repo.Dir, postReviewScriptedGuardE4([]string{"bin/tool"}, []string{}))

	var act loopAction
	var err error
	log := captureStderr(t, func() { act, err = cr.applyPostReviewGuards(PhaseScout, dr) })

	if err != nil || act != loopNext {
		t.Fatalf("act=%v err=%v, want the churn discarded and the phase to continue", act, err)
	}
	if got, _ := os.ReadFile(binary); string(got) != "release build\n" {
		t.Fatalf("the rebuilt binary was not restored to HEAD: %q", got)
	}
	if !strings.Contains(log, "discarded binary rebuild churn in phase scout") {
		t.Fatalf("the discard must be loud:\n%s", log)
	}
}

func TestApplyPostReviewGuards_AGitignoredBinaryIsNeverCheckedOut(t *testing.T) {
	repo := postReviewRepoE4(t, map[string]string{".gitignore": "bin/tool\n"})
	binary := filepath.Join(repo.Dir, "bin", "tool")
	writeFile(t, binary, "local build\n")
	postReviewExecutableAtE4(t, binary, nil)
	cr, dr := postReviewRunE4(t, repo.Dir, postReviewScriptedGuardE4([]string{"bin/tool"}, []string{}))

	var act loopAction
	var err error
	log := captureStderr(t, func() { act, err = cr.applyPostReviewGuards(PhaseScout, dr) })

	if err != nil || act != loopNext {
		t.Fatalf("act=%v err=%v", act, err)
	}
	if !strings.Contains(log, `relBin path "bin/tool" is gitignored; skipping discardMainLeak`) {
		t.Fatalf("the skip must be named:\n%s", log)
	}
	if got, _ := os.ReadFile(binary); string(got) != "local build\n" {
		t.Fatalf("an ignored local build was touched: %q", got)
	}
}

func TestApplyPostReviewGuards_TheKnownGoEvolvePathIsNotDiscardedTwice(t *testing.T) {
	repo := postReviewRepoE4(t, map[string]string{"go/evolve": "release build\n"})
	binary := filepath.Join(repo.Dir, "go", "evolve")
	writeFile(t, binary, "rebuilt churn\n")
	postReviewExecutableAtE4(t, binary, nil)
	cr, dr := postReviewRunE4(t, repo.Dir, postReviewScriptedGuardE4([]string{"go/evolve"}, []string{}))

	var act loopAction
	var err error
	log := captureStderr(t, func() { act, err = cr.applyPostReviewGuards(PhaseScout, dr) })

	if err != nil || act != loopNext {
		t.Fatalf("act=%v err=%v", act, err)
	}
	if strings.Contains(log, "is gitignored") {
		t.Fatalf("go/evolve is handled by the fixed discard, not the relBin branch:\n%s", log)
	}
	if got, _ := os.ReadFile(binary); string(got) != "release build\n" {
		t.Fatalf("the fixed go/evolve discard did not run: %q", got)
	}
}

func TestApplyPostReviewGuards_AnExecutableOutsideTheMainTreeOrUnknownLeavesNoRelativeBinary(t *testing.T) {
	cases := []struct {
		name string
		exe  func(root string) (string, error)
	}{
		{"the executable lookup fails", func(string) (string, error) { return "", errors.New("no executable") }},
		{"the executable lives outside the tree", func(root string) (string, error) { return filepath.Join(filepath.Dir(root), "elsewhere", "tool"), nil }},
		{"a relative executable cannot be related to the root", func(string) (string, error) { return "relative/tool", nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := postReviewRepoE4(t, map[string]string{"keep.txt": "seed\n"})
			exe, err := tc.exe(repo.Dir)
			postReviewExecutableAtE4(t, exe, err)
			cr, dr := postReviewRunE4(t, repo.Dir, postReviewScriptedGuardE4([]string{"keep.txt"}, []string{"keep.txt"}))

			var act loopAction
			var gerr error
			log := captureStderr(t, func() { act, gerr = cr.applyPostReviewGuards(PhaseScout, dr) })

			if act != loopAbort || gerr == nil || !strings.Contains(gerr.Error(), "leaked paths: [keep.txt]") {
				t.Fatalf("act=%v err=%v, want the real leak to abort", act, gerr)
			}
			if strings.Contains(log, "discarded binary rebuild churn") || strings.Contains(log, "is gitignored") {
				t.Fatalf("no binary path may be derived:\n%s", log)
			}
		})
	}
}

func TestApplyPostReviewGuards_AGuardThatCannotReSnapshotAfterTheDiscardAbortsUnchecked(t *testing.T) {
	repo := postReviewRepoE4(t, map[string]string{"keep.txt": "seed\n"})
	postReviewExecutableAtE4(t, "", errors.New("no executable"))
	cr, dr := postReviewRunE4(t, repo.Dir, postReviewScriptedGuardE4([]string{"keep.txt"}, nil))

	var act loopAction
	var err error
	captureStderr(t, func() { act, err = cr.applyPostReviewGuards(PhaseScout, dr) })

	if act != loopAbort || err == nil || !strings.Contains(err.Error(), "post-phase main-tree snapshot for scout failed after 3 attempts") {
		t.Fatalf("act=%v err=%v, want the unchecked-phase abort", act, err)
	}
}

func TestApplyPostReviewGuards_ALeakedPathUnderAConsoleLeaseContinuesButSaysItIsNotClean(t *testing.T) {
	repo := postReviewRepoE4(t, map[string]string{"leased.txt": "seed\n", "other.txt": "seed\n"})
	postReviewExecutableAtE4(t, "", errors.New("no executable"))
	cases := []struct {
		name      string
		leased    map[string]bool
		wantAbort bool
	}{
		{"every leaked path is leased", map[string]bool{"leased.txt": true}, false},
		{"an unleased path still aborts", map[string]bool{"leased.txt": true}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			leaks := []string{"leased.txt"}
			if tc.wantAbort {
				leaks = append(leaks, "other.txt")
			}
			cr, dr := postReviewRunE4(t, repo.Dir, postReviewScriptedGuardE4(leaks, leaks))
			cr.consoleLeased = tc.leased

			var act loopAction
			var err error
			log := captureStderr(t, func() { act, err = cr.applyPostReviewGuards(PhaseScout, dr) })

			if tc.wantAbort {
				if act != loopAbort || err == nil || !strings.Contains(err.Error(), "leaked paths: [other.txt]") {
					t.Fatalf("act=%v err=%v, want only the unleased path reported", act, err)
				}
				return
			}
			if act != loopNext || err != nil {
				t.Fatalf("act=%v err=%v, want a continue", act, err)
			}
			if !strings.Contains(log, "continued only because 1 leaked path(s) were console-leased") || strings.Contains(log, "wrote only legitimate main-tree paths") {
				t.Fatalf("a leased leak must not read as a clean phase:\n%s", log)
			}
		})
	}
}
