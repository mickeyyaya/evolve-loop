package core

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/addedtests"
	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func newTestEvents(c *signalcenter.Center) []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range c.Recent() {
		if e.Code == CodeBuildFloorFlakyNewTest {
			out = append(out, e)
		}
	}
	return out
}

func floorOffInput(t *testing.T, in ReviewInput) ReviewInput {
	in.ProjectRoot = floorRoot(t, `{"cover_changed":{"stage":"off"}}`)
	return in
}

func TestDefaultBuildFloorChecks_AGreenRetakeDoesNotClearATestTheCycleAdded(t *testing.T) {
	in, signals := retakeRepo(t)
	in = floorOffInput(t, in)
	calls := stubUnitRunner(t, false, true)

	out := strings.Join(DefaultBuildFloorChecks(context.Background(), in), "\n")

	if len(*calls) != 2 {
		t.Fatalf("runs = %v, want the first run and one retake", *calls)
	}
	for _, want := range []string{"./q", "TestFlaky", "unit tests FAIL"} {
		if !strings.Contains(out, want) {
			t.Errorf("floor output lacks %q: %q", want, out)
		}
	}
	events := newTestEvents(signals)
	if len(events) != 1 {
		t.Fatalf("signals = %+v, want one BUILD_FLOOR_FLAKY_NEW_TEST", events)
	}
	e := events[0]
	if e.Kind != signalcenter.KindGateRejected || e.Module != signalcenter.ModuleBuild || e.Fields["pkg"] != "./q" || e.Fields["tests"] != "TestFlaky" || e.Fields["retake"] != "green" || e.Fields["class"] != "flaky_new_test" {
		t.Errorf("event = %+v", e)
	}
	if len(retakeEvents(signals)) != 0 {
		t.Errorf("a new-test red must not also report RETAKE_GREEN: %+v", retakeEvents(signals))
	}
}

func TestDefaultBuildFloorChecks_AnInheritedTestKeepsTheGreenRetake(t *testing.T) {
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "go.mod"), "module example.com/m\n\ngo 1.23\n")
	writeFile(t, filepath.Join(repo.Dir, "go", "q", "q.go"), "package q\n\nfunc F() int { return 1 }\n")
	writeFile(t, filepath.Join(repo.Dir, "go", "q", "q_test.go"), "package q\n\nimport \"testing\"\n\nfunc TestFlaky(t *testing.T) { _ = F() }\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	writeFile(t, filepath.Join(repo.Dir, "go", "q", "q.go"), "package q\n\nfunc F() int { return 2 }\n")
	signals := signalcenter.New()
	in := floorOffInput(t, ReviewInput{Phase: string(PhaseBuild), Worktree: repo.Dir, WorktreeBaseSHA: base, Signals: signals, Stderr: &strings.Builder{}})
	stubUnitRunner(t, false, true)

	out := DefaultBuildFloorChecks(context.Background(), in)

	if strings.Contains(strings.Join(out, "\n"), "TestFlaky") || len(newTestEvents(signals)) != 0 {
		t.Fatalf("floor = %v, signals = %+v, want the inherited flake forgiven", out, newTestEvents(signals))
	}
	if events := retakeEvents(signals); len(events) != 1 || events[0].Code != CodeBuildFloorRetakeGreen {
		t.Fatalf("signals = %+v, want one RETAKE_GREEN", events)
	}
}

func TestReadFloorAdded_AnUnreadableChangeKeepsTodaysRuleAndSaysSo(t *testing.T) {
	t.Parallel()
	var stderr strings.Builder
	got := readFloorAdded(ReviewInput{Worktree: t.TempDir(), WorktreeBaseSHA: "deadbeef", Stderr: &stderr})
	if got.pkgs != nil || got.groups != nil || !strings.Contains(stderr.String(), "WARN: the changed files") {
		t.Fatalf("set = %+v, stderr = %q, want an empty set and a WARN", got, stderr.String())
	}
}

func TestReadFloorAdded_NoBaseOrNoWorktreeIsKnownAndSilent(t *testing.T) {
	t.Parallel()
	var stderr strings.Builder
	for _, in := range []ReviewInput{{Stderr: &stderr}, {Worktree: t.TempDir(), Stderr: &stderr}} {
		if got := readFloorAdded(in); got.pkgs != nil || got.groups != nil {
			t.Errorf("set = %+v, want empty", got)
		}
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want no WARN for a known state", stderr.String())
	}
}

