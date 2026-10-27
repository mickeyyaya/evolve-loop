//go:build acs

package cycle2067

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cleanFile = "go/internal/widget/add.go"
	cleanSrc  = "package widget\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n"

	fileWithKillElsewhere = "package widget\n\nimport \"syscall\"\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n\nfunc Reap(pid int) {\n\t_ = syscall.Kill(pid, 9)\n}\n"
	unparsableSrc         = "package widget\n\nfunc Add(a, b int) int {\n\treturn a + \n"
)

type recordingRunner struct {
	report  qualitypanel.MutationReport
	err     error
	scratch []string
	targets [][]qualitypanel.MutationTarget
}

func (r *recordingRunner) Run(scratchDir string, targets []qualitypanel.MutationTarget) (qualitypanel.MutationReport, error) {
	r.scratch = append(r.scratch, scratchDir)
	r.targets = append(r.targets, targets)
	return r.report, r.err
}

func (r *recordingRunner) calls() int { return len(r.scratch) }

func (r *recordingRunner) targetNames() []string {
	var names []string
	for _, batch := range r.targets {
		for _, tg := range batch {
			names = append(names, tg.Name)
		}
	}
	return names
}

type roots struct{ plane, main, scratch string }

func newRoots(t *testing.T) roots {
	t.Helper()
	base := t.TempDir()
	r := roots{plane: filepath.Join(base, "plane"), main: filepath.Join(base, "main"), scratch: filepath.Join(base, "scratch")}
	for _, d := range []string{r.plane, r.main, r.scratch} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

func (r roots) input(fns ...qualitypanel.ChangedFunction) qualitypanel.MutationInput {
	return qualitypanel.MutationInput{Functions: fns, PlaneRoot: r.plane, MainRoot: r.main, ScratchDir: r.scratch, Window: "cycle-2067", Source: "gremlins fake", Cycle: 2067}
}

func fn(file, name, src string) qualitypanel.ChangedFunction {
	return qualitypanel.ChangedFunction{Package: filepath.Dir(file), File: file, Name: name, Source: []byte(src)}
}

func inside(root, path string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	return err == nil && !strings.HasPrefix(rel, "..")
}

func mutants(prefix string, status qualitypanel.MutantStatus, n int, killedBy ...string) []qualitypanel.Mutant {
	out := make([]qualitypanel.Mutant, n)
	for i := range out {
		out[i] = qualitypanel.Mutant{ID: fmt.Sprintf("%s-%d", prefix, i), Status: status, KilledBy: killedBy}
	}
	return out
}

func reportOf(groups ...[]qualitypanel.Mutant) qualitypanel.MutationReport {
	var all []qualitypanel.Mutant
	for _, g := range groups {
		all = append(all, g...)
	}
	return qualitypanel.MutationReport{Mutants: all}
}

func eightKilledTwoLivedOneNotCovered() qualitypanel.MutationReport {
	return reportOf(
		mutants("k", qualitypanel.MutantKilled, 8, "TestAdd"),
		mutants("l", qualitypanel.MutantLived, 2),
		mutants("c", qualitypanel.MutantNotCovered, 1))
}

func measureClean(t *testing.T, report qualitypanel.MutationReport) (qualitypanel.MutationResult, *recordingRunner, roots) {
	t.Helper()
	rt := newRoots(t)
	runner := &recordingRunner{report: report}
	res, err := qualitypanel.MeasureMutation(rt.input(fn(cleanFile, "Add", cleanSrc)), runner)
	if err != nil {
		t.Fatalf("MeasureMutation: %v", err)
	}
	return res, runner, rt
}

func eventsOf(events []signalcenter.Event, kind signalcenter.Kind, index string) []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range events {
		if e.Kind == kind && (index == "" || e.Fields["index"] == index) {
			out = append(out, e)
		}
	}
	return out
}

func near(a, b float64) bool { return math.Abs(a-b) < 0.0005 }

