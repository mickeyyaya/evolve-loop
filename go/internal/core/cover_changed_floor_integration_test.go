//go:build integration

package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/covchanged"
)

func coverChangedModule(t *testing.T, testBody string) string {
	t.Helper()
	module := filepath.Join(t.TempDir(), "go")
	writeFile(t, filepath.Join(module, "go.mod"), "module example.com/m\n\ngo 1.23\n")
	writeFile(t, filepath.Join(module, "p", "a.go"), "package p\n\nfunc F(x int) int {\n\tif x > 0 {\n\t\treturn 1\n\t}\n\treturn x\n}\n")
	writeFile(t, filepath.Join(module, "p", "a_test.go"), "//go:build integration\n\npackage p\n\nimport \"testing\"\n\nfunc TestF(t *testing.T) {\n"+testBody+"}\n")
	return module
}

func TestScopedCoverProfile_RunsThePackagesUnderTheGivenTags(t *testing.T) {
	t.Parallel()

	module := coverChangedModule(t, "\tif F(1) != 1 {\n\t\tt.Fatal(\"F(1)\")\n\t}\n")

	profile, out, status := scopedCoverProfile(context.Background(), module, []string{"./p"}, []string{"integration"})
	defer func() { _ = os.RemoveAll(filepath.Dir(profile)) }()

	if branch, hit := fCoverage(t, profile); status != coverStatusOK || !branch || !hit {
		t.Errorf("status %d out %q branch hit %v; want the integration test to have run the branch that returns 1", status, out, branch)
	}
	untagged, _, status := scopedCoverProfile(context.Background(), module, []string{"./p"}, nil)
	defer func() { _ = os.RemoveAll(filepath.Dir(untagged)) }()
	if _, hit := fCoverage(t, untagged); status != coverStatusOK || hit {
		t.Errorf("without the tag the integration test must not run: status %d, a statement of F is hit %v", status, hit)
	}
}

func TestScopedCoverProfile_AFailingTestIsTestsFailedAndAMissingTempDirIsPlumbing(t *testing.T) {
	module := coverChangedModule(t, "\tt.Fatal(\"red\")\n")
	profile, out, status := scopedCoverProfile(context.Background(), module, []string{"./p"}, []string{"integration"})
	defer func() { _ = os.RemoveAll(filepath.Dir(profile)) }()
	if status != coverStatusTestsFailed || !strings.Contains(out, "red") {
		t.Errorf("a failing test: status %d out %q, want tests-failed with its output", status, out)
	}
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	if profile, _, status := scopedCoverProfile(context.Background(), module, []string{"./p"}, nil); status != coverStatusPlumbingError || profile != "" {
		t.Errorf("no temp dir: status %d profile %q, want a plumbing error and no profile", status, profile)
	}
}

func fakeGoOnPath(t *testing.T, script string) {
	t.Helper()
	realGo, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go not on PATH")
	}
	dir := t.TempDir()
	body := "#!/bin/sh\n" + strings.ReplaceAll(script, "REALGO", realGo)
	if err := os.WriteFile(filepath.Join(dir, "go"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestScopedCoverFunc_NamesEveryFaultOfTheCoveragePass(t *testing.T) {
	failing := coverChangedModule(t, "")
	writeFile(t, filepath.Join(failing, "p", "red_test.go"), "package p\n\nimport \"testing\"\n\nfunc TestRed(t *testing.T) { t.Fatal(\"red\") }\n")
	path, out, status := scopedCoverFunc(context.Background(), failing, []string{"./p"})
	defer func() { _ = os.RemoveAll(path) }()
	if status != coverStatusTestsFailed || !strings.HasSuffix(path, string(os.PathSeparator)) || !strings.Contains(out, "red") {
		t.Errorf("a failing test: status %d path %q out %q; want tests-failed, the temp dir and the output", status, path, out)
	}
	green := coverChangedModule(t, "")
	funcPath, _, status := scopedCoverFunc(context.Background(), green, []string{"./p"})
	defer func() { _ = os.RemoveAll(filepath.Dir(funcPath)) }()
	if data, err := os.ReadFile(funcPath); status != coverStatusOK || err != nil || !strings.Contains(string(data), "total:") {
		t.Errorf("a green module: status %d func file %q %v; want the cover -func output", status, data, err)
	}
	for name, tc := range map[string]struct {
		script string
		want   string
	}{
		"the cover tool fails":            {"case \"$1\" in tool) echo broke >&2; exit 3;; esac\nexec REALGO \"$@\"\n", "go tool cover"},
		"the func file cannot be written": {"p=\"\"; prev=\"\"; for a in \"$@\"; do [ \"$prev\" = -coverprofile ] && p=\"$a\"; prev=\"$a\"; done\nREALGO \"$@\" || exit $?\n[ -n \"$p\" ] && mkdir -p \"$(dirname \"$p\")/cover.func.txt\"\nexit 0\n", "cover.func.txt"},
	} {
		t.Run(name, func(t *testing.T) {
			fakeGoOnPath(t, tc.script)
			module := coverChangedModule(t, "\tif F(1) != 1 {\n\t\tt.Fatal(\"F(1)\")\n\t}\n")
			path, out, status := scopedCoverFunc(context.Background(), module, []string{"./p"})
			defer func() { _ = os.RemoveAll(path) }()
			if status != coverStatusPlumbingError || !strings.Contains(out, tc.want) || !strings.HasSuffix(path, string(os.PathSeparator)) {
				t.Errorf("status %d out %q path %q; want a plumbing error naming %q and the temp dir", status, out, path, tc.want)
			}
		})
	}
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	if path, _, status := scopedCoverFunc(context.Background(), failing, []string{"./p"}); status != coverStatusPlumbingError || path != "" {
		t.Errorf("no temp dir: status %d path %q, want a plumbing error and no path", status, path)
	}
}

func fCoverage(t *testing.T, profile string) (branchHit, anyHit bool) {
	t.Helper()
	data, err := os.ReadFile(profile)
	if err != nil {
		t.Fatalf("read profile: %v", err)
	}
	parsed, err := covchanged.ParseProfile(data, "example.com/m")
	if err != nil {
		t.Fatalf("parse profile: %v", err)
	}
	blocks := parsed["p/a.go"]
	if len(blocks) == 0 {
		t.Fatalf("the profile has no block of p/a.go:\n%s", data)
	}
	for _, b := range blocks {
		if b.Hit && b.StartLine <= 8 && b.EndLine >= 3 {
			anyHit = true
		}
		if b.Hit && b.StartLine <= 5 && b.EndLine >= 5 {
			branchHit = true
		}
	}
	return branchHit, anyHit
}