func TestReadFloorAdded_AGroupingErrorAndAMissingModuleAndAnExcludedFile(t *testing.T) {
	t.Parallel()
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "go.mod"), "module example.com/m\n\ngo 1.23\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	var stderr strings.Builder
	in := ReviewInput{Worktree: repo.Dir, WorktreeBaseSHA: base, Stderr: &stderr}

	writeFile(t, filepath.Join(repo.Dir, "go", "q", "bad_test.go"), "//go:build (\n\npackage q\n")
	if got := readFloorAdded(in); got.pkgs != nil || !strings.Contains(stderr.String(), "added-test grouping failed") {
		t.Fatalf("set = %+v, stderr = %q, want a grouping WARN", got, stderr.String())
	}
	writeFile(t, filepath.Join(repo.Dir, "go", "q", "bad_test.go"), "//go:build requires_tmux\n\npackage q\n")
	stderr.Reset()
	if got := readFloorAdded(in); !strings.Contains(stderr.String(), "EXCLUDED go/q/bad_test.go") || got.pkgs == nil {
		t.Fatalf("set = %+v, stderr = %q, want the excluded file named", got, stderr.String())
	}
	writeFile(t, filepath.Join(repo.Dir, "go", "q", "bad_test.go"), "package q\n")
	writeFile(t, filepath.Join(repo.Dir, "go", "go.mod"), "go 1.23\n")
	stderr.Reset()
	got := readFloorAdded(in)
	if !strings.Contains(stderr.String(), "module path cannot be read") || got.pkgs != nil || len(got.groups) != 1 {
		t.Fatalf("set = %+v, stderr = %q, want the groups kept, no set and a WARN", got, stderr.String())
	}
}