func TestC2067_001_FunctionsThatSendSignalsToProcessesNeverReachTheRunner(t *testing.T) {
	cases := map[string]string{
		"syscall.Kill":    "package widget\n\nimport \"syscall\"\n\nfunc Stop(pid int) {\n\t_ = syscall.Kill(pid, 9)\n}\n",
		"unix.Kill":       "package widget\n\nimport \"golang.org/x/sys/unix\"\n\nfunc Stop(pid int) {\n\t_ = unix.Kill(pid, 9)\n}\n",
		"Process.Kill":    "package widget\n\nimport \"os\"\n\nfunc Stop(p *os.Process) {\n\t_ = p.Kill()\n}\n",
		"Process.Signal":  "package widget\n\nimport \"os\"\n\nfunc Stop(p *os.Process) {\n\t_ = p.Signal(os.Interrupt)\n}\n",
		"aliased syscall": "package widget\n\nimport sc \"syscall\"\n\nfunc Stop(pid int) {\n\t_ = sc.Kill(pid, 9)\n}\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			rt := newRoots(t)
			runner := &recordingRunner{report: eightKilledTwoLivedOneNotCovered()}
			res, err := qualitypanel.MeasureMutation(rt.input(fn("go/internal/widget/stop.go", "Stop", src)), runner)
			if err != nil {
				t.Fatalf("MeasureMutation: %v", err)
			}
			if runner.calls() != 0 {
				t.Errorf("runner got %v for a body that signals a process", runner.targetNames())
			}
			if len(res.Excluded) != 1 || res.Excluded[0].Function.Name != "Stop" {
				t.Errorf("Excluded = %+v, want exactly Stop", res.Excluded)
			}
		})
	}
}

func TestC2067_002_AFunctionWithNoKillCallReachesTheRunnerWithItsLineRange(t *testing.T) {
	res, runner, _ := measureClean(t, eightKilledTwoLivedOneNotCovered())
	if runner.calls() != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls())
	}
	got := runner.targets[0]
	if len(got) != 1 {
		t.Fatalf("targets = %+v, want exactly Add", got)
	}
	want := qualitypanel.MutationTarget{Package: "go/internal/widget", File: cleanFile, Name: "Add", StartLine: 3, EndLine: 5}
	if got[0] != want {
		t.Errorf("target = %+v, want %+v", got[0], want)
	}
	if len(res.Excluded) != 0 || len(res.Refused) != 0 {
		t.Errorf("clean function excluded=%+v refused=%+v", res.Excluded, res.Refused)
	}
}

func TestC2067_003_KillGuardPackagesAreExcludedEvenWithACleanBody(t *testing.T) {
	excluded := []string{"go/internal/proctree/tree.go", "go/internal/gc/sweep.go", "go/internal/bridge/dispatch_reap.go"}
	for _, file := range excluded {
		t.Run(file, func(t *testing.T) {
			rt := newRoots(t)
			runner := &recordingRunner{report: eightKilledTwoLivedOneNotCovered()}
			res, err := qualitypanel.MeasureMutation(rt.input(fn(file, "Add", cleanSrc)), runner)
			if err != nil {
				t.Fatalf("MeasureMutation: %v", err)
			}
			if runner.calls() != 0 || len(res.Excluded) != 1 {
				t.Errorf("%s: runner calls=%d excluded=%d, want 0 and 1", file, runner.calls(), len(res.Excluded))
			}
		})
	}
	t.Run("other bridge files stay measurable", func(t *testing.T) {
		rt := newRoots(t)
		runner := &recordingRunner{report: eightKilledTwoLivedOneNotCovered()}
		if _, err := qualitypanel.MeasureMutation(rt.input(fn("go/internal/bridge/other.go", "Add", cleanSrc)), runner); err != nil {
			t.Fatalf("MeasureMutation: %v", err)
		}
		if runner.calls() != 1 {
			t.Errorf("a bridge file other than dispatch_reap.go was excluded")
		}
	})
}

func TestC2067_004_AFileWithAKillCallElsewhereIsExcludedWhole(t *testing.T) {
	rt := newRoots(t)
	runner := &recordingRunner{report: eightKilledTwoLivedOneNotCovered()}
	res, err := qualitypanel.MeasureMutation(rt.input(fn(cleanFile, "Add", fileWithKillElsewhere)), runner)
	if err != nil {
		t.Fatalf("MeasureMutation: %v", err)
	}
	if runner.calls() != 0 {
		t.Errorf("runner got %v from a file that holds a kill call in Reap", runner.targetNames())
	}
	if len(res.Excluded) != 1 || res.Excluded[0].Function.Name != "Add" {
		t.Errorf("Excluded = %+v, want Add", res.Excluded)
	}
}

