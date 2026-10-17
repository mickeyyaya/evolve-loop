package core

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/coverstrict"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

const fullyCoveredChangedFProfile = "mode: set\nexample.com/m/p/a.go:3.19,4.11 1 1\nexample.com/m/p/a.go:4.11,6.3 1 1\nexample.com/m/p/a.go:7.2,7.10 1 1\n"

type strictRunnerCall struct {
	moduleDir string
	pkgs      []string
	profiled  bool
}

func stubCoverStrictRunner(t *testing.T, results []coverstrict.Result) *[]strictRunnerCall {
	t.Helper()
	var calls []strictRunnerCall
	orig := coverStrictRunner
	coverStrictRunner = func(moduleDir, profile string, pkgs []string) ([]coverstrict.Result, error) {
		calls = append(calls, strictRunnerCall{moduleDir, pkgs, profile != ""})
		return results, nil
	}
	t.Cleanup(func() { coverStrictRunner = orig })
	return &calls
}

func TestCoverChangedFailures_StrictRunner(t *testing.T) {
	repo, base := coverChangedRepo(t)
	below := coverstrict.Result{Pkg: "internal/practicewatch", Min: 100, Percent: 96.6, Statements: 29, Covered: 28, Problem: "96.6% < 100.0%"}
	met := coverstrict.Result{Pkg: "p", Min: 100, Percent: 100, Statements: 3, Covered: 3}
	for name, tc := range map[string]struct {
		policy      string
		results     []coverstrict.Result
		wantFailure bool
		wantWarning bool
	}{
		"enforce refuses a strict package below its floor": {`{"cover_changed":{"stage":"enforce"}}`, []coverstrict.Result{below}, true, false},
		"shadow only warns":     {`{}`, []coverstrict.Result{below}, false, true},
		"a met floor is silent": {`{"cover_changed":{"stage":"enforce"}}`, []coverstrict.Result{met}, false, false},
	} {
		t.Run(name, func(t *testing.T) {
			stubCoverChangedProfiler(t, fullyCoveredChangedFProfile, "", coverStatusOK)
			calls := stubCoverStrictRunner(t, tc.results)
			in := floorInput(t, floorRoot(t, tc.policy), repo, base)

			var failures []string
			stderr := captureStderr(t, func() { failures = coverChangedFailures(context.Background(), in, []string{"go/p/a.go"}) })

			want := []strictRunnerCall{{filepath.Join(repo.Dir, "go"), []string{"./p"}, true}}
			if !reflect.DeepEqual(*calls, want) {
				t.Errorf("strict runner calls = %+v, want %+v", *calls, want)
			}
			named := "cover strict: internal/practicewatch: 96.6% < 100.0% FAIL"
			joined := strings.Join(failures, "\n")
			if got := len(failures) == 1 && strings.Contains(joined, named); got != tc.wantFailure {
				t.Errorf("failures = %v, want a failure naming %q: %v", failures, named, tc.wantFailure)
			}
			if !tc.wantFailure && failures != nil {
				t.Errorf("failures = %v, want none", failures)
			}
			if got := strings.Contains(stderr, "shadow") && strings.Contains(stderr, named); got != tc.wantWarning {
				t.Errorf("stderr = %q, want a shadow warning naming %q: %v", stderr, named, tc.wantWarning)
			}
		})
	}
}

func strictListedRepo(t *testing.T) (*gittest.Repo, string) {
	t.Helper()
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "go.mod"), "module example.com/m\n\ngo 1.23\n")
	writeFile(t, filepath.Join(repo.Dir, "go", ".cover-strict"), "./p 100\n./q 100\n")
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "a.go"), "package p\n\nfunc F(x int) int {\n\treturn x\n}\n")
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "b.go"), "package p\n\nfunc G() int {\n\treturn 2\n}\n")
	writeFile(t, filepath.Join(repo.Dir, "go", "q", "q.go"), "package q\n\nfunc Q() int {\n\treturn 3\n}\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "a.go"), "package p\n\nfunc F(x int) int {\n\tif x > 0 {\n\t\treturn 1\n\t}\n\treturn x\n}\n")
	repo.Git("commit", "-q", "-am", "build")
	return repo, base
}

func TestCoverChangedFailures_StrictRunnerDefaultMeasuresOnlyTheChangedStrictPackages(t *testing.T) {
	repo, base := strictListedRepo(t)
	profile := fullyCoveredChangedFProfile + "example.com/m/p/b.go:3.14,5.2 1 0\n"
	stubCoverChangedProfiler(t, profile, "", coverStatusOK)

	var enforce []string
	enforceErr := captureStderr(t, func() {
		enforce = coverChangedFailures(context.Background(), floorInput(t, floorRoot(t, `{"cover_changed":{"stage":"enforce"}}`), repo, base), []string{"go/p/a.go"})
	})
	var shadow []string
	shadowErr := captureStderr(t, func() {
		shadow = coverChangedFailures(context.Background(), floorInput(t, floorRoot(t, `{}`), repo, base), []string{"go/p/a.go"})
	})

	named := "cover strict: p: 75.0% < 100.0% FAIL"
	joined := strings.Join(enforce, "\n")
	if len(enforce) != 1 || !strings.Contains(joined, named) {
		t.Errorf("enforce = %v (stderr %q), want one failure naming %q", enforce, enforceErr, named)
	}
	if strings.Contains(joined+enforceErr, "cover strict: q") || strings.Contains(joined, "REFUSED") {
		t.Errorf("enforce judged more than the changed strict package p: %v %q", enforce, enforceErr)
	}
	if shadow != nil || !strings.Contains(shadowErr, "shadow") || !strings.Contains(shadowErr, named) {
		t.Errorf("shadow = %v stderr = %q, want no failure and a shadow warning naming %q", shadow, shadowErr, named)
	}
}