func TestFloorAdded_NewTestRedsByCallerShape(t *testing.T) {
	t.Parallel()
	a := floorAdded{module: "example.com/m", pkgs: map[string]bool{"./q": true, "./r": true}, groups: []addedtests.Group{{Packages: []string{"./q", "./r"}}}}
	multi := "--- FAIL: TestA (0.00s)\nFAIL\texample.com/m/q\t0.1s\n--- FAIL: TestB/sub (0.00s)\n--- FAIL: TestB (0.00s)\nFAIL\texample.com/m/r\t0.1s\n--- FAIL: TestOld (0.00s)\nFAIL\texample.com/m/old\t0.1s\n"
	cases := []struct {
		name    string
		first   string
		covered []string
		want    []baseattr.Key
	}{
		{"plain caller", retakeRedOutput, []string{"./q"}, []baseattr.Key{{Package: "./q", Test: "TestFlaky"}}},
		{"tagged caller covers the bare package", retakeRedOutput, []string{"./q"}, []baseattr.Key{{Package: "./q", Test: "TestFlaky"}}},
		{"a tag label is not a package", retakeRedOutput, []string{"./q (-tags acs)"}, nil},
		{"cover caller names several packages", multi, []string{"./q", "./r", "./old"}, []baseattr.Key{{Package: "./q", Test: "TestA"}, {Package: "./r", Test: "TestB"}}},
		{"a package the run did not cover", multi, []string{"./r"}, []baseattr.Key{{Package: "./r", Test: "TestB"}}},
		{"a package with no added test", multi, []string{"./old"}, nil},
		{"no FAIL line", "ok\texample.com/m/q\n", []string{"./q"}, nil},
	}
	for _, tc := range cases {
		if got := a.newTestReds(tc.first, tc.covered); fmt.Sprint(got) != fmt.Sprint(tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
	if got := (floorAdded{}).newTestReds(retakeRedOutput, []string{"example.com/m/q"}); got != nil {
		t.Errorf("an empty set must name nothing, got %v", got)
	}
	if got := (floorAdded{}).relative("example.com/m/q"); got != "example.com/m/q" {
		t.Errorf("relative without a module = %q", got)
	}
}

func TestRetakeFloorOutcome_AGreenRetakeOfANewTestIsRedWithTheCorrection(t *testing.T) {
	t.Parallel()
	signals := signalcenter.New()
	a := floorAdded{module: "example.com/m", pkgs: map[string]bool{"./q": true}}
	in := ReviewInput{Signals: signals, Stderr: &strings.Builder{}, added: a}

	out, outcome := in.retakeFloorOutcome("./q", retakeRedOutput, []string{"./q"}, func() (string, floorRetakeOutcome) { return "ok", floorRetakeGreen })

	if outcome != floorRetakeNewTestRed || !strings.Contains(out, "./q.TestFlaky") || !strings.Contains(out, "phases/ship/repocontract.go") || !strings.Contains(out, "--- FAIL: TestFlaky") {
		t.Fatalf("(%q, %v), want the correction after the first output", out, outcome)
	}
	if events := newTestEvents(signals); len(events) != 1 {
		t.Fatalf("signals = %+v", events)
	}

	out, outcome = in.retakeFloorOutcome("./q", retakeRedOutput, []string{"./other"}, func() (string, floorRetakeOutcome) { return "ok", floorRetakeGreen })
	if outcome != floorRetakeGreen || out != "ok" {
		t.Fatalf("(%q, %v), want today's green for an uncovered package", out, outcome)
	}
}

func TestScopedCoverProfile_AGreenRetakeOfANewTestFailsTheCoverPath(t *testing.T) {
	t.Parallel()
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "go.mod"), "module example.com/m\n\ngo 1.23\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	marker := filepath.Join(t.TempDir(), "ran-once")
	writeFile(t, filepath.Join(repo.Dir, "go", "q", "q.go"), "package q\n\nfunc F() int { return 1 }\n")
	writeFile(t, filepath.Join(repo.Dir, "go", "q", "q_test.go"), fmt.Sprintf("package q\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestFlaky(t *testing.T) {\n\t_ = F()\n\tif _, err := os.Stat(%q); err != nil {\n\t\t_ = os.WriteFile(%q, nil, 0o644)\n\t\tt.Fatal(\"first run\")\n\t}\n}\n", marker, marker))
	signals := signalcenter.New()
	in := ReviewInput{Phase: string(PhaseBuild), Worktree: repo.Dir, ProjectRoot: t.TempDir(), WorktreeBaseSHA: base, Signals: signals, Stderr: &strings.Builder{}}.withFloorAdded()

	_, out, status := scopedCoverProfile(context.Background(), in, filepath.Join(repo.Dir, "go"), []string{"./q"}, nil)

	if status != coverStatusTestsFailed || !strings.Contains(out, "example.com/m/q") || !strings.Contains(out, "./q.TestFlaky") {
		t.Fatalf("(%q, %d), want tests failed with the correction", out, status)
	}
	if events := newTestEvents(signals); len(events) != 1 {
		t.Fatalf("signals = %+v", events)
	}
}

const bigTestName = "TestLaneResume_TwoLanesOfOneScopeAdoptAPausedCycleExactlyOnce"

func bigRedOutput(pkg string) string {
	noise := strings.Repeat("    lane_test.go:88: the two lanes adopted the paused cycle one after the other\n", 15)
	return fmt.Sprintf("=== RUN   %s\n--- FAIL: %s (0.01s)\n%sFAIL\nFAIL\texample.com/m/%s\t0.2s\n", bigTestName, bigTestName, noise, pkg)
}

func keepsCorrection(t *testing.T, label, got string) {
	t.Helper()
	for _, want := range []string{"./cmd/evolve." + bigTestName, "Make the test deterministic"} {
		if !strings.Contains(got, want) {
			t.Errorf("%s: output lacks %q: %q", label, want, got)
		}
	}
}

func flakyNewTestJudgeRepo(t *testing.T) (ReviewInput, string) {
	t.Helper()
	in, markers := flakyNewTestsJudgeRepo(t, "TestFlaky")
	return in, markers[0]
}

func flakyNewTestsJudgeRepo(t *testing.T, names ...string) (ReviewInput, []string) {
	t.Helper()
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "go.mod"), "module example.com/m\n\ngo 1.23\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	dir := t.TempDir()
	var body strings.Builder
	body.WriteString("package q\n\nimport (\n\t\"os\"\n\t\"strings\"\n\t\"testing\"\n)\n")
	markers := make([]string, len(names))
	for i, name := range names {
		markers[i] = filepath.Join(dir, "ran-once-"+name)
		fmt.Fprintf(&body, "\nfunc %s(t *testing.T) {\n\t_ = F()\n\tif _, err := os.Stat(%q); err != nil {\n\t\t_ = os.WriteFile(%q, nil, 0o644)\n\t\tt.Fatal(strings.Repeat(\"x\", 1200))\n\t}\n}\n", name, markers[i], markers[i])
	}
	writeFile(t, filepath.Join(repo.Dir, "go", "q", "q.go"), "package q\n\nfunc F() int { return 1 }\n")
	writeFile(t, filepath.Join(repo.Dir, "go", "q", "q_test.go"), body.String())
	root := floorRoot(t, `{"cover_changed":{"stage":"enforce"}}`)
	return ReviewInput{Phase: string(PhaseBuild), Worktree: repo.Dir, ProjectRoot: root, WorktreeBaseSHA: base, Signals: signalcenter.New(), Stderr: &strings.Builder{}}, markers
}

