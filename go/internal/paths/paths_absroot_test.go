package paths

import (
	"errors"
	"path/filepath"
	"testing"
)

// TestAbsoluteRoot encodes Workstream A: a single shared helper that resolves a
// (possibly relative) project root to an absolute path. Cycle 119/120 proved a
// relative root silently breaks path-crossing contracts (the agent's worktree
// cwd vs the in-process bridge's main cwd), and the fix in commit 80f4206 lived
// only in cmd_loop.go. This helper is the reusable, tested version applied at
// every entrypoint.
func TestAbsoluteRoot(t *testing.T) {
	abs := func(p string) string { a, _ := filepath.Abs(p); return a }
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"dot_becomes_cwd", ".", abs(".")},
		{"relative_is_absolutized", "sub/dir", abs("sub/dir")},
		{"already_absolute_is_idempotent", "/tmp/evolve-x", "/tmp/evolve-x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			warned := false
			got := AbsoluteRoot("--project-root", tc.in, func(string) { warned = true })
			if got != tc.want {
				t.Errorf("AbsoluteRoot(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if !filepath.IsAbs(got) {
				t.Errorf("AbsoluteRoot(%q) = %q, not absolute", tc.in, got)
			}
			if warned {
				t.Errorf("warn fired on success for %q", tc.in)
			}
		})
	}
}

// TestAbsoluteRoot_WarnsAndReturnsInputOnError covers the fail-loud branch: when
// filepath.Abs fails (os.Getwd failure — cwd deleted/unmounted), the helper must
// WARN and return the original (never silently continue with a broken state).
func TestAbsoluteRoot_WarnsAndReturnsInputOnError(t *testing.T) {
	orig := absFn
	defer func() { absFn = orig }()
	absFn = func(string) (string, error) { return "", errors.New("getwd failed") }

	var msg string
	got := AbsoluteRoot("--project-root", "rel/path", func(m string) { msg = m })
	if got != "rel/path" {
		t.Errorf("on Abs error: got %q, want the original input %q", got, "rel/path")
	}
	if msg == "" {
		t.Error("expected a WARN message on Abs error, got none")
	}
}
