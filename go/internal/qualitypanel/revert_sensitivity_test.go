package qualitypanel

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type fakeRevertRunner struct {
	outcomes map[string]RevertOutcome
	fallback RevertOutcome
	err      error
	scratch  []string
	files    []string
}

func (f *fakeRevertRunner) Revert(scratchDir string, h Hunk) (RevertOutcome, error) {
	f.scratch = append(f.scratch, scratchDir)
	f.files = append(f.files, h.File)
	if f.err != nil {
		return "", f.err
	}
	if o, ok := f.outcomes[h.Patch]; ok {
		return o, nil
	}
	return f.fallback, nil
}

func hunkOf(file, patch string) Hunk {
	return Hunk{Package: path.Dir(file), File: file, Patch: patch}
}

func revertInputFor(t *testing.T, hunks ...Hunk) RevertInput {
	t.Helper()
	base := t.TempDir()
	return RevertInput{Hunks: hunks, PlaneRoot: filepath.Join(base, "plane"), MainRoot: filepath.Join(base, "main"), ScratchDir: filepath.Join(base, "scratch"), Window: "cycle-9", Source: "fake", Cycle: 9}
}

func manyHunks(n int) []Hunk {
	out := make([]Hunk, n)
	for i := range out {
		out[i] = hunkOf("go/internal/widget/add.go", fmt.Sprintf("patch-%d\n", i))
	}
	return out
}

func kindsOf(events []signalcenter.Event) []signalcenter.Kind {
	var out []signalcenter.Kind
	for _, e := range events {
		out = append(out, e.Kind)
	}
	return out
}

func TestRevertSensitivity_AHunkWhoseTestsPassWithoutItIsInsensitive(t *testing.T) {
	t.Parallel()
	h := hunkOf("go/internal/widget/add.go", "slack\n")
	res, err := MeasureRevertSensitivity(revertInputFor(t, h), &fakeRevertRunner{fallback: RevertTestsPassed})
	if err != nil {
		t.Fatal(err)
	}
	if res.Insensitive != 1 || res.Sensitive != 0 || len(res.InsensitiveHunks) != 1 || res.InsensitiveHunks[0].Patch != "slack\n" {
		t.Errorf("insensitive=%d sensitive=%d hunks=%+v, want 1 0 [slack]", res.Insensitive, res.Sensitive, res.InsensitiveHunks)
	}
	if res.TQ2.Interval.Value != 0 || res.TQ2.N != 1 {
		t.Errorf("TQ2 value=%v n=%d, want 0 1", res.TQ2.Interval.Value, res.TQ2.N)
	}
}

func TestRevertSensitivity_AHunkWhoseTestsFailWithoutItIsSensitive(t *testing.T) {
	t.Parallel()
	h := hunkOf("go/internal/widget/add.go", "guarded\n")
	res, err := MeasureRevertSensitivity(revertInputFor(t, h), &fakeRevertRunner{fallback: RevertTestsFailed})
	if err != nil {
		t.Fatal(err)
	}
	if res.Sensitive != 1 || res.Insensitive != 0 || len(res.InsensitiveHunks) != 0 {
		t.Errorf("sensitive=%d insensitive=%d hunks=%+v, want 1 0 none", res.Sensitive, res.Insensitive, res.InsensitiveHunks)
	}
	if res.TQ2.Interval.Value != 1 || res.TQ2.N != 1 {
		t.Errorf("TQ2 value=%v n=%d, want 1 1", res.TQ2.Interval.Value, res.TQ2.N)
	}
}

func TestRevertSensitivity_AnUncompilableRevertCountsSensitiveAndIsReported(t *testing.T) {
	t.Parallel()
	broken := hunkOf("go/internal/widget/add.go", "needed-by-a-test\n")
	res, err := MeasureRevertSensitivity(revertInputFor(t, broken), &fakeRevertRunner{fallback: RevertNotCompiled})
	if err != nil {
		t.Fatal(err)
	}
	if res.Sensitive != 1 || res.Insensitive != 0 {
		t.Errorf("sensitive=%d insensitive=%d, want 1 0", res.Sensitive, res.Insensitive)
	}
	if len(res.UncompilableHunks) != 1 || res.UncompilableHunks[0].Patch != "needed-by-a-test\n" {
		t.Errorf("uncompilable = %+v, want the broken hunk", res.UncompilableHunks)
	}
	if res.TQ2.Extra["uncompilable"] != "1" {
		t.Errorf("TQ2.Extra[uncompilable] = %q, want 1", res.TQ2.Extra["uncompilable"])
	}
}