func TestC2067_005_ABodyThatDoesNotParseIsRefusedNotPassedThrough(t *testing.T) {
	rt := newRoots(t)
	runner := &recordingRunner{report: eightKilledTwoLivedOneNotCovered()}
	res, err := qualitypanel.MeasureMutation(rt.input(fn(cleanFile, "Add", unparsableSrc), fn("go/internal/widget/ok.go", "Add", cleanSrc)), runner)
	if err != nil {
		t.Fatalf("MeasureMutation: %v", err)
	}
	if len(res.Refused) != 1 || res.Refused[0].Function.File != cleanFile {
		t.Errorf("Refused = %+v, want the unparsable file only", res.Refused)
	}
	for _, tg := range runner.targets[0] {
		if tg.File == cleanFile {
			t.Errorf("unparsable function reached the runner: %+v", tg)
		}
	}
	if len(runner.targets[0]) != 1 {
		t.Errorf("the parsable function must still reach the runner, got %+v", runner.targets[0])
	}
}

func TestC2067_006_NothingToMutateNeverCallsTheRunnerAndReadsNoData(t *testing.T) {
	rt := newRoots(t)
	runner := &recordingRunner{report: eightKilledTwoLivedOneNotCovered()}
	res, err := qualitypanel.MeasureMutation(rt.input(), runner)
	if err != nil {
		t.Fatalf("MeasureMutation: %v", err)
	}
	if runner.calls() != 0 {
		t.Errorf("runner called %d times with no target; gremlins would then mutate the whole package", runner.calls())
	}
	if res.TQ1.N != 0 {
		t.Errorf("TQ1.N = %d, want 0", res.TQ1.N)
	}
	warnings := eventsOf(res.Events(qualitypanel.StageShadow), signalcenter.KindQualityWarning, "TQ1")
	if len(warnings) != 1 || warnings[0].Code != qualitypanel.CodeVacuousDenominator {
		t.Errorf("TQ1 warnings = %+v, want one %s", warnings, qualitypanel.CodeVacuousDenominator)
	}
}

func TestC2067_007_EightKilledTwoLivedOneNotCoveredScoresPointEightWithWilsonBounds(t *testing.T) {
	res, _, _ := measureClean(t, eightKilledTwoLivedOneNotCovered())
	want := qualitypanel.Wilson(8, 10)
	if res.TQ1.Index != "TQ1" || res.TQ1.Method != qualitypanel.MethodWilson95 || res.TQ1.N != 10 {
		t.Errorf("TQ1 = %+v, want index TQ1, method wilson95, n 10", res.TQ1)
	}
	if !near(res.TQ1.Interval.Value, 0.80) || res.TQ1.Interval != want {
		t.Errorf("interval = %+v, want %+v", res.TQ1.Interval, want)
	}
	if !near(res.TQ1.Interval.Lo, 0.4902) || !near(res.TQ1.Interval.Hi, 0.9433) {
		t.Errorf("Wilson bounds = [%.4f, %.4f], want [0.4902, 0.9433]", res.TQ1.Interval.Lo, res.TQ1.Interval.Hi)
	}
	if res.NotCovered != 1 || res.Killed != 8 || res.Lived != 2 {
		t.Errorf("counts killed=%d lived=%d notCovered=%d, want 8 2 1", res.Killed, res.Lived, res.NotCovered)
	}
	readings := eventsOf(res.Events(qualitypanel.StageShadow), signalcenter.KindQualityReading, "TQ1")
	if len(readings) != 1 {
		t.Fatalf("TQ1 reading events = %d, want 1", len(readings))
	}
	f := readings[0].Fields
	if f["value"] != "0.800" || f["n"] != "10" || f["not_covered"] != "1" || f["lo"] != "0.490" || f["hi"] != "0.943" {
		t.Errorf("TQ1 event fields = %v", f)
	}
}

func TestC2067_008_TimedOutCountsAsKilledAndNotViableIsDropped(t *testing.T) {
	report := reportOf(
		mutants("k", qualitypanel.MutantKilled, 7, "TestAdd"),
		mutants("t", qualitypanel.MutantTimedOut, 1),
		mutants("l", qualitypanel.MutantLived, 2),
		mutants("v", qualitypanel.MutantNotViable, 4),
		mutants("c", qualitypanel.MutantNotCovered, 3))
	res, _, _ := measureClean(t, report)
	if res.TQ1.N != 10 || !near(res.TQ1.Interval.Value, 0.80) {
		t.Errorf("TQ1 n=%d value=%.3f, want 10 and 0.800", res.TQ1.N, res.TQ1.Interval.Value)
	}
	if res.NotCovered != 3 {
		t.Errorf("NotCovered = %d, want 3", res.NotCovered)
	}
}

