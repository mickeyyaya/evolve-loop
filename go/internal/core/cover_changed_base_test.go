package core

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

const coverChangedFullProfile = "mode: set\nexample.com/m/p/a.go:3.19,4.11 1 1\nexample.com/m/p/a.go:4.11,6.3 1 1\nexample.com/m/p/a.go:7.2,7.10 1 1\n"

type baseRunCall struct {
	pkgs  []string
	names []string
}

func stubCoverChangedBaseRunner(t *testing.T, red []string, err error) *[]baseRunCall {
	t.Helper()
	var calls []baseRunCall
	orig := coverChangedBaseRunner
	coverChangedBaseRunner = func(_ context.Context, _ ReviewInput, pkgs, names []string) ([]string, error) {
		calls = append(calls, baseRunCall{pkgs, names})
		return red, err
	}
	t.Cleanup(func() { coverChangedBaseRunner = orig })
	return &calls
}

func enforceFloorInput(t *testing.T, repo *gittest.Repo, base string, stderr *injectedStderr) ReviewInput {
	t.Helper()
	in := floorInput(t, floorRoot(t, `{"cover_changed":{"stage":"enforce"}}`), repo, base)
	in.Stderr = stderr
	return in
}

func TestCoverChangedFailures_ATestRedOnTheBaseDoesNotRefuseTheLane(t *testing.T) {
	repo, base := coverChangedRepo(t)
	stubCoverChangedProfiler(t, coverChangedFullProfile, "--- FAIL: TestRealTmux (0.10s)\n    tmux_test.go:9: no tmux\nFAIL\tp\n", coverStatusTestsFailed)
	stubSelfCheckRunner(t, "ok\tp", true)
	calls := stubCoverChangedBaseRunner(t, []string{"TestRealTmux"}, nil)
	var stderr injectedStderr

	failures := coverChangedFailures(context.Background(), enforceFloorInput(t, repo, base, &stderr), []string{"go/p/a.go"})

	if failures != nil {
		t.Errorf("failures = %v, want none: the only failing test is red on the base too", failures)
	}
	if want := []baseRunCall{{[]string{"./p"}, []string{"TestRealTmux"}}}; !reflect.DeepEqual(*calls, want) {
		t.Errorf("base runs = %+v, want %+v", *calls, want)
	}
	if note := stderr.String(); !strings.Contains(note, "red on the base "+base+" too") || !strings.Contains(note, "TestRealTmux") {
		t.Errorf("stderr = %q, want a note that names the base-red test", note)
	}
}

func TestCoverChangedFailures_ALaneOnlyRedTestIsRefusedAndNamesTheBaseRed(t *testing.T) {
	repo, base := coverChangedRepo(t)
	stubCoverChangedProfiler(t, coverChangedFullProfile, "--- FAIL: TestRealTmux (0.10s)\n--- FAIL: TestNew (0.00s)\nFAIL\tp\n", coverStatusTestsFailed)
	stubSelfCheckRunner(t, "ok\tp", true)
	calls := stubCoverChangedBaseRunner(t, []string{"TestRealTmux", "TestOther"}, nil)

	failures := coverChangedFailures(context.Background(), enforceFloorInput(t, repo, base, &injectedStderr{}), []string{"go/p/a.go"})

	if len(failures) != 1 {
		t.Fatalf("failures = %v, want one refusal", failures)
	}
	for _, want := range []string{coverChangedTestsFailedHeadline, "red on the lane and green on the base: TestNew", "information: red on the base too, not refused: TestRealTmux"} {
		if !strings.Contains(failures[0], want) {
			t.Errorf("the refusal does not carry %q:\n%s", want, failures[0])
		}
	}
	if strings.Contains(failures[0], "TestOther") {
		t.Errorf("the refusal names TestOther, which did not fail on the lane:\n%s", failures[0])
	}
	if want := []string{"TestNew", "TestRealTmux"}; len(*calls) != 1 || !slices.Equal((*calls)[0].names, want) {
		t.Errorf("base runs = %+v, want one run of %v", *calls, want)
	}
}