func TestRevertSensitivity_TheValueIsTheSensitiveShareWithAWilsonInterval(t *testing.T) {
	t.Parallel()
	hunks := []Hunk{hunkOf("a/x.go", "p1\n"), hunkOf("a/x.go", "p2\n"), hunkOf("a/x.go", "p3\n"), hunkOf("a/x.go", "p4\n")}
	runner := &fakeRevertRunner{fallback: RevertTestsFailed, outcomes: map[string]RevertOutcome{"p4\n": RevertTestsPassed}}
	res, err := MeasureRevertSensitivity(revertInputFor(t, hunks...), runner)
	if err != nil {
		t.Fatal(err)
	}
	want := Wilson(3, 4)
	if res.TQ2.Interval != want || res.TQ2.N != 4 || res.TQ2.Method != MethodWilson95 {
		t.Errorf("TQ2 = %+v, want interval %+v n 4 method wilson95", res.TQ2, want)
	}
	if res.TQ2.Index != "TQ2" || res.TQ2.Window != "cycle-9" || res.TQ2.Source != "fake" || res.TQ2.Cycle != 9 {
		t.Errorf("TQ2 labels = %q %q %q %d, want TQ2 cycle-9 fake 9", res.TQ2.Index, res.TQ2.Window, res.TQ2.Source, res.TQ2.Cycle)
	}
}

func TestRevertSensitivity_NoHunksMeansNoRunnerCallAndAVacuousDenominatorWarning(t *testing.T) {
	t.Parallel()
	runner := &fakeRevertRunner{}
	res, err := MeasureRevertSensitivity(revertInputFor(t), runner)
	if err != nil {
		t.Fatal(err)
	}
	if len(runner.scratch) != 0 || res.TQ2.N != 0 {
		t.Errorf("runner calls=%d n=%d, want 0 0", len(runner.scratch), res.TQ2.N)
	}
	var warned bool
	for _, e := range res.Events(StageShadow) {
		warned = warned || (e.Kind == signalcenter.KindQualityWarning && e.Code == CodeVacuousDenominator)
	}
	if !warned {
		t.Errorf("events = %v, want a quality.warning with QUALITY_VACUOUS_DENOMINATOR", kindsOf(res.Events(StageShadow)))
	}
}

func TestRevertSensitivity_AKillGuardPathHunkIsExcludedAndNeverReachesTheRunner(t *testing.T) {
	t.Parallel()
	for _, file := range []string{"go/internal/proctree/a.go", "go/internal/gc/a.go", "go/internal/bridge/dispatch_reap.go"} {
		runner := &fakeRevertRunner{fallback: RevertTestsFailed}
		res, err := MeasureRevertSensitivity(revertInputFor(t, hunkOf(file, "p\n")), runner)
		if err != nil || len(runner.scratch) != 0 || len(res.Excluded) != 1 || !strings.Contains(res.Excluded[0].Reason, "kill guard") || res.TQ2.N != 0 {
			t.Errorf("%s: err=%v calls=%d excluded=%+v n=%d", file, err, len(runner.scratch), res.Excluded, res.TQ2.N)
		}
	}
	runner := &fakeRevertRunner{fallback: RevertTestsFailed}
	if _, err := MeasureRevertSensitivity(revertInputFor(t, hunkOf("go/internal/bridge/other.go", "p\n")), runner); err != nil || len(runner.scratch) != 1 {
		t.Errorf("a bridge file other than dispatch_reap.go must reach the runner: err=%v calls=%d", err, len(runner.scratch))
	}
}