func TestAddedTaggedTestFailures_AGreenRetakeOfANewTaggedTestIsRed(t *testing.T) {
	in, signals := retakeRepo(t)
	writeFile(t, filepath.Join(in.Worktree, "go", "acs", "cycle9", "predicates_test.go"), "//go:build acs\n\npackage cycle9\n\nimport \"testing\"\n\nfunc TestFlaky(t *testing.T) {}\n")
	runs := 0
	prev := buildSelfCheckTaggedRunner
	buildSelfCheckTaggedRunner = func(context.Context, string, string, []string) (string, bool) {
		runs++
		if runs == 1 {
			return "--- FAIL: TestFlaky (0.00s)\nFAIL\texample.com/m/acs/cycle9\t0.2s\n", false
		}
		return "ok\n", true
	}
	t.Cleanup(func() { buildSelfCheckTaggedRunner = prev })

	got := addedTaggedTestFailures(context.Background(), in.withFloorAdded(), filepath.Join(in.Worktree, "go"))

	events := newTestEvents(signals)
	if len(got) != 1 || runs != 2 || len(events) != 1 || events[0].Fields["pkg"] != "./acs/cycle9" {
		t.Fatalf("failures = %v after %d run(s), signals = %+v, want one failure and one FLAKY_NEW_TEST for ./acs/cycle9", got, runs, events)
	}
}

func TestUnitTestFloorJudge_AGreenRetakeOfANewTestIsRed(t *testing.T) {
	in, signals := retakeRepo(t)
	stubUnitRunner(t, false, true)

	out, err := UnitTestFloorJudge(context.Background(), in)

	if err != nil || !strings.Contains(strings.Join(out, "\n"), "./q.TestFlaky") || len(newTestEvents(signals)) != 1 {
		t.Fatalf("(%v, %v), signals = %+v, want the judge to name ./q.TestFlaky", out, err, newTestEvents(signals))
	}
}

func TestCoverChangedFloorJudge_AGreenRetakeOfANewTestIsRed(t *testing.T) {
	t.Parallel()
	in, _ := flakyNewTestJudgeRepo(t)

	out, err := CoverChangedFloorJudge(context.Background(), in)

	if err != nil || !strings.Contains(strings.Join(out, "\n"), "./q.TestFlaky") || len(newTestEvents(in.Signals)) != 1 {
		t.Fatalf("(%v, %v), signals = %+v, want the judge to name ./q.TestFlaky", out, err, newTestEvents(in.Signals))
	}
}

func TestRetakeSelfCheckFailures_ThePlainPathKeepsTheCorrectionOfALongRed(t *testing.T) {
	t.Parallel()
	in := ReviewInput{Signals: signalcenter.New(), Stderr: &strings.Builder{}, added: floorAdded{module: "example.com/m", pkgs: map[string]bool{"./cmd/evolve": true}}}
	first := bigRedOutput("cmd/evolve")

	fails := in.retakeSelfCheckFailures([]selfCheckFailure{{Pkg: "./cmd/evolve", Output: first}}, func(string) (string, bool) { return "ok", true })

	if len(first) < 1200 || len(fails) != 1 {
		t.Fatalf("red = %d bytes, failures = %v, want a long red that fails", len(first), fails)
	}
	keepsCorrection(t, "plain", floorLinesFor(fails, nil, nil)[0])
}

