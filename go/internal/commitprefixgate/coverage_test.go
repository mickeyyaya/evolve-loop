package commitprefixgate

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func fixedTime() time.Time { return time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC) }

func TestRun_Bypass_EmptyShipClassDefaultsToCycle(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t, "")
	res, err := Run(Options{
		CommitMsg:    "feat: msg",
		RepoDir:      repo,
		Bypass:       true,
		ShipClass:    "",
		GetDiffPaths: stubDiffPaths(nil),
	})
	if !errors.Is(err, ErrScopeViolation) {
		t.Fatalf("err = %v, want ErrScopeViolation", err)
	}
	if res.Allowed {
		t.Error("Allowed = true, want false (empty ship class must not unlock bypass)")
	}
}

func TestMatchPath_CollapseAndPrefixForms(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, pat, path string
		want            bool
	}{
		{"double-star-mid", "go**internal", "go/x/internal", true},
		{"prefix-slash-star-nested", "docs/**", "docs/a/b/c.md", true},
		{"prefix-slash-star-non-member", "docs/**", "src/a.md", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := matchPath(tc.pat, tc.path); got != tc.want {
				t.Errorf("matchPath(%q, %q) = %v, want %v", tc.pat, tc.path, got, tc.want)
			}
		})
	}
}

func TestGlobMatch_EndOfPathEdges(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, pat, path string
		want            bool
	}{
		{"question-past-end", "ab?", "ab", false},
		{"class-past-end", "ab[cd]", "ab", false},
		{"class-unterminated", "a[bc", "abc", false},
		{"class-valid", "a[bc]", "ab", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := globMatch(tc.pat, tc.path); got != tc.want {
				t.Errorf("globMatch(%q, %q) = %v, want %v", tc.pat, tc.path, got, tc.want)
			}
		})
	}
}

func swapExecGit(t *testing.T, fn func(args ...string) *exec.Cmd) {
	t.Helper()
	prev := execGit
	execGit = fn
	t.Cleanup(func() { execGit = prev })
}

func TestDefaultGetDiffPaths_StagedParsesAndTrims(t *testing.T) {
	swapExecGit(t, func(args ...string) *exec.Cmd {
		want := []string{"-C", "/repo", "diff", "--cached", "--name-only"}
		if len(args) != len(want) {
			t.Errorf("argv = %v, want %v", args, want)
		} else {
			for i := range want {
				if args[i] != want[i] {
					t.Errorf("argv[%d] = %q, want %q", i, args[i], want[i])
				}
			}
		}
		return exec.Command("printf", "a.go\nb.go\n\n")
	})
	got, err := defaultGetDiffPaths("/repo", ModeStaged, "")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	want := []string{"a.go", "b.go"}
	if len(got) != len(want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("paths[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDefaultGetDiffPaths_RefModeBuildsRangeArgv(t *testing.T) {
	swapExecGit(t, func(args ...string) *exec.Cmd {
		want := []string{"-C", "/repo", "diff", "main..HEAD", "--name-only"}
		if len(args) != len(want) {
			t.Fatalf("argv = %v, want %v", args, want)
		}
		for i := range want {
			if args[i] != want[i] {
				t.Errorf("argv[%d] = %q, want %q", i, args[i], want[i])
			}
		}
		return exec.Command("printf", "x.go\n")
	})
	got, err := defaultGetDiffPaths("/repo", ModeRef, "main")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 1 || got[0] != "x.go" {
		t.Errorf("paths = %v, want [x.go]", got)
	}
}

func TestDefaultGetDiffPaths_RefModeRequiresDiffRef(t *testing.T) {
	swapExecGit(t, func(args ...string) *exec.Cmd {
		t.Error("execGit should NOT be called when ModeRef has empty diffRef")
		return exec.Command("true")
	})
	_, err := defaultGetDiffPaths("/repo", ModeRef, "")
	if err == nil {
		t.Fatal("err = nil, want diff-ref-required error")
	}
	if err.Error() != "diff ref required for ModeRef" {
		t.Errorf("err = %q, want %q", err, "diff ref required for ModeRef")
	}
}

func TestDefaultGetDiffPaths_CommandError(t *testing.T) {
	swapExecGit(t, func(args ...string) *exec.Cmd {
		return exec.Command("false")
	})
	_, err := defaultGetDiffPaths("/repo", ModeStaged, "")
	if err == nil {
		t.Fatal("err = nil, want non-zero exit error propagated")
	}
}

func TestRun_DefaultsGetDiffPathsWhenNil(t *testing.T) {
	swapExecGit(t, func(args ...string) *exec.Cmd {
		return exec.Command("printf", "go/internal/x.go\n")
	})
	repo := makeRepo(t, `{"prefixes":{"feat":{"required_paths":["go/**"]}}}`)
	res, err := Run(Options{
		CommitMsg: "feat: real change",
		RepoDir:   repo,
		Mode:      ModeStaged,
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !res.Allowed {
		t.Errorf("res = %+v, want Allowed=true (go/ path satisfies required_paths)", res)
	}
	if len(res.DiffPaths) != 1 || res.DiffPaths[0] != "go/internal/x.go" {
		t.Errorf("DiffPaths = %v, want [go/internal/x.go]", res.DiffPaths)
	}
}

func TestAppendGuardsLog_EmptyPathNoOp(t *testing.T) {
	t.Parallel()
	appendGuardsLog("", fixedTime(), "ignored")
}

func TestAppendGuardsLog_OpenErrorIsSwallowed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	notADir := filepath.Join(dir, "afile")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	logPath := filepath.Join(notADir, "guards.log")
	appendGuardsLog(logPath, fixedTime(), "should not panic")
	if _, err := os.ReadFile(logPath); err == nil {
		t.Errorf("guards.log unexpectedly readable at %q despite open error", logPath)
	}
}
