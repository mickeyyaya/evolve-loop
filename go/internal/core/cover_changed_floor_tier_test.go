package core

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/covchanged"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

const bridgeBaseSource = "package bridge\n\nfunc Old(x int) int {\n\treturn x\n}\n\nfunc Part(x int) int {\n\treturn x\n}\n"

const bridgeTreeSource = "package bridge\n\nfunc Old(x int) int {\n\tif x > 0 {\n\t\treturn 1\n\t}\n\treturn x\n}\n\nfunc Part(x int) int {\n\tif x > 0 {\n\t\treturn 1\n\t}\n\treturn x\n}\n\nfunc Fresh(x int) int {\n\treturn x + 1\n}\n"

const bridgeNewFileSource = "package bridge\n\nfunc Brand(x int) int {\n\treturn x + 2\n}\n"

const (
	oldAtZero   = "example.com/m/internal/bridge/a.go:3.21,4.11 1 0\nexample.com/m/internal/bridge/a.go:4.11,6.3 1 0\nexample.com/m/internal/bridge/a.go:7.2,7.10 1 0\n"
	oldCovered  = "example.com/m/internal/bridge/a.go:3.21,4.11 1 1\nexample.com/m/internal/bridge/a.go:4.11,6.3 1 1\nexample.com/m/internal/bridge/a.go:7.2,7.10 1 1\n"
	partHalf    = "example.com/m/internal/bridge/a.go:10.22,11.11 1 1\nexample.com/m/internal/bridge/a.go:11.11,13.3 1 0\nexample.com/m/internal/bridge/a.go:14.2,14.10 1 1\n"
	partCovered = "example.com/m/internal/bridge/a.go:10.22,11.11 1 1\nexample.com/m/internal/bridge/a.go:11.11,13.3 1 1\nexample.com/m/internal/bridge/a.go:14.2,14.10 1 1\n"
	freshZero   = "example.com/m/internal/bridge/a.go:17.23,19.2 1 0\n"
	freshFull   = "example.com/m/internal/bridge/a.go:17.23,19.2 1 1\n"
	brandZero   = "example.com/m/internal/bridge/b.go:3.23,5.2 1 0\n"
	brandFull   = "example.com/m/internal/bridge/b.go:3.23,5.2 1 1\n"
)

func bridgeRepo(t *testing.T) (*gittest.Repo, string) {
	t.Helper()
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "go.mod"), "module example.com/m\n\ngo 1.23\n")
	writeFile(t, filepath.Join(repo.Dir, "go", "internal", "bridge", "a.go"), bridgeBaseSource)
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	writeFile(t, filepath.Join(repo.Dir, "go", "internal", "bridge", "a.go"), bridgeTreeSource)
	writeFile(t, filepath.Join(repo.Dir, "go", "internal", "bridge", "b.go"), bridgeNewFileSource)
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "build")
	return repo, base
}

var bridgePaths = []string{"go/internal/bridge/a.go", "go/internal/bridge/b.go"}

func bridgeFloor(t *testing.T, profile, stage string) (failures []string, in ReviewInput, stderr string, calls *[]profilerCall) {
	t.Helper()
	repo, base := bridgeRepo(t)
	calls = stubCoverChangedProfiler(t, "mode: set\n"+profile, "", coverStatusOK)
	var buf bytes.Buffer
	in = floorInput(t, floorRoot(t, `{"cover_changed":{"stage":"`+stage+`"}}`), repo, base)
	in.residuals = &floorResidualLog{}
	in.Stderr = &buf
	failures = coverChangedFailures(context.Background(), in, bridgePaths)
	return failures, in, buf.String(), calls
}

func TestPackageCoverTags_NeverChangeTheDefaults(t *testing.T) {
	t.Parallel()

	got := packageCoverTags("./internal/bridge")
	got[0] = "mutated"

	if !slices.Equal(covchanged.DefaultTags, []string{"integration"}) {
		t.Errorf("DefaultTags = %v, want [integration]", covchanged.DefaultTags)
	}
}

