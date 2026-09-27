package ship

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type aloneRecorder struct {
	calls   int
	got     []packFailure
	outcome packOutcome
	cleared []string
}

func (r *aloneRecorder) run(failures []packFailure) packOutcome {
	r.calls++
	r.got = failures
	return r.outcome
}

func classifiedWithAlone(t *testing.T, first packOutcome, alone *aloneRecorder) (error, string) {
	t.Helper()
	var out bytes.Buffer
	var rerun *aloneRerun
	if alone != nil {
		rerun = &aloneRerun{run: alone.run, cleared: func(names []string) { alone.cleared = names }}
	}
	err := runClassifiedPackRetrying(context.Background(), &out, t.TempDir(), "importer backstop", true, func() packOutcome { return first }, rerun)
	return err, out.String()
}

func TestClassifiedPack_ANamedRedGreenAloneIsFlakeEvidenceNotTheLanesRED(t *testing.T) {
	alone := &aloneRecorder{outcome: greenPack()}
	err, out := classifiedWithAlone(t, redPack("p/channel.TestChannel_EndToEnd"), alone)
	if err != nil {
		t.Fatalf("a named red that is green alone was still the lane's RED: %v", err)
	}
	if alone.calls != 1 || len(alone.got) != 1 || alone.got[0] != (packFailure{Package: "p/channel", Test: "TestChannel_EndToEnd"}) {
		t.Fatalf("alone re-run calls=%d got=%+v", alone.calls, alone.got)
	}
	if !strings.Contains(out, "SHIP_BACKSTOP_FLAKE") || !strings.Contains(out, "p/channel.TestChannel_EndToEnd") {
		t.Fatalf("the flake evidence is not logged loudly:\n%s", out)
	}
	if strings.Join(alone.cleared, ",") != "p/channel.TestChannel_EndToEnd" {
		t.Fatalf("the listener hears the cleared names: %v", alone.cleared)
	}
}

func TestBackstopFlakeSignal_IsAShipWarningWithTheCode(t *testing.T) {
	center := signalcenter.New()
	backstopFlakeSignal(center, 1718)([]string{"p/channel.TestChannel_EndToEnd"})
	events := center.Recent()
	if len(events) != 1 {
		t.Fatalf("events = %+v", events)
	}
	e := events[0]
	if e.Code != CodeBackstopFlake || e.Kind != signalcenter.KindShipWarning || e.Module != signalcenter.ModuleShip || e.Cycle != 1718 || !strings.Contains(e.Reason, "p/channel.TestChannel_EndToEnd") {
		t.Fatalf("event = %+v", e)
	}
	backstopFlakeSignal(nil, 1718)([]string{"anything"})
}

func TestClassifiedPack_ANamedRedStillRedAloneIsTheLanesRED(t *testing.T) {
	alone := &aloneRecorder{outcome: redPack("p/channel.TestChannel_EndToEnd")}
	err, out := classifiedWithAlone(t, redPack("p/channel.TestChannel_EndToEnd", "p/other.TestFlaky"), alone)
	if err == nil || !strings.Contains(err.Error(), "failing: p/channel.TestChannel_EndToEnd") || strings.Contains(err.Error(), "TestFlaky") {
		t.Fatalf("the lane's RED names what is still red alone, not the first run's whole list: err=%v", err)
	}
	if strings.Contains(out, "SHIP_BACKSTOP_FLAKE") {
		t.Fatalf("a still-red test was called a flake:\n%s", out)
	}
}

func TestClassifiedPack_AnAmbiguousAloneRunKeepsTheNamedRed(t *testing.T) {
	alone := &aloneRecorder{outcome: ambiguousPack()}
	err, _ := classifiedWithAlone(t, redPack("p/channel.TestChannel_EndToEnd"), alone)
	if err == nil || !strings.Contains(err.Error(), "failing: p/channel.TestChannel_EndToEnd") {
		t.Fatalf("an alone run that died must leave the named red standing: err=%v", err)
	}
}

func TestClassifiedPack_ManyNamedRedsAreNotRerunAlone(t *testing.T) {
	alone := &aloneRecorder{outcome: greenPack()}
	err, _ := classifiedWithAlone(t, redPack("a.TestA", "b.TestB", "c.TestC", "d.TestD"), alone)
	if err == nil || alone.calls != 0 {
		t.Fatalf("four named reds are a broken contract, not a flake: err=%v aloneCalls=%d", err, alone.calls)
	}
}