func TestCoverChangedFailures_ABaseRunFaultOrNoTestNameCountsEveryFailure(t *testing.T) {
	repo, base := coverChangedRepo(t)
	stubSelfCheckRunner(t, "ok\tp", true)
	for name, tc := range map[string]struct {
		output string
		err    error
		runs   int
		want   string
	}{
		"the base run fails": {"--- FAIL: TestRealTmux (0.10s)\nFAIL\tp\n", errors.New("no base tree"), 1, "could not run the failing tests on the base"},
		"no test name":       {"# p\np/a_test.go:3:1: syntax error\nFAIL\tp [build failed]\n", nil, 0, ""},
	} {
		t.Run(name, func(t *testing.T) {
			stubCoverChangedProfiler(t, coverChangedFullProfile, tc.output, coverStatusTestsFailed)
			calls := stubCoverChangedBaseRunner(t, []string{"TestRealTmux"}, tc.err)
			var stderr injectedStderr

			failures := coverChangedFailures(context.Background(), enforceFloorInput(t, repo, base, &stderr), []string{"go/p/a.go"})

			if len(failures) != 1 || strings.Contains(failures[0], "information:") {
				t.Errorf("failures = %v, want one refusal with no base-red names", failures)
			}
			if len(*calls) != tc.runs || !strings.Contains(stderr.String(), tc.want) {
				t.Errorf("base runs = %d, stderr = %q; want %d runs and %q", len(*calls), stderr.String(), tc.runs, tc.want)
			}
		})
	}
}

func TestFailedTestNames_ReadsTheTopLevelFailuresOnce(t *testing.T) {
	t.Parallel()

	out := "=== RUN   TestA\n--- FAIL: TestB (0.00s)\n    --- FAIL: TestB/sub (0.00s)\n--- FAIL: TestA (0.01s)\n--- PASS: TestC (0.00s)\n--- FAIL: TestB (0.00s)\nFAIL\tp\n"

	if got, want := failedTestNames(out), []string{"TestA", "TestB"}; !slices.Equal(got, want) {
		t.Errorf("failedTestNames = %v, want %v", got, want)
	}
	if got := failedTestNames("ok\tp\n"); got != nil {
		t.Errorf("failedTestNames of a green run = %v, want nil", got)
	}
}

func baseRedRepo(t *testing.T) (*gittest.Repo, string) {
	t.Helper()
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "go.mod"), "module example.com/m\n\ngo 1.23\n")
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "a.go"), "package p\n\nfunc F() int { return 1 }\n")
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "a_test.go"), "package p\n\nimport \"testing\"\n\nfunc TestRed(t *testing.T) { t.Fatal(\"red on the base\") }\n\nfunc TestGreen(t *testing.T) { F() }\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "a_test.go"), "package p\n\nimport \"testing\"\n\nfunc TestRed(t *testing.T) { t.Fatal(\"red on the base\") }\n\nfunc TestGreen(t *testing.T) { t.Fatal(\"red on the lane\") }\n")
	repo.Git("commit", "-q", "-am", "build")
	return repo, base
}

func TestBaseFailingTests_RunsTheNamedTestsOnTheBaseTree(t *testing.T) {
	t.Parallel()
	repo, base := baseRedRepo(t)

	got, err := baseFailingTests(context.Background(), ReviewInput{Worktree: repo.Dir, WorktreeBaseSHA: base}, []string{"./p"}, []string{"TestGreen", "TestRed"})

	if err != nil || !slices.Equal(got, []string{"TestRed"}) {
		t.Errorf("baseFailingTests = %v, %v; want [TestRed]: TestGreen is green on the base", got, err)
	}
}

func TestBaseFailingTests_ABaseThatIsNotACommitIsAnError(t *testing.T) {
	t.Parallel()
	repo, _ := baseRedRepo(t)

	if got, err := baseFailingTests(context.Background(), ReviewInput{Worktree: repo.Dir, WorktreeBaseSHA: "no-such-ref"}, []string{"./p"}, []string{"TestRed"}); err == nil || got != nil {
		t.Errorf("baseFailingTests = %v, %v; want an error", got, err)
	}
}

func TestBaseFailingTests_NoTempDirectoryIsAnError(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))

	if got, err := baseFailingTests(context.Background(), ReviewInput{Worktree: t.TempDir()}, []string{"./p"}, []string{"TestRed"}); err == nil || got != nil {
		t.Errorf("baseFailingTests = %v, %v; want an error", got, err)
	}
}