func TestCoverChangedPackages_JudgesEachPackageUnderItsOwnTags(t *testing.T) {
	t.Parallel()
	module := t.TempDir()
	writeFile(t, filepath.Join(module, "internal", "bridge", "a.go"), "package bridge\n")
	writeFile(t, filepath.Join(module, "internal", "x", "a.go"), "package x\n")
	writeFile(t, filepath.Join(module, "acs", "c1", "p_test.go"), "package c1\n")
	writeFile(t, filepath.Join(module, "internal", "plain", "a.go"), "//go:build !notmux\n\npackage plain\n")
	writeFile(t, filepath.Join(module, "internal", "floor", "a.go"), "//go:build notmux\n\npackage floor\n")
	paths := []string{"go/internal/bridge/a.go", "go/internal/x/a.go", "go/internal/plain/a.go", "go/internal/floor/a.go", "go/acs/c1/p_test.go"}

	got := coverChangedPackages(module, paths)

	if want := []string{"./internal/bridge", "./internal/plain", "./internal/x"}; !reflect.DeepEqual(got, want) {
		t.Errorf("coverChangedPackages = %v, want %v", got, want)
	}
}

func TestCoverChangedFailures_ABridgeChangeIsMeasuredUnderTheFloorTags(t *testing.T) {
	failures, _, _, calls := bridgeFloor(t, oldCovered+partHalf+freshFull+brandFull, "enforce")

	if len(*calls) != 1 || !slices.Equal((*calls)[0].tags, covchanged.DefaultTags) || !slices.Equal((*calls)[0].pkgs, []string{"./internal/bridge"}) {
		t.Errorf("profiler calls = %+v, want one call of ./internal/bridge with the base tags (the tier adds notmux per package)", *calls)
	}
	if len(failures) != 1 || !strings.Contains(failures[0], "internal/bridge.Part 66.7% (2/3)") || !strings.Contains(failures[0], "-tags integration,notmux") || !strings.Contains(failures[0], "1 of 4") {
		t.Errorf("failures = %v, want the partly covered Part refused with the floor tags in the headline", failures)
	}
}

func TestCoverChangedFailures_ANewFunctionAtZeroIsRefusedAndAnOldOneThatOnlyTheTierCoversIsNamedNotMeasured(t *testing.T) {
	failures, in, stderr, _ := bridgeFloor(t, oldAtZero+partCovered+freshZero+brandZero, "enforce")

	joined := strings.Join(failures, "\n")
	if len(failures) != 1 || !strings.Contains(joined, "internal/bridge.Fresh") || !strings.Contains(joined, "internal/bridge.Brand") || strings.Contains(joined, "internal/bridge.Old") || !strings.Contains(joined, "2 of 4") {
		t.Errorf("failures = %v, want Fresh and Brand refused and Old not refused", failures)
	}
	if !strings.Contains(stderr, "WARN") || !strings.Contains(stderr, "internal/bridge.Old") || !strings.Contains(stderr, "not measured at the floor") || !strings.Contains(stderr, "no test in the floor build covers it") || !strings.Contains(stderr, "the notmux tier did not run") {
		t.Errorf("stderr = %q, want a WARN that names Old, the floor and the tag", stderr)
	}
	var notMeasured []FloorResidual
	for _, r := range in.residuals.list() {
		if r.Func == "internal/bridge.Old" {
			notMeasured = append(notMeasured, r)
		}
	}
	if len(notMeasured) != 1 || notMeasured[0].Class != FloorResidualCoverChanged || notMeasured[0].Source != "go/internal/bridge/a.go" || notMeasured[0].Line != 3 || !strings.Contains(notMeasured[0].Failure, "not measured at the floor") {
		t.Errorf("residuals = %+v, want one typed residual of Old that says not measured", in.residuals.list())
	}
}

