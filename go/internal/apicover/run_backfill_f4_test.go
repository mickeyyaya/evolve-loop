package apicover

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTreeF4(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestRun_MeasurementFailuresReturnCodeTwoWithTheCause(t *testing.T) {
	cases := []struct {
		name    string
		files   map[string]string
		dirFn   func(dir string) string
		wantErr string
	}{
		{"missing directory fails at enumerate", nil, func(d string) string { return filepath.Join(d, "absent") }, "no such file"},
		{"unparseable non-test source fails at enumerate", map[string]string{"a.go": "package a\nfunc ("}, nil, "expected"},
		{"unparseable test file fails at reference scan", map[string]string{"a.go": "package a\n", "a_test.go": "package a\nfunc ("}, nil, "expected"},
		{"no go.mod above the package fails at import path", map[string]string{"a.go": "package a\n"}, nil, "go.mod not found above"},
		{"go.mod without a module line fails at import path", map[string]string{"go.mod": "go 1.23\n", "a.go": "package a\n"}, nil, "no module line"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeTreeF4(t, tc.files)
			if tc.dirFn != nil {
				dir = tc.dirFn(dir)
			}
			var buf bytes.Buffer
			code, err := Run(context.Background(), Config{Dirs: []string{dir}}, &buf)
			if code != 2 || err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("code=%d err=%v, want code 2 and %q", code, err, tc.wantErr)
			}
			if buf.Len() != 0 {
				t.Errorf("a failed measurement must print no report: %q", buf.String())
			}
		})
	}
}

func TestRun_SecondDirFailureStillReportsTheFirstDirAndReturnsTwo(t *testing.T) {
	good := writeTreeF4(t, map[string]string{"go.mod": "module example.com/m\n", "a.go": "package a\n"})
	var buf bytes.Buffer
	code, err := Run(context.Background(), Config{Dirs: []string{good, filepath.Join(good, "absent")}}, &buf)
	if code != 2 || err == nil {
		t.Fatalf("code=%d err=%v", code, err)
	}
	if !strings.Contains(buf.String(), "== "+good+" ==") {
		t.Errorf("first directory must already be reported: %q", buf.String())
	}
}

func TestLoadCoverByPath_Shapes(t *testing.T) {
	t.Run("empty path is an empty map", func(t *testing.T) {
		got, err := loadCoverByPath("")
		if err != nil || len(got) != 0 {
			t.Fatalf("got=%v err=%v", got, err)
		}
	})
	t.Run("absent file is an open error", func(t *testing.T) {
		if got, err := loadCoverByPath(filepath.Join(t.TempDir(), "absent")); err == nil || got != nil {
			t.Fatalf("got=%v err=%v", got, err)
		}
	})
	t.Run("a line past the scanner ceiling is a parse error", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "cover.func")
		if err := os.WriteFile(path, []byte(strings.Repeat("x", 1100*1024)+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got, err := loadCoverByPath(path); err == nil || got != nil {
			t.Fatalf("got=%v err=%v", got, err)
		}
	})
	t.Run("entries are keyed by path and line", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "cover.func")
		body := "example.com/m/a.go:12:\tFoo\t87.5%\ntotal:\t(statements)\t90.0%\n"
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := loadCoverByPath(path)
		if err != nil || len(got) != 1 || got["example.com/m/a.go:12"] != 87.5 {
			t.Fatalf("got=%v err=%v", got, err)
		}
	})
}

func TestPackageImportPath_ModuleRootAndNestedDirectory(t *testing.T) {
	root := writeTreeF4(t, map[string]string{"go.mod": "module example.com/m\n", "sub/pkg/a.go": "package pkg\n"})
	if got, err := packageImportPath(root); err != nil || got != "example.com/m" {
		t.Fatalf("root: got=%q err=%v", got, err)
	}
	if got, err := packageImportPath(filepath.Join(root, "sub", "pkg")); err != nil || got != "example.com/m/sub/pkg" {
		t.Fatalf("nested: got=%q err=%v", got, err)
	}
}

func TestModuleLine_Shapes(t *testing.T) {
	cases := map[string]string{
		"module example.com/m\n":            "example.com/m",
		"go 1.23\n\n  module  spaced/m  \n": "spaced/m",
		"go 1.23\n":                         "",
		"":                                  "",
		"modulex example.com/m\n":           "",
	}
	for in, want := range cases {
		if got := moduleLine([]byte(in)); got != want {
			t.Errorf("moduleLine(%q) = %q, want %q", in, got, want)
		}
	}
}