func TestRevertSensitivity_AScratchInsideTheTreesOrNotAbsoluteIsRefusedBeforeAnyRun(t *testing.T) {
	t.Parallel()
	for name, mutate := range map[string]func(*RevertInput){
		"scratch inside plane": func(in *RevertInput) { in.ScratchDir = filepath.Join(in.PlaneRoot, "scratch") },
		"scratch inside main":  func(in *RevertInput) { in.ScratchDir = filepath.Join(in.MainRoot, "deep", "scratch") },
		"scratch is plane":     func(in *RevertInput) { in.ScratchDir = in.PlaneRoot },
		"relative scratch":     func(in *RevertInput) { in.ScratchDir = "scratch" },
		"relative plane":       func(in *RevertInput) { in.PlaneRoot = "plane" },
	} {
		in := revertInputFor(t, hunkOf("a/x.go", "p\n"))
		mutate(&in)
		runner := &fakeRevertRunner{fallback: RevertTestsFailed}
		if _, err := MeasureRevertSensitivity(in, runner); err == nil || len(runner.scratch) != 0 {
			t.Errorf("%s: err=%v calls=%d, want an error and 0 calls", name, err, len(runner.scratch))
		}
	}
}

func TestRevertSensitivity_EveryHunkIsRunInTheCleanedScratchDirInInputOrder(t *testing.T) {
	t.Parallel()
	in := revertInputFor(t, hunkOf("a/one.go", "1\n"), hunkOf("b/two.go", "2\n"))
	in.ScratchDir += string(filepath.Separator) + "." + string(filepath.Separator)
	runner := &fakeRevertRunner{fallback: RevertTestsFailed}
	if _, err := MeasureRevertSensitivity(in, runner); err != nil {
		t.Fatal(err)
	}
	want := filepath.Clean(in.ScratchDir)
	if len(runner.scratch) != 2 || runner.scratch[0] != want || runner.scratch[1] != want || runner.files[0] != "a/one.go" || runner.files[1] != "b/two.go" {
		t.Errorf("scratch=%v files=%v, want 2 calls in %q in input order", runner.scratch, runner.files, want)
	}
}

func TestRevertSensitivity_ARunnerErrorFailsTheMeasurementLoudly(t *testing.T) {
	t.Parallel()
	runner := &fakeRevertRunner{err: fmt.Errorf("git apply exploded")}
	_, err := MeasureRevertSensitivity(revertInputFor(t, hunkOf("a/x.go", "p\n")), runner)
	if err == nil || !strings.Contains(err.Error(), "revert runner") || !strings.Contains(err.Error(), "git apply exploded") {
		t.Errorf("err = %v, want it to wrap the runner error under \"revert runner\"", err)
	}
}

func TestRevertSensitivity_AnOutcomeOutsideTheThreeKnownOnesIsAnError(t *testing.T) {
	t.Parallel()
	runner := &fakeRevertRunner{fallback: RevertOutcome("flaky")}
	if _, err := MeasureRevertSensitivity(revertInputFor(t, hunkOf("a/x.go", "p\n")), runner); err == nil || !strings.Contains(err.Error(), "flaky") {
		t.Errorf("err = %v, want an error naming the unknown outcome", err)
	}
}

func TestRevertSensitivity_TQ2StartsAtStageShadowSoNoBreachEverFiresThere(t *testing.T) {
	t.Parallel()
	if got := StageFor("TQ2", nil); got != StageShadow {
		t.Fatalf("StageFor(TQ2) = %s, want %s", got, StageShadow)
	}
	res, err := MeasureRevertSensitivity(revertInputFor(t, manyHunks(15)...), &fakeRevertRunner{fallback: RevertTestsPassed})
	if err != nil {
		t.Fatal(err)
	}
	if got := kindsOf(res.Events(StageShadow)); len(got) != 1 || got[0] != signalcenter.KindQualityReading {
		t.Errorf("shadow kinds = %v, want only quality.reading", got)
	}
	var breach *signalcenter.Event
	for _, e := range res.Events(StageReport) {
		if e.Kind == signalcenter.KindQualityBreach {
			e := e
			breach = &e
		}
	}
	if breach == nil || breach.Code != BreachCode("TQ2") {
		t.Errorf("report stage breach = %+v, want %s: 0 of 15 hunks sensitive is the whole interval below 0.90", breach, BreachCode("TQ2"))
	}
}

const fixtureWidgetTail = "func Pad5() int {\n\treturn 5\n}\n\nfunc Pad6() int {\n\treturn 6\n}\n\nfunc Pad7() int {\n\treturn 7\n}\n\nfunc Pad8() int {\n\treturn 8\n}\n"