func TestC2067_009_TQ3CountsATestWithZeroUniqueKills(t *testing.T) {
	report := qualitypanel.MutationReport{
		Mutants: []qualitypanel.Mutant{
			{ID: "m1", Status: qualitypanel.MutantKilled, KilledBy: []string{"TestA"}},
			{ID: "m2", Status: qualitypanel.MutantKilled, KilledBy: []string{"TestA", "TestB", "TestD"}},
			{ID: "m3", Status: qualitypanel.MutantKilled, KilledBy: []string{"TestB"}},
			{ID: "m4", Status: qualitypanel.MutantLived},
		},
		TestSeconds: map[string]float64{"TestA": 2, "TestB": 1, "TestC": 1, "TestD": 1},
	}
	res, _, _ := measureClean(t, report)
	if got := strings.Join(res.ZeroUniqueTests, ","); got != "TestC,TestD" {
		t.Errorf("ZeroUniqueTests = %q, want TestC,TestD", got)
	}
	if res.TQ3.Index != "TQ3" || res.TQ3.Method != qualitypanel.MethodCensus || res.TQ3.N != 4 {
		t.Errorf("TQ3 = %+v, want index TQ3, census, n 4", res.TQ3)
	}
	if !near(res.TQ3.Interval.Value, 2.0/5.0) {
		t.Errorf("TQ3 value = %.4f, want 0.4000 (2 unique kills over 5 test-seconds)", res.TQ3.Interval.Value)
	}
}

func TestC2067_010_TQ3WithNoMeasuredTimeIsFiniteNotInfinite(t *testing.T) {
	report := qualitypanel.MutationReport{Mutants: []qualitypanel.Mutant{{ID: "m1", Status: qualitypanel.MutantKilled, KilledBy: []string{"TestA"}}}}
	res, _, _ := measureClean(t, report)
	v := res.TQ3.Interval.Value
	if math.IsNaN(v) || math.IsInf(v, 0) {
		t.Errorf("TQ3 value = %v with zero test seconds, want a finite number", v)
	}
}

func livedOnlyReport() qualitypanel.MutationReport {
	return reportOf(mutants("l", qualitypanel.MutantLived, 30))
}

func TestC2067_011_TheIndexStartsAtShadowAndEmitsNoBreach(t *testing.T) {
	for _, id := range []string{"TQ1", "TQ3"} {
		if got := qualitypanel.StageFor(id, nil); got != qualitypanel.StageShadow {
			t.Errorf("StageFor(%s) = %s, want shadow", id, got)
		}
	}
	res, _, _ := measureClean(t, livedOnlyReport())
	if res.TQ1.N != 30 || res.TQ1.Interval.Value != 0 {
		t.Fatalf("fixture must score TQ1 at 0 over 30 mutants, got %+v", res.TQ1)
	}
	if breaches := eventsOf(res.Events(qualitypanel.StageShadow), signalcenter.KindQualityBreach, ""); len(breaches) != 0 {
		t.Errorf("shadow stage emitted %d breach events", len(breaches))
	}
	atReport := eventsOf(res.Events(qualitypanel.StageReport), signalcenter.KindQualityBreach, "")
	if len(atReport) != 1 || atReport[0].Code != qualitypanel.BreachCode("TQ1") {
		t.Errorf("control: stage report must breach on 0 of 30, got %+v", atReport)
	}
	if shadow := eventsOf(res.Events(qualitypanel.StageShadow), signalcenter.KindQualityReading, "TQ1"); len(shadow) != 1 || shadow[0].Fields["stage"] != "shadow" {
		t.Errorf("shadow reading event = %+v", shadow)
	}
}

func TestC2067_012_TheRunnerIsAlwaysHandedAPathOutsideThePlaneAndMain(t *testing.T) {
	_, runner, rt := measureClean(t, eightKilledTwoLivedOneNotCovered())
	if runner.calls() != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls())
	}
	got := runner.scratch[0]
	if got != rt.scratch || inside(rt.plane, got) || inside(rt.main, got) {
		t.Errorf("runner scratch = %q, want %q outside %q and %q", got, rt.scratch, rt.plane, rt.main)
	}
}

func TestC2067_013_AScratchPathInsideThePlaneOrMainIsRefusedBeforeTheRunnerRuns(t *testing.T) {
	rt := newRoots(t)
	cases := map[string]string{
		"inside plane":      filepath.Join(rt.plane, "scratch"),
		"inside main":       filepath.Join(rt.main, "deep", "scratch"),
		"plane itself":      rt.plane,
		"main itself":       rt.main,
		"dot-dot into main": filepath.Join(rt.scratch, "..", "main", "x"),
		"empty":             "",
	}
	for name, scratch := range cases {
		t.Run(name, func(t *testing.T) {
			runner := &recordingRunner{report: eightKilledTwoLivedOneNotCovered()}
			in := rt.input(fn(cleanFile, "Add", cleanSrc))
			in.ScratchDir = scratch
			if _, err := qualitypanel.MeasureMutation(in, runner); err == nil {
				t.Errorf("scratch %q was accepted", scratch)
			}
			if runner.calls() != 0 {
				t.Errorf("runner ran with scratch %q", scratch)
			}
		})
	}
}

