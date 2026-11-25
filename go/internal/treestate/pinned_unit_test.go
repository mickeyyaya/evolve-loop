package treestate

import (
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func TestDiffArgs_PinsEveryPresentationInputAndEndsWithTheRevisions(t *testing.T) {
	t.Parallel()
	got := strings.Join(DiffArgs("a", "b"), " ")
	want := "-c core.quotePath=false -c diff.suppressBlankEmpty=false -c core.bigFileThreshold=512m diff --full-index --no-ext-diff --no-textconv --no-color --no-renames --no-relative --src-prefix=a/ --dst-prefix=b/ --diff-algorithm=myers --indent-heuristic -U3 --inter-hunk-context=0 --ignore-submodules=dirty --submodule=short -O/dev/null --end-of-options a b"
	if got != want {
		t.Errorf("DiffArgs = %q\nwant       %q", got, want)
	}
}

func TestDiffArgs_ReturnsAFreshSliceEachCall(t *testing.T) {
	t.Parallel()
	first := DiffArgs("HEAD")
	first[0] = "tampered"
	if DiffArgs("HEAD")[0] == "tampered" {
		t.Error("DiffArgs shares its backing array between calls")
	}
}

func captureRun(out string, exit int, runErr error, gotArgs *[]string, gotEnv *[]string) sysexec.RunFunc {
	return func(_ context.Context, _, _ string, args, env []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
		*gotArgs = args
		*gotEnv = env
		_, _ = io.WriteString(stdout, out)
		return exit, runErr
	}
}

func TestPinned_RunsTheDiffArgsAndScrubsGitDiffOptsFromTheChildEnv(t *testing.T) {
	t.Parallel()
	var args, env []string
	given := []string{"PATH=/bin", "GIT_DIFF_OPTS=--unified=1", "HOME=/h"}
	if _, err := Pinned(context.Background(), captureRun("", 0, nil, &args, &env), "", given); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(args, DiffArgs("HEAD")) {
		t.Errorf("args = %v, want DiffArgs(HEAD)", args)
	}
	if want := []string{"PATH=/bin", "HOME=/h"}; !slices.Equal(env, want) {
		t.Errorf("env = %v, want %v", env, want)
	}
	if len(given) != 3 || given[1] != "GIT_DIFF_OPTS=--unified=1" {
		t.Errorf("Pinned mutated the caller env: %v", given)
	}
}

func TestPinned_ANilEnvInheritsTheProcessEnvWithoutGitDiffOpts(t *testing.T) {
	t.Parallel()
	var args, env []string
	if _, err := Pinned(context.Background(), captureRun("", 0, nil, &args, &env), "", nil); err != nil {
		t.Fatal(err)
	}
	if len(env) == 0 {
		t.Error("a nil env reached git as empty, want the process environment")
	}
	for _, kv := range env {
		if strings.HasPrefix(kv, "GIT_DIFF_OPTS=") {
			t.Errorf("env still holds %s", kv)
		}
	}
}

func TestPinnedBetween_RunsTheTwoRevisions(t *testing.T) {
	t.Parallel()
	var args, env []string
	if _, err := PinnedBetween(context.Background(), captureRun("", 0, nil, &args, &env), "", nil, "p", "c"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(args, DiffArgs("p", "c")) {
		t.Errorf("args = %v, want DiffArgs(p, c)", args)
	}
}

func TestPinned_StripsOnlyTheHunkHeaderFunctionContext(t *testing.T) {
	t.Parallel()
	var args, env []string
	raw := "diff --git a/f b/f\n@@ -1,2 +1,2 @@ func Host()\n-@@ -1 +1 @@ stays\n+@@ -1 +1 @@ stays\n@@ -9 +9 @@\n ctx @@ keep\n"
	stripped := "diff --git a/f b/f\n@@ -1,2 +1,2 @@\n-@@ -1 +1 @@ stays\n+@@ -1 +1 @@ stays\n@@ -9 +9 @@\n ctx @@ keep\n"
	got, err := Pinned(context.Background(), captureRun(raw, 1, nil, &args, &env), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := sha256Hex(stripped); got != want {
		t.Errorf("digest = %s, want the digest of the stripped text %s", got, want)
	}
}

func TestPinned_FailuresCarryTheRealArgv(t *testing.T) {
	t.Parallel()
	var args, env []string
	_, err := Pinned(context.Background(), captureRun("", 128, nil, &args, &env), "", nil)
	var re *RunError
	if !errors.As(err, &re) {
		t.Fatalf("err = %v, want a *RunError", err)
	}
	if re.ExitCode != 128 || !slices.Equal(re.Args, DiffArgs("HEAD")) {
		t.Errorf("RunError = %+v, want exit 128 and the pinned argv", re)
	}
	boom := errors.New("boom")
	_, err = PinnedBetween(context.Background(), captureRun("", -1, boom, &args, &env), "", nil, "p", "c")
	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want it to wrap boom", err)
	}
	if !errors.As(err, &re) || !slices.Equal(re.Args, DiffArgs("p", "c")) {
		t.Errorf("RunError = %+v, want the pinned argv of p and c", re)
	}
}

func TestRunError_PrintsTheRealArgv(t *testing.T) {
	t.Parallel()
	e := &RunError{ExitCode: 2, Stderr: "bad", Args: []string{"-c", "k=v", "diff", "HEAD"}}
	if got, want := e.Error(), "git -c k=v diff HEAD exit 2: bad"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
