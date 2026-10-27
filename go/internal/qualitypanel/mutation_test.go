package qualitypanel

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	addFile = "go/internal/widget/add.go"
	addSrc  = "package widget\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n"
)

type fakeRunner struct {
	report  MutationReport
	err     error
	scratch []string
	targets [][]MutationTarget
}

func (f *fakeRunner) Run(scratchDir string, targets []MutationTarget) (MutationReport, error) {
	f.scratch = append(f.scratch, scratchDir)
	f.targets = append(f.targets, targets)
	return f.report, f.err
}

func inputFor(t *testing.T, fns ...ChangedFunction) MutationInput {
	t.Helper()
	base := t.TempDir()
	return MutationInput{Functions: fns, PlaneRoot: filepath.Join(base, "plane"), MainRoot: filepath.Join(base, "main"), ScratchDir: filepath.Join(base, "scratch"), Window: "cycle-9", Source: "fake", Cycle: 9}
}

func addFn(file, src string) ChangedFunction {
	return ChangedFunction{Package: filepath.Dir(file), File: file, Name: "Add", Source: []byte(src)}
}

func killedBy(id string, tests ...string) Mutant {
	return Mutant{ID: id, Status: MutantKilled, KilledBy: tests}
}

func TestMeasureMutation_ASignalCallInTheFileKeepsTheFunctionFromTheRunner(t *testing.T) {
	t.Parallel()
	for name, call := range map[string]string{"syscall.Kill": "syscall.Kill(1, 9)", "Killpg": "unix.Killpg(1, 9)", "Process.Kill": "p.Kill()", "Process.Signal": "p.Signal(nil)"} {
		src := "package widget\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n\nfunc Stop(p P) {\n\t_ = " + call + "\n}\n"
		runner := &fakeRunner{}
		res, err := MeasureMutation(inputFor(t, addFn(addFile, src)), runner)
		if err != nil || len(runner.scratch) != 0 || len(res.Excluded) != 1 {
			t.Errorf("%s: err=%v runner calls=%d excluded=%d, want nil 0 1", name, err, len(runner.scratch), len(res.Excluded))
		}
	}
}

func TestMeasureMutation_AKillGuardPathIsExcludedWithItsReason(t *testing.T) {
	t.Parallel()
	for _, file := range []string{"go/internal/proctree/a.go", "go/internal/gc/a.go", "go/internal/bridge/dispatch_reap.go"} {
		runner := &fakeRunner{}
		res, err := MeasureMutation(inputFor(t, addFn(file, addSrc)), runner)
		if err != nil || len(runner.scratch) != 0 || len(res.Excluded) != 1 || !strings.Contains(res.Excluded[0].Reason, "kill guard") {
			t.Errorf("%s: err=%v calls=%d excluded=%+v", file, err, len(runner.scratch), res.Excluded)
		}
	}
	runner := &fakeRunner{}
	if _, err := MeasureMutation(inputFor(t, addFn("go/internal/bridge/other.go", addSrc)), runner); err != nil || len(runner.scratch) != 1 {
		t.Errorf("a bridge file other than dispatch_reap.go must reach the runner: err=%v calls=%d", err, len(runner.scratch))
	}
}

func TestMeasureMutation_ARefusedBodyNeverReachesTheRunner(t *testing.T) {
	t.Parallel()
	unparsable := addFn(addFile, "package widget\n\nfunc Add(a, b int) int {\n\treturn a + \n")
	missing := ChangedFunction{Package: "p", File: "p/x.go", Name: "Gone", Source: []byte(addSrc)}
	runner := &fakeRunner{}
	res, err := MeasureMutation(inputFor(t, unparsable, missing), runner)
	if err != nil || len(runner.scratch) != 0 || len(res.Refused) != 2 {
		t.Fatalf("err=%v calls=%d refused=%d, want nil 0 2", err, len(runner.scratch), len(res.Refused))
	}
	if !strings.Contains(res.Refused[0].Reason, "does not parse") || !strings.Contains(res.Refused[1].Reason, "no function named Gone") {
		t.Errorf("reasons = %q and %q", res.Refused[0].Reason, res.Refused[1].Reason)
	}
}