func TestCoverChangedFailures_OnlyAnOldFunctionAtZeroLeavesTheHandoffAlone(t *testing.T) {
	failures, _, stderr, _ := bridgeFloor(t, oldAtZero+partCovered+freshFull+brandFull, "enforce")

	if failures != nil || !strings.Contains(stderr, "internal/bridge.Old") {
		t.Errorf("failures = %v, stderr = %q, want no failure and the WARN", failures, stderr)
	}
}

func TestCoverChangedFailures_AtShadowTheRefusalOnlyWarnsAndTheNotMeasuredLineStays(t *testing.T) {
	var failures []string
	var stderr string
	shadow := captureStderr(t, func() {
		failures, _, stderr, _ = bridgeFloor(t, oldAtZero+partHalf+freshFull+brandFull, "shadow")
	})

	if failures != nil || !strings.Contains(shadow, "cover-changed (shadow)") || !strings.Contains(shadow, "internal/bridge.Part") || !strings.Contains(stderr, "not measured at the floor") {
		t.Errorf("failures = %v, shadow = %q, stderr = %q, want warnings only", failures, shadow, stderr)
	}
}

func TestCoverChangedFailures_AnOldFunctionAtZeroOutsideTheTieredPackageIsStillRefused(t *testing.T) {
	repo, base := coverChangedRepo(t)
	stubCoverChangedProfiler(t, "mode: set\nexample.com/m/p/a.go:3.19,4.11 1 0\nexample.com/m/p/a.go:4.11,6.3 1 0\nexample.com/m/p/a.go:7.2,7.10 1 0\n", "", coverStatusOK)
	in := floorInput(t, floorRoot(t, `{"cover_changed":{"stage":"enforce"}}`), repo, base)

	failures := coverChangedFailures(context.Background(), in, []string{"go/p/a.go"})

	if len(failures) != 1 || !strings.Contains(failures[0], "p.F 0.0% (0/3)") {
		t.Errorf("failures = %v, want p.F refused at zero", failures)
	}
}

func TestCoverChangedFailures_ThePlumbingFallbackNeverRunsTheBridgePackage(t *testing.T) {
	repo, base := bridgeRepo(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "a.go"), "package p\n")
	stubCoverChangedProfiler(t, "", "bad .cover-split", coverStatusPlumbingError)
	ran := stubSelfCheckRunner(t, "ok", true)
	var buf bytes.Buffer
	in := floorInput(t, floorRoot(t, `{"cover_changed":{"stage":"enforce"}}`), repo, base)
	in.Stderr = &buf

	failures := coverChangedFailures(context.Background(), in, append([]string{"go/p/a.go"}, bridgePaths...))

	if failures != nil || !slices.Equal(*ran, []string{"./p"}) {
		t.Errorf("failures = %v, unit run = %v, want only ./p and no failure", failures, *ran)
	}
	if !strings.Contains(buf.String(), "env-exclusive package(s) ./internal/bridge") || !strings.Contains(buf.String(), "the cover run measures them") {
		t.Errorf("stderr = %q, want the lane note that says the cover run measures the package", buf.String())
	}
}

func TestNoteEnvExclusive_TheLaneNoteSaysTheCoverRunMeasuresThePackage(t *testing.T) {
	t.Parallel()
	var warn strings.Builder

	noteEnvExclusive([]string{"./internal/bridge"}, &warn)

	for _, want := range []string{"./internal/bridge", "not run by this lane", "the cover run measures them under -tags notmux", "Backstop:"} {
		if !strings.Contains(warn.String(), want) {
			t.Errorf("lane note = %q, want %q", warn.String(), want)
		}
	}
	warn.Reset()
	noteEnvExclusive(nil, &warn)
	if warn.Len() != 0 {
		t.Errorf("lane note for no package = %q, want nothing", warn.String())
	}
}

