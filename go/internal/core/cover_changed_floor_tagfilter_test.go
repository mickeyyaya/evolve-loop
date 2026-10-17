package core

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

const acsOnlyPredicate = "//go:build acs\n\npackage cycle1916\n\nimport \"testing\"\n\nfunc TestC1916(t *testing.T) {}\n"

func TestCoverChangedPackages_ExcludesTagOnly(t *testing.T) {
	t.Parallel()

	module := t.TempDir()
	writeFile(t, filepath.Join(module, "go.mod"), "module example.com/m\n\ngo 1.23\n")
	writeFile(t, filepath.Join(module, "p", "a.go"), "package p\n\nfunc F() int { return 1 }\n")
	writeFile(t, filepath.Join(module, "acs", "cycle1916", "predicates_test.go"), acsOnlyPredicate)
	writeFile(t, filepath.Join(module, "it", "it_test.go"), "//go:build integration\n\npackage it\n\nimport \"testing\"\n\nfunc TestIt(t *testing.T) {}\n")
	writeFile(t, filepath.Join(module, "tools", "gen.go"), "//go:build tools\n\npackage tools\n")
	if err := os.MkdirAll(filepath.Join(module, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	paths := []string{"go/acs/cycle1916/predicates_test.go", "go/p/a.go", "go/it/it_test.go", "go/tools/gen.go", "go/empty/x.go"}

	got := coverChangedPackages(module, paths)

	if want := []string{"./it", "./p"}; !reflect.DeepEqual(got, want) {
		t.Errorf("coverChangedPackages = %v, want %v: a package that builds no file under covchanged.DefaultTags must never reach the profiler", got, want)
	}
}

func coverChangedRepoWithACSPackage(t *testing.T) (*gittest.Repo, string) {
	t.Helper()
	repo, base := coverChangedRepo(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "acs", "cycle1916", "predicates_test.go"), acsOnlyPredicate)
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "acs predicates")
	return repo, base
}

func TestCoverChangedFailures_ExcludesACSTagOnlyPackageFromProfiler(t *testing.T) {
	repo, base := coverChangedRepoWithACSPackage(t)
	for name, tc := range map[string]struct {
		paths     []string
		wantCalls []profilerCall
	}{
		"the production package alone is profiled": {
			[]string{"go/acs/cycle1916/predicates_test.go", "go/p/a.go"},
			[]profilerCall{{filepath.Join(repo.Dir, "go"), []string{"./p"}, []string{"integration"}}},
		},
		"an acs-only change never runs the profiler": {
			[]string{"go/acs/cycle1916/predicates_test.go"},
			nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			calls := stubCoverChangedProfiler(t, coverChangedProfile, "", coverStatusOK)
			in := floorInput(t, floorRoot(t, `{"cover_changed":{"stage":"enforce"}}`), repo, base)

			var failures []string
			stderr := captureStderr(t, func() { failures = coverChangedFailures(context.Background(), in, tc.paths) })

			if !reflect.DeepEqual(*calls, tc.wantCalls) {
				t.Errorf("profiler calls = %+v, want %+v", *calls, tc.wantCalls)
			}
			if strings.Contains(stderr, "setup failed") || strings.Contains(strings.Join(failures, "\n"), "setup failed") {
				t.Errorf("a [setup failed] surfaced: failures=%v stderr=%q", failures, stderr)
			}
		})
	}
}

func uncoveredAddedFunctionRepo(t *testing.T) (*gittest.Repo, string) {
	t.Helper()
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "go.mod"), "module example.com/m\n\ngo 1.23\n")
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "a.go"), "package p\n\nfunc F(x int) int {\n\treturn x + 1\n}\n")
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "a_test.go"), "package p\n\nimport \"testing\"\n\nfunc TestF(t *testing.T) {\n\tif F(1) != 2 {\n\t\tt.Fatal(\"F\")\n\t}\n}\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "a.go"), "package p\n\nfunc F(x int) int {\n\treturn x + 1\n}\n\nfunc MarkTruncated(s string) string {\n\treturn s + \"...\"\n}\n")
	writeFile(t, filepath.Join(repo.Dir, "go", "acs", "cycle1916", "predicates_test.go"), acsOnlyPredicate)
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "build adds an untested function and its acs predicates")
	return repo, base
}

func TestCoverChangedFailures_UncoveredFunction(t *testing.T) {
	repo, base := uncoveredAddedFunctionRepo(t)
	paths := []string{"go/acs/cycle1916/predicates_test.go", "go/p/a.go"}

	var enforce []string
	enforceErr := captureStderr(t, func() {
		enforce = coverChangedFailures(context.Background(), floorInput(t, floorRoot(t, `{"cover_changed":{"stage":"enforce"}}`), repo, base), paths)
	})
	var shadow []string
	shadowErr := captureStderr(t, func() {
		shadow = coverChangedFailures(context.Background(), floorInput(t, floorRoot(t, `{}`), repo, base), paths)
	})

	joined := strings.Join(enforce, "\n")
	if len(enforce) != 1 || !strings.Contains(joined, "REFUSED") || !strings.Contains(joined, "p.MarkTruncated") {
		t.Errorf("enforce = %v (stderr %q), want one refusal that names p.MarkTruncated", enforce, enforceErr)
	}
	if strings.Contains(joined, "p.F ") {
		t.Errorf("enforce refuses the covered, unchanged p.F: %v", enforce)
	}
	if shadow != nil || !strings.Contains(shadowErr, "cover-changed (shadow)") || !strings.Contains(shadowErr, "p.MarkTruncated") {
		t.Errorf("shadow = %v stderr = %q, want no failure and a shadow warning that names p.MarkTruncated", shadow, shadowErr)
	}
	for stage, text := range map[string]string{"enforce": joined + enforceErr, "shadow": shadowErr} {
		if strings.Contains(text, "setup failed") || strings.Contains(text, "fail under -tags integration") {
			t.Errorf("%s: the acs package broke the profile instead of the floor measuring the change:\n%s", stage, text)
		}
	}
}