func TestMeasureMutation_ACleanFunctionReachesTheRunnerWithItsLineRange(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{}
	in := inputFor(t, addFn(addFile, addSrc))
	if _, err := MeasureMutation(in, runner); err != nil {
		t.Fatal(err)
	}
	want := []MutationTarget{{Package: "go/internal/widget", File: addFile, Name: "Add", StartLine: 3, EndLine: 5}}
	if !reflect.DeepEqual(runner.targets, [][]MutationTarget{want}) || runner.scratch[0] != in.ScratchDir {
		t.Errorf("targets=%+v scratch=%v, want %+v and %s", runner.targets, runner.scratch, want, in.ScratchDir)
	}
}

func TestMeasureMutation_AScratchPathInsideAnyTreeOrRelativeIsRefused(t *testing.T) {
	t.Parallel()
	base := inputFor(t, addFn(addFile, addSrc))
	cases := map[string]func(*MutationInput){
		"inside plane": func(in *MutationInput) { in.ScratchDir = filepath.Join(in.PlaneRoot, "s") },
		"inside main":  func(in *MutationInput) { in.ScratchDir = filepath.Join(in.MainRoot, "a", "s") },
		"plane itself": func(in *MutationInput) { in.ScratchDir = in.PlaneRoot },
		"empty":        func(in *MutationInput) { in.ScratchDir = "" },
		"relative":     func(in *MutationInput) { in.ScratchDir = "scratch" },
		"no plane":     func(in *MutationInput) { in.PlaneRoot = "" },
		"no main":      func(in *MutationInput) { in.MainRoot = "" },
	}
	for name, mutate := range cases {
		in := base
		mutate(&in)
		runner := &fakeRunner{}
		if _, err := MeasureMutation(in, runner); err == nil || len(runner.scratch) != 0 {
			t.Errorf("%s: err=%v calls=%d, want an error and no run", name, err, len(runner.scratch))
		}
	}
	sibling := base
	sibling.ScratchDir = base.PlaneRoot + "-scratch"
	if _, err := MeasureMutation(sibling, &fakeRunner{}); err != nil {
		t.Errorf("a sibling directory that shares the plane name prefix is outside the plane: %v", err)
	}
}

func TestMeasureMutation_ARunnerFailureIsWrapped(t *testing.T) {
	t.Parallel()
	boom := errors.New("gremlins exploded")
	_, err := MeasureMutation(inputFor(t, addFn(addFile, addSrc)), &fakeRunner{err: boom})
	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want it to wrap the runner failure", err)
	}
}

func TestMeasureMutation_NothingToMutateSkipsTheRunnerAndWarnsVacuous(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{}
	res, err := MeasureMutation(inputFor(t), runner)
	if err != nil || len(runner.scratch) != 0 || res.TQ1.N != 0 {
		t.Fatalf("err=%v calls=%d n=%d", err, len(runner.scratch), res.TQ1.N)
	}
	var codes []signalcenter.Code
	for _, e := range res.Events(StageShadow) {
		codes = append(codes, e.Code)
	}
	if !reflect.DeepEqual(codes, []signalcenter.Code{"", CodeVacuousDenominator, "", CodeVacuousDenominator}) {
		t.Errorf("codes = %v, want a reading and a vacuous warning for each of TQ1 and TQ3", codes)
	}
}

func TestMeasureMutation_EightKilledTwoLivedOneNotCoveredScoresPointEight(t *testing.T) {
	t.Parallel()
	report := MutationReport{}
	for i := 0; i < 8; i++ {
		report.Mutants = append(report.Mutants, killedBy("k", "TestAdd"))
	}
	report.Mutants = append(report.Mutants, Mutant{Status: MutantLived}, Mutant{Status: MutantLived}, Mutant{Status: MutantNotCovered}, Mutant{Status: MutantNotViable})
	res, err := MeasureMutation(inputFor(t, addFn(addFile, addSrc)), &fakeRunner{report: report})
	if err != nil {
		t.Fatal(err)
	}
	if res.TQ1.Interval != Wilson(8, 10) || res.TQ1.N != 10 || res.TQ1.Method != MethodWilson95 || res.NotCovered != 1 || res.Killed != 8 || res.Lived != 2 {
		t.Errorf("result = %+v", res)
	}
	var fields map[string]string
	for _, e := range res.Events(StageShadow) {
		if e.Kind == signalcenter.KindQualityReading && e.Fields["index"] == "TQ1" {
			fields = e.Fields
		}
	}
	for k, v := range map[string]string{"value": "0.800", "n": "10", "not_covered": "1", "lo": "0.490", "hi": "0.943", "window": "cycle-9", "source": "fake", "stage": "shadow"} {
		if fields[k] != v {
			t.Errorf("fields[%s] = %q, want %q", k, fields[k], v)
		}
	}
}

