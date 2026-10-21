package core

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/addedtests"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/covchanged"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

func coverGapLines() []string {
	return covchanged.Verdict{Refused: []covchanged.Func{{File: "internal/x/a.go", Line: 3, Key: "internal/x.F", Statements: 3, Covered: 2, Profiled: true}}}.Problems()
}

func TestCoverChangedResult_EnforceRejectsEveryGapByName(t *testing.T) {
	t.Parallel()

	failures, warning := coverChangedResult(config.StageEnforce, "1 of 1 changed function(s) are below 100%", coverGapLines())

	if warning != "" || len(failures) != 1 {
		t.Fatalf("enforce: failures=%v warning=%q, want one failure and no warning", failures, warning)
	}
	for _, want := range []string{"1 of 1 changed function(s) are below 100%", "REFUSED internal/x/a.go:3 internal/x.F 66.7% (2/3)"} {
		if !strings.Contains(failures[0], want) {
			t.Errorf("the failure does not name %q:\n%s", want, failures[0])
		}
	}
}

func TestCoverChangedResult_ShadowOnlyWarnsAndOffOrNoLinesIsSilent(t *testing.T) {
	t.Parallel()

	if failures, warning := coverChangedResult(config.StageShadow, "head", coverGapLines()); failures != nil || !strings.Contains(warning, "shadow") || !strings.Contains(warning, "internal/x.F") {
		t.Errorf("shadow: failures=%v warning=%q, want a warning naming the function and no failure", failures, warning)
	}
	for stage, lines := range map[config.Stage][]string{config.StageOff: coverGapLines(), config.StageEnforce: nil, config.StageShadow: nil} {
		if failures, warning := coverChangedResult(stage, "head", lines); failures != nil || warning != "" {
			t.Errorf("stage %s, %d lines: failures=%v warning=%q, want silence", stage, len(lines), failures, warning)
		}
	}
}

func TestCoverChangedPackages_KeepsOnlyLivePackagesOutsideTestdata(t *testing.T) {
	t.Parallel()

	module := t.TempDir()
	writeFile(t, filepath.Join(module, "internal", "x", "a.go"), "package x\n")
	writeFile(t, filepath.Join(module, "internal", "x", "testdata", "y", "y.go"), "package y\n")
	writeFile(t, filepath.Join(module, "cmd", "z", "main.go"), "package main\n")
	paths := []string{"go/internal/x/a.go", "go/internal/x/a_test.go", "go/internal/x/testdata/y/y.go", "go/internal/gone/g.go", "go/cmd/z/main.go", "docs/x.md"}

	got := coverChangedPackages(module, paths)

	if want := []string{"./cmd/z", "./internal/x"}; !reflect.DeepEqual(got, want) {
		t.Errorf("coverChangedPackages = %v, want %v", got, want)
	}
}

func TestCoverTestArgs_AddTheTagsOfTheCoverageRunOnlyWhenGiven(t *testing.T) {
	t.Setenv(ipcenv.FleetWidthKey, "")

	got := coverTestArgs("/tmp/cover.out", []string{"./p"}, []string{"integration", "acs"})

	want := []string{"test", "-count=1", "-timeout", addedtests.PackageTimeout, "-tags", "integration,acs", "-coverprofile", "/tmp/cover.out", "./p"}
	if !slices.Equal(got, want) {
		t.Errorf("args = %v, want %v", got, want)
	}
}

func coverChangedRepo(t *testing.T) (*gittest.Repo, string) {
	t.Helper()
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "go.mod"), "module example.com/m\n\ngo 1.23\n")
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "a.go"), "package p\n\nfunc F(x int) int {\n\treturn x\n}\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "a.go"), "package p\n\nfunc F(x int) int {\n\tif x > 0 {\n\t\treturn 1\n\t}\n\treturn x\n}\n")
	repo.Git("commit", "-q", "-am", "build")
	return repo, base
}

type profilerCall struct {
	moduleDir string
	pkgs      []string
	tags      []string
}

func stubCoverChangedProfiler(t *testing.T, profile, output string, status int) *[]profilerCall {
	t.Helper()
	var calls []profilerCall
	orig := coverChangedProfiler
	coverChangedProfiler = func(_ context.Context, _ ReviewInput, moduleDir string, pkgs, tags []string) (string, string, int) {
		calls = append(calls, profilerCall{moduleDir, pkgs, tags})
		if status == coverStatusPlumbingError {
			return "", output, status
		}
		path := filepath.Join(t.TempDir(), "cover.out")
		if err := os.WriteFile(path, []byte(profile), 0o644); err != nil {
			t.Fatal(err)
		}
		return path, output, status
	}
	t.Cleanup(func() { coverChangedProfiler = orig })
	return &calls
}

const coverChangedProfile = "mode: set\nexample.com/m/p/a.go:3.19,4.11 1 1\nexample.com/m/p/a.go:4.11,6.3 1 0\nexample.com/m/p/a.go:7.2,7.10 1 1\n"

func TestCoverChangedFailures_EnforceRefusesTheChangedFunctionOfTheBuild(t *testing.T) {
	repo, base := coverChangedRepo(t)
	calls := stubCoverChangedProfiler(t, coverChangedProfile, "", coverStatusOK)
	in := floorInput(t, floorRoot(t, `{"cover_changed":{"stage":"enforce"}}`), repo, base)

	failures := coverChangedFailures(context.Background(), in, []string{"go/p/a.go"})

	if len(failures) != 1 || !strings.Contains(failures[0], "p/a.go:3 p.F 66.7% (2/3)") || !strings.Contains(failures[0], "1 of 1") {
		t.Fatalf("failures = %v, want p.F refused at 2/3", failures)
	}
	want := profilerCall{filepath.Join(repo.Dir, "go"), []string{"./p"}, covchanged.DefaultTags}
	if len(*calls) != 1 || !reflect.DeepEqual((*calls)[0], want) {
		t.Errorf("profiler calls = %+v, want %+v", *calls, want)
	}
}