func TestNoteCoverEnvExclusive_TheCoverNoteNamesTheExclusionTag(t *testing.T) {
	t.Parallel()
	var warn strings.Builder

	noteCoverEnvExclusive([]string{"./internal/bridge", "./internal/x"}, &warn)

	for _, want := range []string{"./internal/bridge", "run in the cover run under -tags notmux", "leaves their tmux tier out of the build", "Backstop:"} {
		if !strings.Contains(warn.String(), want) {
			t.Errorf("cover note = %q, want %q", warn.String(), want)
		}
	}
	if strings.Contains(warn.String(), "./internal/x") {
		t.Errorf("cover note = %q, must not name a package that is not env-exclusive", warn.String())
	}
	warn.Reset()
	noteCoverEnvExclusive([]string{"./internal/x"}, &warn)
	if warn.Len() != 0 {
		t.Errorf("cover note for a plain package = %q, want nothing", warn.String())
	}
}

func TestSplitNotMeasured_OnlyAProfiledZeroOfAnOldTieredFunctionIsNotMeasured(t *testing.T) {
	t.Parallel()
	tiered := func(name string, statements, covered int, profiled bool) covchanged.Func {
		return covchanged.Func{File: "internal/bridge/a.go", Name: name, Key: "internal/bridge." + name, Statements: statements, Covered: covered, Profiled: profiled}
	}
	old := func(f covchanged.Func) bool { return f.Name != "Fresh" }
	refused := []covchanged.Func{
		tiered("Old", 3, 0, true),
		tiered("Fresh", 3, 0, true),
		tiered("Part", 3, 2, true),
		tiered("NoProfile", 0, 0, false),
		{File: "internal/x/a.go", Name: "Old", Key: "internal/x.Old", Statements: 3, Profiled: true},
	}

	kept, unmeasured := splitNotMeasured(refused, old)

	keys := func(fs []covchanged.Func) []string {
		var out []string
		for _, f := range fs {
			out = append(out, f.Key)
		}
		return out
	}
	if want := []string{"internal/bridge.Fresh", "internal/bridge.Part", "internal/bridge.NoProfile", "internal/x.Old"}; !slices.Equal(keys(kept), want) {
		t.Errorf("kept = %v, want %v", keys(kept), want)
	}
	if want := []string{"internal/bridge.Old"}; !slices.Equal(keys(unmeasured), want) {
		t.Errorf("unmeasured = %v, want %v", keys(unmeasured), want)
	}
}

func TestBaseNames_ReadEachFileOnceAndTreatAnUnreadableOrUnparsableBaseAsNew(t *testing.T) {
	t.Parallel()
	reads := map[string]int{}
	read := func(file string) ([]byte, error) {
		reads[file]++
		switch file {
		case "missing.go":
			return nil, errors.New("not at the base")
		case "broken.go":
			return []byte("package p\nfunc ("), nil
		}
		return []byte("package p\n\nfunc F() {}\n"), nil
	}
	var warn strings.Builder
	names := baseNames{read: read, warn: &warn}

	got := []bool{
		names.has(covchanged.Func{File: "ok.go", Name: "F"}),
		names.has(covchanged.Func{File: "ok.go", Name: "F"}),
		names.has(covchanged.Func{File: "ok.go", Name: "G"}),
		names.has(covchanged.Func{File: "missing.go", Name: "F"}),
		names.has(covchanged.Func{File: "broken.go", Name: "F"}),
	}

	if want := []bool{true, true, false, false, false}; !slices.Equal(got, want) {
		t.Errorf("has = %v, want %v", got, want)
	}
	if reads["ok.go"] != 1 {
		t.Errorf("reads of ok.go = %d, want 1", reads["ok.go"])
	}
	if strings.Count(warn.String(), "read missing.go") != 1 || strings.Count(warn.String(), "read broken.go") != 1 || !strings.Contains(warn.String(), "not at the base") || strings.Contains(warn.String(), "ok.go") {
		t.Errorf("warn = %q, want one line each for missing.go and broken.go and none for ok.go", warn.String())
	}
	names.has(covchanged.Func{File: "missing.go", Name: "G"})
	if strings.Count(warn.String(), "read missing.go") != 1 {
		t.Errorf("warn = %q, want one line per file", warn.String())
	}
}