func TestC2067_014_ARunnerFailureIsReturnedNotSwallowed(t *testing.T) {
	rt := newRoots(t)
	runner := &recordingRunner{err: fmt.Errorf("gremlins exploded")}
	if _, err := qualitypanel.MeasureMutation(rt.input(fn(cleanFile, "Add", cleanSrc)), runner); err == nil || !strings.Contains(err.Error(), "gremlins exploded") {
		t.Errorf("err = %v, want it to wrap the runner failure", err)
	}
}

func TestC2067_015_MeasuringWithAFakeRunnerNeverStartsTheRealGremlinsBinary(t *testing.T) {
	shim := t.TempDir()
	marker := filepath.Join(shim, "gremlins-ran")
	script := "#!/bin/sh\ntouch " + marker + "\n"
	if err := os.WriteFile(filepath.Join(shim, "gremlins"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shim+string(os.PathListSeparator)+os.Getenv("PATH"))
	measureClean(t, eightKilledTwoLivedOneNotCovered())
	rt := newRoots(t)
	_, _ = qualitypanel.MeasureMutation(rt.input(fn(cleanFile, "Add", fileWithKillElsewhere), fn("go/internal/gc/x.go", "Add", cleanSrc), fn(cleanFile, "Add", unparsableSrc)), &recordingRunner{})
	if _, err := os.Stat(marker); err == nil {
		t.Errorf("a gremlins binary on PATH was started by MeasureMutation")
	}
}

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func coverFuncOf(t *testing.T, pkg string) (string, string) {
	t.Helper()
	scratch := t.TempDir()
	profile := filepath.Join(scratch, "cover.out")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	var coverFunc string
	for _, args := range [][]string{
		{"test", "-count=1", "-coverprofile=" + profile, pkg},
		{"tool", "cover", "-func=" + profile},
	} {
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = goDir(t)
		cmd.Env = ipcenv.GoTestEnv()
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("go %s: %v\n%s%s", strings.Join(args, " "), err, stdout.String(), stderr.String())
		}
		coverFunc = stdout.String()
	}
	path := filepath.Join(scratch, "cover.func")
	if err := os.WriteFile(path, []byte(coverFunc), 0o644); err != nil {
		t.Fatal(err)
	}
	return coverFunc, path
}

func TestC2067_016_EveryFunctionOfTheQualityPanelIsAtFullCoverageAndTheNewOnesExist(t *testing.T) {
	coverFunc, _ := coverFuncOf(t, "./internal/qualitypanel")
	sawMutation := false
	for _, line := range strings.Split(coverFunc, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] == "total:" {
			continue
		}
		if strings.Contains(fields[0], "/mutation") {
			sawMutation = true
		}
		if pct := fields[len(fields)-1]; pct != "100.0%" {
			t.Errorf("not at 100%%: %s", line)
		}
	}
	if !sawMutation {
		t.Errorf("the cover profile lists no function of a mutation*.go file")
	}
}

func TestC2067_017_ApicoverNamesAndCoversEveryNewExportOfTheQualityPanel(t *testing.T) {
	dir := filepath.Join(goDir(t), "internal", "qualitypanel")
	symbols, err := apicover.Enumerate(context.Background(), dir)
	if err != nil {
		t.Fatalf("enumerate %s: %v", dir, err)
	}
	exported := map[string]bool{}
	for _, s := range symbols {
		exported[s.Name] = true
	}
	for _, name := range []string{"MeasureMutation", "MutationRunner", "MutationInput", "MutationResult", "MutationReport", "Mutant", "MutantStatus", "ChangedFunction", "MutationTarget"} {
		if !exported[name] {
			t.Errorf("internal/qualitypanel exports no %s", name)
		}
	}
	_, coverPath := coverFuncOf(t, "./internal/qualitypanel")
	var report bytes.Buffer
	code, err := apicover.Run(context.Background(), apicover.Config{Dirs: []string{dir}, CoverPath: coverPath, Enforce: true}, &report)
	if err != nil || code != 0 {
		t.Errorf("apicover on internal/qualitypanel exited %d (err=%v):\n%s", code, err, report.String())
	}
}
