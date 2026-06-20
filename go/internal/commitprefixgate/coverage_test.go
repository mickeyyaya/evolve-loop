package commitprefixgate

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// fixedTime is a deterministic timestamp for log-append edge tests.
func fixedTime() time.Time { return time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC) }

// coverage_test.go targets the branches the behavior suite in
// commitprefixgate_test.go does not yet exercise: the SHIP_CLASS-defaulting
// bypass-deny path, the matchPath `**`-collapse and `prefix/**` forms, the
// globMatchRec `?`/`[` end-of-path edges, defaultGetDiffPaths via the execGit
// seam (no real git subprocess), and appendGuardsLog's empty-path and
// open-error early returns.

// === Bypass with empty SHIP_CLASS defaults to "cycle" and is denied ========
// Hits the `shipClass == ""` default branch (commitprefixgate.go:131-133):
// an empty class must resolve to "cycle", which is NOT manual, so bypass is
// rejected with ErrScopeViolation — proving an unset ship class can never
// silently unlock the bypass.
func TestRun_Bypass_EmptyShipClassDefaultsToCycle(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t, "")
	res, err := Run(Options{
		CommitMsg:    "feat: msg",
		RepoDir:      repo,
		Bypass:       true,
		ShipClass:    "", // unset → defaults to "cycle" → bypass denied
		GetDiffPaths: stubDiffPaths(nil),
	})
	if !errors.Is(err, ErrScopeViolation) {
		t.Fatalf("err = %v, want ErrScopeViolation", err)
	}
	if res.Allowed {
		t.Error("Allowed = true, want false (empty ship class must not unlock bypass)")
	}
}

// === matchPath: `**`-collapse and `prefix/**` HasPrefix branches ===========
func TestMatchPath_CollapseAndPrefixForms(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, pat, path string
		want            bool
	}{
		// `**` between literals: bash case-glob crosses `/`, so a double-star
		// mid-pattern matches a nested path.
		{"double-star-mid", "go**internal", "go/x/internal", true},
		// `prefix/**` matches any path nested under the prefix.
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

// === globMatchRec: `?` and `[` at end-of-path edges ========================
func TestGlobMatch_EndOfPathEdges(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, pat, path string
		want            bool
	}{
		// `?` when path is exhausted (si >= len(path)) → false (line 357-359).
		{"question-past-end", "ab?", "ab", false},
		// `[` class when path is exhausted (si >= len(path)) → false (line 368-370).
		{"class-past-end", "ab[cd]", "ab", false},
		// `[` class with no closing `]` → false (end >= len(pat), line 368-370).
		{"class-unterminated", "a[bc", "abc", false},
		// sanity: a valid class still matches so the edge cases above are the
		// only thing failing, not the class machinery itself.
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

// === defaultGetDiffPaths via execGit seam (no real git) ====================
// Overrides the package-level execGit var to return a deterministic command
// (printf-style via `exec.Command`), so the production code path is exercised
// without a real git repo, subprocess timing, or network. Mutates a package
// global → no t.Parallel; restored via t.Cleanup.

func swapExecGit(t *testing.T, fn func(args ...string) *exec.Cmd) {
	t.Helper()
	prev := execGit
	execGit = fn
	t.Cleanup(func() { execGit = prev })
}

func TestDefaultGetDiffPaths_StagedParsesAndTrims(t *testing.T) {
	swapExecGit(t, func(args ...string) *exec.Cmd {
		// Assert the staged-mode argv is exactly what the bash port shelled out.
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
		// Output with a trailing newline + a blank line → blanks filtered out.
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
		// A command that exits non-zero → cmd.Output() returns an error.
		return exec.Command("false")
	})
	_, err := defaultGetDiffPaths("/repo", ModeStaged, "")
	if err == nil {
		t.Fatal("err = nil, want non-zero exit error propagated")
	}
}

// === Run wires defaultGetDiffPaths when no seam is provided ================
// Leaving GetDiffPaths nil forces the `opts.GetDiffPaths == nil` default
// branch (commitprefixgate.go:113-115); execGit is stubbed so no real git runs.
func TestRun_DefaultsGetDiffPathsWhenNil(t *testing.T) {
	swapExecGit(t, func(args ...string) *exec.Cmd {
		return exec.Command("printf", "go/internal/x.go\n")
	})
	repo := makeRepo(t, `{"prefixes":{"feat":{"required_paths":["go/**"]}}}`)
	res, err := Run(Options{
		CommitMsg: "feat: real change",
		RepoDir:   repo,
		Mode:      ModeStaged,
		// GetDiffPaths intentionally nil → defaultGetDiffPaths via stubbed execGit
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

// === appendGuardsLog edges ==================================================
func TestAppendGuardsLog_EmptyPathNoOp(t *testing.T) {
	t.Parallel()
	// Empty path → early return, no panic, nothing written.
	appendGuardsLog("", fixedTime(), "ignored")
}

func TestAppendGuardsLog_OpenErrorIsSwallowed(t *testing.T) {
	t.Parallel()
	// Point the log at a path whose parent is a regular FILE, so MkdirAll and
	// OpenFile both fail — appendGuardsLog must swallow the error (best-effort).
	dir := t.TempDir()
	notADir := filepath.Join(dir, "afile")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	logPath := filepath.Join(notADir, "guards.log") // parent is a file → open fails
	appendGuardsLog(logPath, fixedTime(), "should not panic")
	// The open failed and was swallowed: no readable log exists at the path.
	if _, err := os.ReadFile(logPath); err == nil {
		t.Errorf("guards.log unexpectedly readable at %q despite open error", logPath)
	}
}