func TestGitBaseReader_ReadsTheFileAtTheBaseOfTheWorktree(t *testing.T) {
	t.Parallel()
	repo, base := bridgeRepo(t)
	in := floorInput(t, t.TempDir(), repo, base)

	read := gitBaseReader(context.Background(), in, "go")
	data, err := read("internal/bridge/a.go")
	_, missing := read("internal/bridge/b.go")

	if err != nil || string(data) != strings.TrimSpace(bridgeBaseSource) {
		t.Errorf("read = %q, %v, want the file as of the base", data, err)
	}
	if missing == nil {
		t.Error("read of a file that is not at the base = nil error, want one")
	}
}

func TestCoverChangedFailures_ARefusedFunctionWithAnUnreadableBaseIsRefusedAndTheReadErrorIsNamedOnce(t *testing.T) {
	failures, _, stderr, _ := bridgeFloor(t, oldAtZero+partCovered+freshFull+brandZero, "enforce")

	if len(failures) != 1 || !strings.Contains(failures[0], "internal/bridge.Brand") {
		t.Errorf("failures = %v, want Brand refused", failures)
	}
	if strings.Count(stderr, "could not read internal/bridge/b.go at the base") != 1 {
		t.Errorf("stderr = %q, want the read error of b.go named once", stderr)
	}
}

func TestCoverChangedFailures_AFailingTieredRunNamesItsTagsInTheHeadlineAndTheReproFlags(t *testing.T) {
	f := newBaseRedFixture(t, "TestPart")
	failing := goTestRed("TestPart", "TestFresh")
	orig := coverChangedProfiler
	coverChangedProfiler = func(context.Context, ReviewInput, string, []string, []string) (string, string, int, []string) {
		return "", failing, coverStatusTestsFailed, packageCoverTags("./internal/bridge")
	}
	t.Cleanup(func() { coverChangedProfiler = orig })
	writeFile(t, filepath.Join(f.in.Worktree, "go", "internal", "bridge", "a.go"), "package bridge\n")
	writeFile(t, filepath.Join(f.in.Worktree, ".evolve", "policy.json"), `{"cover_changed":{"stage":"enforce"}}`)
	var flags []string
	f.in.baseRun = func(ctx context.Context, dir string, r baseattr.Reds) ([]baseattr.Key, error) {
		flags = r.Flags
		return f.runs.run(ctx, dir, r)
	}

	failures := coverChangedFailures(context.Background(), f.in, []string{"go/internal/bridge/a.go"})

	if len(failures) != 1 || !strings.Contains(failures[0], "fail under -tags integration,notmux, so no changed function is proven") {
		t.Errorf("failures = %v, want the headline to name -tags integration,notmux", failures)
	}
	if want := []string{"-tags", "integration,notmux"}; !slices.Equal(flags, want) {
		t.Errorf("repro flags = %v, want %v", flags, want)
	}
}

