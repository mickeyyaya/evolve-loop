//go:build integration

package core

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
)

func writeModuleFiles(t *testing.T, moduleDir string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		path := filepath.Join(moduleDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func stubGoTool(t *testing.T, script string) {
	t.Helper()
	bin := t.TempDir()
	fakeclitest.Install(t, filepath.Join(bin, "go"), "#!/bin/sh\n"+script+"\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func realTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestBuildTagVisiblePackages_DropsOnlyPackagesWithNoDefaultTagFiles(t *testing.T) {
	t.Parallel()

	moduleDir := filepath.Join(realTempDir(t), "go")
	writeModuleFiles(t, moduleDir, map[string]string{
		"go.mod":             "module example.com/tagfixture\n\ngo 1.23\n",
		"src/s.go":           "package src\n",
		"onlytest/o_test.go": "package onlytest\n",
		"xtest/x_test.go":    "package xtest_test\n",
		"tagged/t_test.go":   "//go:build acs\n\npackage tagged\n",
	})
	in := []string{"./src", "./onlytest", "./xtest", "./tagged", "./missing"}
	var got []string

	var sink injectedStderr
	got = buildTagVisiblePackages(context.Background(), moduleDir, in, &sink)
	stderr := sink.String()

	if want := []string{"./src", "./onlytest", "./xtest", "./missing"}; !slices.Equal(got, want) {
		t.Fatalf("buildTagVisiblePackages = %q, want %q (an unresolvable package stays for the floor to judge)", got, want)
	}
	if !strings.Contains(stderr, "./tagged has no Go files under the default build tags") || strings.Contains(stderr, "./src has no") {
		t.Fatalf("stderr = %q, want a NOTE for ./tagged only", stderr)
	}
}

func TestBuildTagVisiblePackages_AllVisibleKeepsTheInputUnchanged(t *testing.T) {
	t.Parallel()

	moduleDir := filepath.Join(t.TempDir(), "go")
	writeModuleFiles(t, moduleDir, map[string]string{
		"go.mod":   "module example.com/tagfixture\n\ngo 1.23\n",
		"src/s.go": "package src\n",
	})
	in := []string{"./src"}

	got := buildTagVisiblePackages(context.Background(), moduleDir, in, io.Discard)

	if !slices.Equal(got, in) {
		t.Fatalf("buildTagVisiblePackages = %q, want %q", got, in)
	}
}

func TestBuildTagVisiblePackages_GoListFailureFailsOpenWithAWarning(t *testing.T) {
	t.Parallel()

	in := []string{"./a", "./b"}
	var got []string

	var sink injectedStderr
	got = buildTagVisiblePackages(context.Background(), t.TempDir(), in, &sink)
	stderr := sink.String()

	if !slices.Equal(got, in) || !strings.Contains(stderr, "[build-floor] WARN: go list failed") {
		t.Fatalf("buildTagVisiblePackages = %q, stderr %q; want the input kept and the WARN", got, stderr)
	}
}

func TestScopedCoverFunc_GreenRunWritesTheFuncReport(t *testing.T) {
	t.Parallel()

	moduleDir := filepath.Join(t.TempDir(), "go")
	writeCoverFixture(t, moduleDir)

	path, output, status := scopedCoverFunc(context.Background(), ReviewInput{}, moduleDir, []string{"./p"})
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(path)) })

	if status != coverStatusOK || output != "" || filepath.Base(path) != "cover.func.txt" {
		t.Fatalf("scopedCoverFunc = %q, %q, %d; want the func report path and coverStatusOK", path, output, status)
	}
	report, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(report), "p/p.go:3:") || !strings.Contains(string(report), "total:") {
		t.Fatalf("func report = %q, %v; want One and the total", report, err)
	}
}

func TestScopedCoverFunc_FailingTestsAreTestsFailedWithTheirOutput(t *testing.T) {
	t.Parallel()

	moduleDir := filepath.Join(t.TempDir(), "go")
	writeCoverFixture(t, moduleDir)
	writeModuleFiles(t, moduleDir, map[string]string{
		"p/red_test.go": "package p\n\nimport \"testing\"\n\nfunc TestRed(t *testing.T) { t.Fatal(\"red-marker\") }\n",
	})

	path, output, status := scopedCoverFunc(context.Background(), ReviewInput{}, moduleDir, []string{"./p"})
	t.Cleanup(func() { _ = os.RemoveAll(path) })

	if status != coverStatusTestsFailed || !strings.HasSuffix(path, "/") || !strings.Contains(output, "red-marker") {
		t.Fatalf("scopedCoverFunc = %q, %q, %d; want the temp dir, the red output and coverStatusTestsFailed", path, output, status)
	}
}

func TestScopedCoverFunc_PlumbingFailuresAreNeverTestFailures(t *testing.T) {
	for _, tc := range []struct {
		name, script, wantOutput string
		missingTmp               bool
	}{
		{name: "no temp dir", missingTmp: true, wantOutput: "no such file or directory"},
		{name: "cover tool fails", script: `case "$1" in test) exit 0;; *) echo cover-broke >&2; exit 3;; esac`, wantOutput: "go tool cover: exit status 3"},
		{name: "func report unwritable", script: `case "$1" in test) exit 0;; *) p="${3#-func=}"; chmod 555 "$(dirname "$p")"; echo report;; esac`, wantOutput: "permission denied"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			moduleDir := filepath.Join(t.TempDir(), "go")
			writeCoverFixture(t, moduleDir)
			tmp := t.TempDir()
			if tc.missingTmp {
				tmp = filepath.Join(tmp, "absent")
			}
			t.Setenv("TMPDIR", tmp)
			if tc.script != "" {
				stubGoTool(t, tc.script)
			}

			path, output, status := scopedCoverFunc(context.Background(), ReviewInput{}, moduleDir, []string{"./p"})
			if path != "" {
				t.Cleanup(func() { _ = os.Chmod(path, 0o755) })
			}

			if status != coverStatusPlumbingError || !strings.Contains(output, tc.wantOutput) {
				t.Fatalf("scopedCoverFunc = %q, %q, %d; want coverStatusPlumbingError naming %q", path, output, status, tc.wantOutput)
			}
		})
	}
}