func TestCoverChangedFailures_TheCoverChangedPathKeepsTheCorrectionOfALongRed(t *testing.T) {
	repo, base := coverChangedRepo(t)
	in := floorInput(t, floorRoot(t, `{"cover_changed":{"stage":"enforce"}}`), repo, base)
	in.Signals, in.Stderr = signalcenter.New(), &strings.Builder{}
	in.added = floorAdded{module: "example.com/m", pkgs: map[string]bool{"./cmd/evolve": true}}
	out, outcome := in.retakeFloorOutcome("./cmd/evolve", bigRedOutput("cmd/evolve"), []string{"./cmd/evolve"}, func() (string, floorRetakeOutcome) { return "ok", floorRetakeGreen })
	if outcome != floorRetakeNewTestRed {
		t.Fatalf("outcome = %v, want a new-test red", outcome)
	}
	stubCoverChangedProfiler(t, coverChangedProfile, out, coverStatusTestsFailed)

	failures := coverChangedFailures(context.Background(), in, []string{"go/p/a.go"})

	keepsCorrection(t, "cover-changed", strings.Join(failures, "\n"))
}

func TestApicoverNamingFailures_TheEnforcedPathKeepsTheCorrectionOfALongRed(t *testing.T) {
	t.Parallel()
	in, _ := flakyNewTestJudgeRepo(t)
	in = in.withFloorAdded()
	moduleDir := filepath.Join(in.Worktree, "go")

	got := strings.Join(apicoverNamingFailures(context.Background(), in, moduleDir, []string{"./q"}, []string{"go/q/q.go"}), "\n")

	for _, want := range []string{"enforced package tests FAIL", "./q.TestFlaky", "Make the test deterministic"} {
		if !strings.Contains(got, want) {
			t.Errorf("enforced output lacks %q: %q", want, got)
		}
	}
}

func TestApicoverNamingFailures_TheEnforcedPathKeepsACorrectionLongerThanTheCut(t *testing.T) {
	t.Parallel()
	names := make([]string, 20)
	for i := range names {
		names[i] = fmt.Sprintf("TestFlakyNewTestNumber%dThatRacesOnTheSharedLaneResumeRegistry", i)
	}
	in, _ := flakyNewTestsJudgeRepo(t, names...)
	in = in.withFloorAdded()

	got := strings.Join(apicoverNamingFailures(context.Background(), in, filepath.Join(in.Worktree, "go"), []string{"./q"}, []string{"go/q/q.go"}), "\n")

	for _, name := range names {
		if !strings.Contains(got, "./q."+name) {
			t.Errorf("enforced output lost %s from a correction longer than 600 bytes: %q", name, got)
		}
	}
	if !strings.Contains(got, "…") {
		t.Errorf("enforced output was not cut after the correction: %q", got)
	}
}

func TestCorrectionEnvelope_RoundTripsAndPassesPlainOutputThrough(t *testing.T) {
	t.Parallel()
	attached := attachCorrection("line one\nline two", "--- FAIL: TestA\n")
	correction, rest := splitCorrection(attached)
	if correction != "line one line two" || rest != "--- FAIL: TestA\n" {
		t.Fatalf("split = (%q, %q)", correction, rest)
	}
	for _, plain := range []string{"", "--- FAIL: TestA\nFAIL\n", floorCorrectionPrefix + "no newline"} {
		if c, r := splitCorrection(plain); c != "" || r != plain {
			t.Errorf("splitCorrection(%q) = (%q, %q), want the output unchanged", plain, c, r)
		}
	}
	if got := joinCorrection("", "rest"); got != "rest" {
		t.Errorf("join without a correction = %q", got)
	}
}

func TestCapKeepingCorrection_CutsOnlyTheOutput(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("y", 50)
	if got := capKeepingCorrection(attachCorrection("keep me", long), 10); got != "keep me\n"+long[:10]+"…" {
		t.Errorf("cut = %q", got)
	}
	if got := capKeepingCorrection(long, 10); got != long[:10]+"…" {
		t.Errorf("plain cut = %q", got)
	}
	if got := capKeepingCorrection("short", 10); got != "short" {
		t.Errorf("short = %q", got)
	}
}
