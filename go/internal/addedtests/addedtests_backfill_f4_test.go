package addedtests

import (
	"errors"
	"go/build"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeSourceF4(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBuildTags_TableOfConstraintShapes(t *testing.T) {
	cases := []struct {
		name         string
		file         string
		content      string
		wantTags     []string
		wantRunnable bool
	}{
		{"no constraint builds by default", "a_test.go", "package a\n", nil, true},
		{"foreign GOOS suffix has no go:build line and is unrunnable", "a_windows_test.go", "package a\n", nil, false},
		{"single tag", "a_test.go", "//go:build acs\n\npackage a\n", []string{"acs"}, true},
		{"or picks the smallest satisfying selection", "a_test.go", "//go:build acs || e2e\n\npackage a\n", []string{"acs"}, true},
		{"and needs both tags sorted", "a_test.go", "//go:build e2e && acs\n\npackage a\n", []string{"acs", "e2e"}, true},
		{"negation of one tag is satisfied by the other", "a_test.go", "//go:build !acs && e2e\n\npackage a\n", []string{"e2e"}, true},
		{"unsatisfiable constraint exhausts every selection", "a_test.go", "//go:build acs && !acs\n\npackage a\n", nil, false},
		{"requires_tmux is excluded", "a_test.go", "//go:build requires_tmux\n\npackage a\n", nil, false},
		{"more than 12 tags is excluded", "a_test.go", "//go:build " + strings.Join([]string{"t1", "t2", "t3", "t4", "t5", "t6", "t7", "t8", "t9", "t10", "t11", "t12", "t13"}, " && ") + "\n\npackage a\n", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tags, runnable, err := BuildTags(writeSourceF4(t, tc.file, tc.content))
			if err != nil {
				t.Fatal(err)
			}
			if runnable != tc.wantRunnable || !reflect.DeepEqual(tags, tc.wantTags) {
				t.Fatalf("tags=%v runnable=%v, want tags=%v runnable=%v", tags, runnable, tc.wantTags, tc.wantRunnable)
			}
		})
	}
}

func TestBuildTags_MissingFileReturnsTheMatchError(t *testing.T) {
	tags, runnable, err := BuildTags(filepath.Join(t.TempDir(), "absent_test.go"))
	if err == nil || runnable || tags != nil {
		t.Fatalf("tags=%v runnable=%v err=%v", tags, runnable, err)
	}
}

func TestBuildTags_MatchFailureDuringSearchIsReturned(t *testing.T) {
	orig := matchFileWithContext
	t.Cleanup(func() { matchFileWithContext = orig })
	boom := errors.New("read vanished")
	matchFileWithContext = func(build.Context, string, string) (bool, error) { return false, boom }

	tags, runnable, err := BuildTags(writeSourceF4(t, "a_test.go", "//go:build acs\n\npackage a\n"))
	if !errors.Is(err, boom) || runnable || tags != nil {
		t.Fatalf("tags=%v runnable=%v err=%v", tags, runnable, err)
	}
}

func TestBuildTags_ConstraintParseErrorIsReturned(t *testing.T) {
	path := writeSourceF4(t, "a_test.go", "//go:build acs &&\n\npackage a\n")
	tags, runnable, err := BuildTags(path)
	if err == nil || runnable || tags != nil {
		t.Fatalf("tags=%v runnable=%v err=%v", tags, runnable, err)
	}
}

func TestGoBuildExpr_Shapes(t *testing.T) {
	t.Run("missing file is an open error", func(t *testing.T) {
		if expr, err := goBuildExpr(filepath.Join(t.TempDir(), "nope.go")); err == nil || expr != nil {
			t.Fatalf("expr=%v err=%v", expr, err)
		}
	})
	t.Run("code before any constraint stops the scan", func(t *testing.T) {
		expr, err := goBuildExpr(writeSourceF4(t, "a.go", "package a\n\n//go:build acs\n"))
		if err != nil || expr != nil {
			t.Fatalf("a constraint after the package clause is not a build constraint; expr=%v err=%v", expr, err)
		}
	})
	t.Run("leading comments and blanks are skipped", func(t *testing.T) {
		expr, err := goBuildExpr(writeSourceF4(t, "a.go", "// header\n\n//go:build acs\n\npackage a\n"))
		if err != nil || expr == nil || expr.String() != "acs" {
			t.Fatalf("expr=%v err=%v", expr, err)
		}
	})
	t.Run("an over-long line surfaces the scanner error", func(t *testing.T) {
		expr, err := goBuildExpr(writeSourceF4(t, "a.go", "// "+strings.Repeat("x", 70*1024)+"\npackage a\n"))
		if err == nil || expr != nil {
			t.Fatalf("expr=%v err=%v", expr, err)
		}
	})
	t.Run("empty file has no expression and no error", func(t *testing.T) {
		expr, err := goBuildExpr(writeSourceF4(t, "a.go", ""))
		if err != nil || expr != nil {
			t.Fatalf("expr=%v err=%v", expr, err)
		}
	})
}