func fixtureWidget(doubleBody, tagBody string, withTriple bool) string {
	src := "package widget\n\nfunc Double(n int) int {\n\treturn " + doubleBody + "\n}\n\n" +
		"func Pad1() int {\n\treturn 1\n}\n\nfunc Pad2() int {\n\treturn 2\n}\n\nfunc Pad3() int {\n\treturn 3\n}\n\nfunc Pad4() int {\n\treturn 4\n}\n\n" +
		"func Tag() string {\n\treturn " + tagBody + "\n}\n\n" + fixtureWidgetTail
	if withTriple {
		src += "\nfunc Triple(n int) int {\n\treturn n * 3\n}\n"
	}
	return src
}

const fixtureWidgetTest = "package widget\n\nimport \"testing\"\n\nfunc TestDouble(t *testing.T) {\n\tif got := Double(3); got != 6 {\n\t\tt.Fatalf(\"Double(3) = %d, want 6\", got)\n\t}\n}\n\nfunc TestTriple(t *testing.T) {\n\tif got := Triple(2); got != 6 {\n\t\tt.Fatalf(\"Triple(2) = %d, want 6\", got)\n\t}\n}\n"

func shipFixture(t *testing.T) (repo *gittest.Repo, base, ship string) {
	t.Helper()
	repo = gittest.Fixture(t)
	repo.Write("go.mod", "module fixture\n\ngo 1.21\n")
	repo.Write("widget.go", fixtureWidget("n", `"a"`, false))
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base = repo.Git("rev-parse", "HEAD")
	repo.Write("widget.go", fixtureWidget("n * 2", `"b"`, true))
	repo.Write("widget_test.go", fixtureWidgetTest)
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "ship")
	ship = repo.Git("rev-parse", "HEAD")
	return repo, base, ship
}