func TestClassifiedPack_ABuildFailureIsNeverRerunAlone(t *testing.T) {
	alone := &aloneRecorder{outcome: greenPack()}
	err, _ := classifiedWithAlone(t, redPack("p.TestA", "q [build failed]"), alone)
	if err == nil || alone.calls != 0 {
		t.Fatalf("a build failure cannot be a flake: err=%v aloneCalls=%d", err, alone.calls)
	}
}

func TestClassifiedPack_WithoutAnAloneRunnerANamedRedIsTheLanesRED(t *testing.T) {
	err, _ := classifiedWithAlone(t, redPack("internal/phasespec.TestSpecParity"), nil)
	if err == nil || !strings.Contains(err.Error(), "failing: internal/phasespec.TestSpecParity") {
		t.Fatalf("a pack of the ship's own change has no alone re-run: err=%v", err)
	}
}

func TestPackagesOf_GroupsFailuresByPackageInOrder(t *testing.T) {
	failures := []packFailure{{Package: "z", Test: "T1"}, {Package: "a", Test: "T2"}, {Package: "z", Test: "T3"}}
	if got := packagesOf(failures); strings.Join(got, ",") != "a,z" {
		t.Fatalf("packages = %v", got)
	}
}

func swapGoTestJSON(t *testing.T, byPackage map[string]packOutcome) *[]string {
	t.Helper()
	prev := goTestJSONFn
	var ran []string
	goTestJSONFn = func(_ context.Context, _ string, _ io.Writer, args []string) packOutcome {
		pkg := args[len(args)-1]
		ran = append(ran, pkg)
		return byPackage[pkg]
	}
	t.Cleanup(func() { goTestJSONFn = prev })
	return &ran
}

func TestRunRepoContractTestsAlone_KeepsTheUnprovenAndTheStillRed(t *testing.T) {
	ran := swapGoTestJSON(t, map[string]packOutcome{
		"a": greenPack(),
		"b": ambiguousPack(),
		"c": redPack("c.TestC2"),
	})
	var out bytes.Buffer
	got := runRepoContractPackagesAlone(context.Background(), t.TempDir(), &out, []packFailure{
		{Package: "c", Test: "TestC1"}, {Package: "c", Test: "TestC2"}, {Package: "a", Test: "TestA"}, {Package: "b", Test: "TestB"},
	})
	if strings.Join(*ran, ",") != "a,b,c" {
		t.Fatalf("one re-run per package, in order: %v", *ran)
	}
	if names := strings.Join(got.failedNames(), ","); names != "b.TestB,c.TestC2" || !got.realRed() {
		t.Fatalf("the unproven package keeps its reds and the red one names what is still red: %s (err=%v)", names, got.err)
	}
	if strings.Contains(got.err.Error(), "\n") {
		t.Fatalf("the merged error is one line for the ship error: %q", got.err.Error())
	}
	for _, want := range []string{
		"alone re-run: go test -json -count=1 -timeout " + repoContractTestTimeout + " c\n",
		"alone re-run: a green by itself\n",
		"alone re-run: b named nothing (signal: killed); its first-run reds stand\n",
		"alone re-run: c still red by itself: c.TestC2\n",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("each package's argv and outcome are logged; missing %q:\n%s", want, out.String())
		}
	}
}

func TestRunRepoContractTestsAlone_EveryPackageGreenIsGreen(t *testing.T) {
	swapGoTestJSON(t, map[string]packOutcome{"a": greenPack(), "b": greenPack()})
	got := runRepoContractPackagesAlone(context.Background(), t.TempDir(), io.Discard, []packFailure{{Package: "a", Test: "TestA"}, {Package: "b", Test: "TestB"}})
	if !got.green() || len(got.failures) != 0 {
		t.Fatalf("two packages green alone are green: %+v", got)
	}
}

func TestPackOutcome_AllNamedTests(t *testing.T) {
	if !redPack("p.TestA", "q.TestB").allNamedTests() {
		t.Fatal("two named tests are all named")
	}
	if redPack("p.TestA", "q [build failed]").allNamedTests() {
		t.Fatal("a build failure is not a named test")
	}
	if (packOutcome{err: errors.New("signal: killed")}).allNamedTests() {
		t.Fatal("an ambiguous exit names nothing")
	}
}