func TestCoverChangedFailures_AFailingTestRefusesAtEnforceAndOnlyWarnsAtShadow(t *testing.T) {
	repo, base := coverChangedRepo(t)
	stubCoverChangedProfiler(t, coverChangedProfile, "--- FAIL: TestF\nFAIL\tp", coverStatusTestsFailed)
	stubSelfCheckRunner(t, "ok\tp", true)

	enforce := coverChangedFailures(context.Background(), floorInput(t, floorRoot(t, `{"cover_changed":{"stage":"enforce"}}`), repo, base), []string{"go/p/a.go"})
	shadow := coverChangedFailures(context.Background(), floorInput(t, floorRoot(t, `{"cover_changed":{"stage":"shadow"}}`), repo, base), []string{"go/p/a.go"})

	if len(enforce) != 1 || !strings.Contains(enforce[0], "--- FAIL: TestF") || !strings.Contains(enforce[0], "-tags integration") {
		t.Errorf("enforce = %v, want one refusal carrying the test failure", enforce)
	}
	if shadow != nil {
		t.Errorf("shadow = %v, want a warning only", shadow)
	}
}

func TestCoverChangedFailures_ShadowOffAndEveryPlumbingFaultLeaveTheHandoffAlone(t *testing.T) {
	repo, base := coverChangedRepo(t)
	for name, tc := range map[string]struct {
		policy  string
		paths   []string
		profile string
		status  int
		calls   int
	}{
		"shadow only warns":                 {`{"cover_changed":{"stage":"shadow"}}`, []string{"go/p/a.go"}, coverChangedProfile, coverStatusOK, 1},
		"off never measures":                {`{"cover_changed":{"stage":"off"}}`, []string{"go/p/a.go"}, coverChangedProfile, coverStatusOK, 0},
		"a docs-only change never measures": {`{"cover_changed":{"stage":"enforce"}}`, []string{"docs/a.md"}, coverChangedProfile, coverStatusOK, 0},
		"a plumbing fault warns":            {`{"cover_changed":{"stage":"enforce"}}`, []string{"go/p/a.go"}, "", coverStatusPlumbingError, 1},
		"a malformed profile warns":         {`{"cover_changed":{"stage":"enforce"}}`, []string{"go/p/a.go"}, "not a profile\n", coverStatusOK, 1},
	} {
		t.Run(name, func(t *testing.T) {
			calls := stubCoverChangedProfiler(t, tc.profile, "tool broke", tc.status)
			in := floorInput(t, floorRoot(t, tc.policy), repo, base)
			if failures := coverChangedFailures(context.Background(), in, tc.paths); failures != nil || len(*calls) != tc.calls {
				t.Errorf("failures=%v calls=%d, want none and %d calls", failures, len(*calls), tc.calls)
			}
		})
	}
	calls := stubCoverChangedProfiler(t, coverChangedProfile, "", coverStatusOK)
	if failures := coverChangedFailures(context.Background(), ReviewInput{ProjectRoot: floorRoot(t, `{"cover_changed":{"stage":"enforce"}}`)}, []string{"go/p/a.go"}); failures != nil || len(*calls) != 0 {
		t.Errorf("no worktree: failures=%v calls=%d, want nothing", failures, len(*calls))
	}
}

func TestFloorStage_AnUnknownWordTurnsTheCheckOffAndNamesIt(t *testing.T) {
	t.Parallel()

	for word, want := range map[string]config.Stage{"enforce": config.StageEnforce, "off": config.StageOff, "bogus": config.StageOff} {
		if got := coverChangedStage(floorRoot(t, `{"cover_changed":{"stage":"`+word+`"}}`), io.Discard); got != want {
			t.Errorf("stage %q = %s, want %s", word, got, want)
		}
	}
	if got := coverChangedStage("", io.Discard); got != config.StageEnforce {
		t.Errorf("no project root = %s, want the compiled enforce", got)
	}
	var commentSink, coverSink injectedStderr
	commentFloorStage(floorRoot(t, `{"comment_floor":{"stage":"bogus"}}`), &commentSink)
	coverChangedStage(floorRoot(t, `{"cover_changed":{"stage":"bogus"}}`), &coverSink)
	comment, cover := commentSink.String(), coverSink.String()
	if !strings.Contains(comment, `comment_floor.stage "bogus" is not off, shadow or enforce; the comment floor is off`) {
		t.Errorf("comment floor warning = %q, want the original text", comment)
	}
	if !strings.Contains(cover, `cover_changed.stage "bogus" is not off, shadow or enforce; the cover-changed check is off`) {
		t.Errorf("cover-changed warning = %q", cover)
	}
}

func TestCoverChangedFailures_LeavesAFailureTheUnitRunAlsoRefusesToTheChangedPackageFloor(t *testing.T) {
	repo, base := coverChangedRepo(t)
	stubCoverChangedProfiler(t, coverChangedProfile, "--- FAIL: TestF\nFAIL\tp", coverStatusTestsFailed)
	ran := stubSelfCheckRunner(t, "--- FAIL: TestF\nFAIL\tp", false)

	failures := coverChangedFailures(context.Background(), floorInput(t, floorRoot(t, `{"cover_changed":{"stage":"enforce"}}`), repo, base), []string{"go/p/a.go"})

	if failures != nil {
		t.Errorf("failures = %v, want none: the changed-package floor already refuses the failing unit run", failures)
	}
	if !slices.Equal(*ran, []string{"./p"}) {
		t.Errorf("unit run packages = %v, want [./p]", *ran)
	}
}