func TestParseHunks_OneHunkPerChangeOfAProductionGoFileAndNothingElse(t *testing.T) {
	t.Parallel()
	diff := strings.Join([]string{
		"diff --git a/go/internal/widget/add.go b/go/internal/widget/add.go",
		"index 111..222 100644",
		"--- a/go/internal/widget/add.go",
		"+++ b/go/internal/widget/add.go",
		"@@ -1,3 +1,3 @@",
		" package widget",
		"-func A() {}",
		"+func A() int { return 1 }",
		"@@ -10,3 +10,3 @@",
		" x",
		"-y",
		"+z",
		"diff --git a/go/internal/widget/add_test.go b/go/internal/widget/add_test.go",
		"--- a/go/internal/widget/add_test.go",
		"+++ b/go/internal/widget/add_test.go",
		"@@ -1 +1 @@",
		"-a",
		"+b",
		"diff --git a/README.md b/README.md",
		"--- a/README.md",
		"+++ b/README.md",
		"@@ -1 +1 @@",
		"-a",
		"+b",
		"diff --git a/main.go b/main.go",
		"--- a/main.go",
		"+++ b/main.go",
		"@@ -1 +1 @@",
		"-a",
		"+b",
	}, "\n")
	got := ParseHunks(diff)
	if len(got) != 3 {
		t.Fatalf("hunks = %d, want 3 (two in add.go, one in main.go; the test file and README are not production Go)", len(got))
	}
	for i, wantFile := range []string{"go/internal/widget/add.go", "go/internal/widget/add.go", "main.go"} {
		h := got[i]
		if h.File != wantFile || h.Package != path.Dir(wantFile) {
			t.Errorf("hunk %d file=%q package=%q, want %q %q", i, h.File, h.Package, wantFile, path.Dir(wantFile))
		}
		if !strings.HasPrefix(h.Patch, "diff --git a/"+wantFile+" b/"+wantFile+"\n") || strings.Count(h.Patch, "\n@@ ")+boolInt(strings.HasPrefix(h.Patch, "@@ ")) != 1 || !strings.HasSuffix(h.Patch, "\n") {
			t.Errorf("hunk %d patch = %q, want the file header, exactly one hunk and a closing newline", i, h.Patch)
		}
	}
	if !strings.Contains(got[1].Patch, "-y") || strings.Contains(got[1].Patch, "return 1") {
		t.Errorf("second hunk patch = %q, want only its own change", got[1].Patch)
	}
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func TestParseHunks_AnEmptyDiffHasNoHunks(t *testing.T) {
	t.Parallel()
	if got := ParseHunks(""); len(got) != 0 {
		t.Errorf("hunks = %+v, want none", got)
	}
}

func TestGitRevertRunner_TheShipFixtureSplitsIntoSensitiveInsensitiveAndUncompilableHunks(t *testing.T) {
	repo, base, ship := shipFixture(t)
	hunks := ParseHunks(repo.Git("diff", base, ship))
	if len(hunks) != 3 {
		t.Fatalf("hunks = %d, want 3 (Double, Tag, Triple)", len(hunks))
	}
	tmp := t.TempDir()
	in := RevertInput{Hunks: hunks, PlaneRoot: repo.Dir, MainRoot: filepath.Join(tmp, "main"), ScratchDir: filepath.Join(tmp, "scratch"), Window: "cycle-9", Source: "git", Cycle: 9}
	res, err := MeasureRevertSensitivity(in, GitRevertRunner{Repo: repo.Dir, Commit: ship})
	if err != nil {
		t.Fatal(err)
	}
	if res.Sensitive != 2 || res.Insensitive != 1 {
		t.Fatalf("sensitive=%d insensitive=%d, want 2 1", res.Sensitive, res.Insensitive)
	}
	if len(res.InsensitiveHunks) != 1 || !strings.Contains(res.InsensitiveHunks[0].Patch, `"b"`) {
		t.Errorf("insensitive hunks = %+v, want the untested Tag change", res.InsensitiveHunks)
	}
	if len(res.UncompilableHunks) != 1 || !strings.Contains(res.UncompilableHunks[0].Patch, "Triple") {
		t.Errorf("uncompilable hunks = %+v, want the Triple addition the test needs", res.UncompilableHunks)
	}
	if res.TQ2.Interval != Wilson(2, 3) {
		t.Errorf("TQ2 interval = %+v, want Wilson(2, 3)", res.TQ2.Interval)
	}
}

func TestGitRevertRunner_ProbingNeverWritesInThePlaneTreeAndLeavesNoScratchBehind(t *testing.T) {
	repo, base, ship := shipFixture(t)
	hunks := ParseHunks(repo.Git("diff", base, ship))
	sourceBefore, err := os.ReadFile(filepath.Join(repo.Dir, "widget.go"))
	if err != nil {
		t.Fatal(err)
	}
	headBefore := repo.Git("rev-parse", "HEAD")
	tmp := t.TempDir()
	scratch := filepath.Join(tmp, "scratch")
	in := RevertInput{Hunks: hunks, PlaneRoot: repo.Dir, MainRoot: filepath.Join(tmp, "main"), ScratchDir: scratch, Window: "cycle-9", Source: "git", Cycle: 9}
	if _, err := MeasureRevertSensitivity(in, GitRevertRunner{Repo: repo.Dir, Commit: ship}); err != nil {
		t.Fatal(err)
	}
	sourceAfter, err := os.ReadFile(filepath.Join(repo.Dir, "widget.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(sourceAfter) != string(sourceBefore) {
		t.Errorf("widget.go in the plane tree changed during the probe")
	}
	if status := repo.Git("status", "--porcelain"); status != "" {
		t.Errorf("plane tree status = %q, want clean", status)
	}
	if head := repo.Git("rev-parse", "HEAD"); head != headBefore {
		t.Errorf("HEAD moved from %s to %s", headBefore, head)
	}
	if _, err := os.Stat(scratch); !os.IsNotExist(err) {
		t.Errorf("scratch %s still exists after the probe (stat err = %v)", scratch, err)
	}
	if list := repo.Git("worktree", "list"); strings.Count(list, "\n") != 0 {
		t.Errorf("git worktree list = %q, want only the plane tree", list)
	}
}

func TestGitRevertRunner_AHunkThatDoesNotApplyIsAnErrorNotASilentOutcome(t *testing.T) {
	repo, _, ship := shipFixture(t)
	tmp := t.TempDir()
	bogus := Hunk{Package: ".", File: "widget.go", Patch: "diff --git a/widget.go b/widget.go\n--- a/widget.go\n+++ b/widget.go\n@@ -1,3 +1,3 @@\n nothing\n-like\n+this\n"}
	outcome, err := GitRevertRunner{Repo: repo.Dir, Commit: ship}.Revert(filepath.Join(tmp, "scratch"), bogus)
	if err == nil {
		t.Errorf("outcome = %q, err = nil, want an error for a patch that does not apply", outcome)
	}
}
