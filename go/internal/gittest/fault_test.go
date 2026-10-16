package gittest

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func gitExit(t *testing.T, dir string, args ...string) (stdout string, code int) {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return string(out), 0
	case errors.As(err, &exitErr):
		return string(out), exitErr.ExitCode()
	default:
		t.Fatalf("git %v did not run: %v", args, err)
		return "", -1
	}
}

func committedFixture(t *testing.T) *Repo {
	t.Helper()
	r := Fixture(t)
	r.Git("commit", "-q", "--allow-empty", "-m", "base")
	return r
}

func TestFaultOn_VerbFails(t *testing.T) {
	r := committedFixture(t)

	FaultOn(t, "status")

	cmd := exec.Command("git", "-C", r.Dir, "status")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 128 {
		t.Fatalf("faulted status err = %v, want exit 128", err)
	}
	if !strings.Contains(stderr.String(), "fatal:") || !strings.Contains(stderr.String(), "status") {
		t.Errorf("stderr = %q, want a fatal: line naming the verb", stderr.String())
	}
}

func TestFaultOn_NonTargetedVerbPasses(t *testing.T) {
	r := committedFixture(t)
	head := r.Git("rev-parse", "HEAD")

	FaultOn(t, "show")

	if out, code := gitExit(t, r.Dir, "rev-parse", "HEAD"); code != 0 || strings.TrimSpace(out) != head {
		t.Errorf("rev-parse = %q exit %d, want %s", out, code, head)
	}
	if out, code := gitExit(t, r.Dir, "show-ref", "--verify", "refs/heads/main"); code != 0 || !strings.Contains(out, head) {
		t.Errorf("show-ref under a show fault = %q exit %d, want the real answer", out, code)
	}
	if _, code := gitExit(t, r.Dir, "show", "-s", "HEAD"); code == 0 {
		t.Error("show under a show fault exited 0")
	}
}

func TestFaultOn_SkipCount(t *testing.T) {
	cases := []struct {
		name      string
		skip      int
		wantCodes []int
	}{
		{"no skip fails the first call", 0, []int{5, 5}},
		{"skip one lets one call through", 1, []int{0, 5, 5}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := committedFixture(t)

			InstallFault(t, GitFault{Verb: "status", Skip: tc.skip, Code: 5})

			for i, want := range tc.wantCodes {
				if _, code := gitExit(t, r.Dir, "status", "--porcelain"); code != want {
					t.Errorf("call %d exit = %d, want %d", i+1, code, want)
				}
			}
		})
	}
}

func TestBranchExists(t *testing.T) {
	r := committedFixture(t)
	r.Git("branch", "feature")
	r.Git("tag", "v1")
	cases := []struct {
		name, dir, branch string
		want              bool
	}{
		{"present", r.Dir, "feature", true},
		{"absent", r.Dir, "nope", false},
		{"tag", r.Dir, "v1", false},
		{"empty", r.Dir, "", false},
		{"not a repo", t.TempDir(), "main", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := BranchExists(t, tc.dir, tc.branch); got != tc.want {
				t.Errorf("BranchExists(%q) = %v, want %v", tc.branch, got, tc.want)
			}
		})
	}
}