func TestCoverChangedFailures_EachPackageIsJudgedUnderItsOwnTagSet(t *testing.T) {
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "go.mod"), "module example.com/m\n\ngo 1.23\n")
	writeFile(t, filepath.Join(repo.Dir, "go", "internal", "x", "keep.go"), "package x\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	writeFile(t, filepath.Join(repo.Dir, "go", "internal", "x", "serve_floor.go"), "//go:build notmux\n\npackage x\n\nfunc Floor() int {\n\treturn 1\n}\n")
	writeFile(t, filepath.Join(repo.Dir, "go", "internal", "x", "serve_tmux.go"), "//go:build !notmux\n\npackage x\n\nfunc Tmux() int {\n\treturn 2\n}\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "build")
	stubCoverChangedProfiler(t, "mode: set\nexample.com/m/internal/x/serve_tmux.go:5.17,7.2 1 0\n", "", coverStatusOK)
	in := floorInput(t, floorRoot(t, `{"cover_changed":{"stage":"enforce"}}`), repo, base)
	in.Stderr = &bytes.Buffer{}

	failures := coverChangedFailures(context.Background(), in, []string{"go/internal/x/serve_floor.go", "go/internal/x/serve_tmux.go"})

	joined := strings.Join(failures, "\n")
	if len(failures) != 1 || !strings.Contains(joined, "serve_tmux.go:5 internal/x.Tmux 0.0% (0/1)") || strings.Contains(joined, "Floor") || strings.Contains(joined, "no profile") {
		t.Errorf("failures = %v, want only the !notmux function refused at 0%% and the notmux one exempt by its constraint", failures)
	}
}

func TestCoverTagGroups_GroupThePackagesByTheirTagSetInFirstSeenOrder(t *testing.T) {
	t.Parallel()

	groups := coverTagGroups([]string{"./cmd/evolve", "./internal/bridge", "./internal/x"})

	want := []tagGroup{{tags: []string{"integration"}, pkgs: []string{"./cmd/evolve", "./internal/x"}}, {tags: []string{"integration", "notmux"}, pkgs: []string{"./internal/bridge"}}}
	if !reflect.DeepEqual(groups, want) {
		t.Errorf("groups = %+v, want %+v", groups, want)
	}
	if got := unionTags(groups); !slices.Equal(got, []string{"integration", "notmux"}) {
		t.Errorf("unionTags = %v, want [integration notmux]", got)
	}
}

const mixedXBaseSource = "package x\n\nfunc X(x int) int {\n\treturn x\n}\n"

const mixedXTreeSource = "package x\n\nfunc X(x int) int {\n\tif x > 0 {\n\t\treturn 1\n\t}\n\treturn x\n}\n"

const mixedXCovered = "example.com/m/internal/x/x.go:3.19,4.11 1 1\nexample.com/m/internal/x/x.go:4.11,6.3 1 1\nexample.com/m/internal/x/x.go:7.2,7.10 1 1\n"

func TestCoverChangedFailures_AMixedChangeJudgesEachGroupOnlyOnItsOwnPackages(t *testing.T) {
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "go.mod"), "module example.com/m\n\ngo 1.23\n")
	writeFile(t, filepath.Join(repo.Dir, "go", "internal", "bridge", "a.go"), bridgeBaseSource)
	writeFile(t, filepath.Join(repo.Dir, "go", "internal", "x", "x.go"), mixedXBaseSource)
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	writeFile(t, filepath.Join(repo.Dir, "go", "internal", "bridge", "a.go"), bridgeTreeSource)
	writeFile(t, filepath.Join(repo.Dir, "go", "internal", "bridge", "b.go"), bridgeNewFileSource)
	writeFile(t, filepath.Join(repo.Dir, "go", "internal", "x", "x.go"), mixedXTreeSource)
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "build")
	stubCoverChangedProfiler(t, "mode: set\n"+oldCovered+partHalf+freshFull+brandFull+mixedXCovered, "", coverStatusOK)
	in := floorInput(t, floorRoot(t, `{"cover_changed":{"stage":"enforce"}}`), repo, base)
	in.residuals = &floorResidualLog{}
	in.Stderr = &bytes.Buffer{}

	failures := coverChangedFailures(context.Background(), in, append([]string{"go/internal/x/x.go"}, bridgePaths...))

	joined := strings.Join(failures, "\n")
	if len(failures) != 1 || strings.Count(joined, "REFUSED") != 1 || !strings.Contains(joined, "REFUSED internal/bridge/a.go:10 internal/bridge.Part") || !strings.Contains(joined, "1 of 5 changed function(s)") {
		t.Errorf("failures = %v, want exactly one refused bridge function out of 5 changed", failures)
	}
}
