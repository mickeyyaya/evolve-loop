package bridge

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func FuzzWorktreeStatusPathsBackfillF1(f *testing.F) {
	for _, seed := range []string{"", "?? a\x00", " M dir/b.go\x00?? c\x00", "XY \x00", "\x00\x00", "ab"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, out string) {
		runner := func(_ context.Context, _, _ string, _, _ []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
			_, _ = io.WriteString(stdout, out)
			return 0, nil
		}
		paths, err := worktreeStatusPaths(context.Background(), "/wt", Deps{Runner: runner})
		if err != nil {
			t.Fatalf("a zero-exit git status can never be an error: %v", err)
		}
		for _, p := range paths {
			if p == "" || !strings.Contains(out, p) {
				t.Fatalf("path %q is not a suffix of a record in %q", p, out)
			}
		}
	})
}

func FuzzWithinDirBackfillF1(f *testing.F) {
	for _, seed := range [][2]string{{"/a", "/a"}, {"/a", "/a/b"}, {"/a", "/ab"}, {"", "/a"}, {"/a/b", "/a"}, {"rel", "/abs"}} {
		f.Add(seed[0], seed[1])
	}
	f.Fuzz(func(t *testing.T, dir, path string) {
		got := withinDir(dir, path)
		if dir == "" && got {
			t.Fatalf("an empty directory contains nothing, got true for %q", path)
		}
		if dir != "" && !withinDir(dir, dir) {
			t.Fatalf("a directory must contain itself: %q", dir)
		}
		if dir != "" && !withinDir(dir, filepath.Join(dir, "child")) {
			t.Fatalf("a child of %q must be inside it", dir)
		}
	})
}