func TestMeasureMutation_TimedOutCountsAsKilledAndLivedOnlyBreachesAtReport(t *testing.T) {
	t.Parallel()
	timedOut := MutationReport{Mutants: []Mutant{{Status: MutantTimedOut}, {Status: MutantLived}}}
	res, _ := MeasureMutation(inputFor(t, addFn(addFile, addSrc)), &fakeRunner{report: timedOut})
	if res.Killed != 1 || res.TQ1.N != 2 {
		t.Errorf("killed=%d n=%d, want 1 and 2", res.Killed, res.TQ1.N)
	}
	var lived MutationReport
	for i := 0; i < 30; i++ {
		lived.Mutants = append(lived.Mutants, Mutant{Status: MutantLived})
	}
	res, _ = MeasureMutation(inputFor(t, addFn(addFile, addSrc)), &fakeRunner{report: lived})
	breaches := func(stage Stage) int {
		n := 0
		for _, e := range res.Events(stage) {
			if e.Kind == signalcenter.KindQualityBreach {
				n++
			}
		}
		return n
	}
	if breaches(StageShadow) != 0 || breaches(StageReport) != 1 {
		t.Errorf("breaches shadow=%d report=%d, want 0 and 1", breaches(StageShadow), breaches(StageReport))
	}
}

func TestMeasureMutation_TQ3CountsATestWithZeroUniqueKills(t *testing.T) {
	t.Parallel()
	report := MutationReport{
		Mutants:     []Mutant{killedBy("m1", "TestA"), killedBy("m2", "TestA", "TestB", "TestD"), killedBy("m3", "TestB"), {ID: "m4", Status: MutantLived, KilledBy: []string{"TestZ"}}},
		TestSeconds: map[string]float64{"TestA": 2, "TestB": 1, "TestC": 1, "TestD": 1},
	}
	res, _ := MeasureMutation(inputFor(t, addFn(addFile, addSrc)), &fakeRunner{report: report})
	if !reflect.DeepEqual(res.ZeroUniqueTests, []string{"TestC", "TestD"}) {
		t.Errorf("ZeroUniqueTests = %v", res.ZeroUniqueTests)
	}
	if res.TQ3.Index != "TQ3" || res.TQ3.Method != MethodCensus || res.TQ3.N != 4 || res.TQ3.Interval.Value != 0.4 {
		t.Errorf("TQ3 = %+v, want census over 4 tests at 0.4", res.TQ3)
	}
}

func TestMeasureMutation_TQ3WithNoMeasuredTimeIsEmptyNotInfinite(t *testing.T) {
	t.Parallel()
	report := MutationReport{Mutants: []Mutant{killedBy("m1", "TestA")}}
	res, _ := MeasureMutation(inputFor(t, addFn(addFile, addSrc)), &fakeRunner{report: report})
	if res.TQ3.N != 0 || res.TQ3.Interval.Value != 0 {
		t.Errorf("TQ3 = %+v, want n 0 and value 0 when no test time is measured", res.TQ3)
	}
}

func TestMutantStatus_TheFiveWireNames(t *testing.T) {
	t.Parallel()
	got := []MutantStatus{MutantKilled, MutantLived, MutantNotCovered, MutantTimedOut, MutantNotViable}
	want := []string{"killed", "lived", "not_covered", "timed_out", "not_viable"}
	for i := range got {
		if string(got[i]) != want[i] {
			t.Errorf("status %d = %q, want %q", i, got[i], want[i])
		}
	}
	var _ MutationRunner = (*fakeRunner)(nil)
}
