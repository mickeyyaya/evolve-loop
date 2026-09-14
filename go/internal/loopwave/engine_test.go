package loopwave

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// harness is one engine over a temp project root, reporting into a recording
// Center whose WARN+ events also render on console (the root sink topology's
// shape), with the kept stderr lines on stderr.
type harness struct {
	root, evolveDir string
	stderr, console *bytes.Buffer
	events          *[]signalcenter.Event
	center          *signalcenter.Center
	ports           Ports
	e               *Engine
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	root := t.TempDir()
	h := &harness{root: root, evolveDir: filepath.Join(root, ".evolve"), stderr: &bytes.Buffer{}, console: &bytes.Buffer{}, events: &[]signalcenter.Event{}}
	if err := os.MkdirAll(filepath.Join(h.evolveDir, "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	h.center = signalcenter.New()
	h.center.Subscribe(func(e signalcenter.Event) { *h.events = append(*h.events, e) })
	h.center.Subscribe(signalcenter.Filter(signalcenter.StderrSink(h.console), signalcenter.SeverityWarn))
	h.ports = Ports{
		LastCycle: func(context.Context) (int, error) { return 0, nil },
		Workspace: func(n int) string { return filepath.Join(h.evolveDir, "runs", fmt.Sprintf("cycle-%d", n)) },
		Protected: func(path string) bool { return strings.Contains(path, "protected") },
		Shrink:    fleet.QuotaAwareCount,
	}
	h.e = New(Roots{ProjectRoot: root, EvolveDir: h.evolveDir}, h.ports, h.stderr, WithSignals(func() *signalcenter.Center { return h.center }))
	return h
}

func (h *harness) codes() []string {
	var out []string
	for _, e := range *h.events {
		out = append(out, string(e.Code))
	}
	return out
}

func (h *harness) only(t *testing.T, code signalcenter.Code) signalcenter.Event {
	t.Helper()
	if len(*h.events) != 1 || (*h.events)[0].Code != code {
		t.Fatalf("exactly one %s event, got %v", code, h.codes())
	}
	return (*h.events)[0]
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func golden(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// recorder records every Run and answers one result per spec; failing lists
// the spec indexes that fail.
type recorder struct {
	calls   [][]fleet.CycleSpec
	failing map[int]bool
}

func (r *recorder) Run(_ context.Context, specs []fleet.CycleSpec) []fleet.Result {
	r.calls = append(r.calls, specs)
	out := make([]fleet.Result, len(specs))
	for i := range specs {
		out[i] = fleet.Result{Index: i}
		if r.failing[i] {
			out[i].ExitCode = 4
		}
	}
	return out
}

func floorsPlan(context.Context, int) ([]byte, []string, error) {
	return []byte(`{"committed_floors":["core"]}`), nil, nil
}

func twoCardPlan(context.Context, int) ([]byte, []string, error) {
	return []byte(`{"top_n":[{"id":"a","files":["a.go"]},{"id":"b","files":["b.go"]}]}`), nil, nil
}

func request(wave int, fc policy.FleetConfig, preflight func() error, plan PlanFn, l Launcher) DispatchRequest {
	return DispatchRequest{Config: fc, Wave: wave, Preflight: preflight, Plan: plan, Launcher: l}
}

func waveFC(count int) policy.FleetConfig {
	return policy.FleetConfig{Count: count, PlanSource: "triage", Scheduling: "wave"}
}

func pass() error { return nil }

// --- 11. codes ---

func TestCodes_AreRegisteredUnderLoopWithDocs(t *testing.T) {
	for _, c := range []signalcenter.Code{CodeMinWidthRepair, CodeWaveDispatchFailed, CodeWaveEmptyPlan, CodeWaveAllLanesStale} {
		if m, ok := signalcenter.IsRegistered(c); !ok || m != signalcenter.ModuleLoop {
			t.Errorf("%s must be registered under loop (got %q, %v)", c, m, ok)
		}
	}
	docs := signalcenter.RegisteredCodes()[signalcenter.ModuleLoop]
	for _, d := range docs {
		if d.Code == CodeMinWidthRepair && d.Doc != "the fleet shrank below its committed width and one isolated lane was dispatched instead (min-width repair)" {
			t.Errorf("LOOP_MIN_WIDTH_REPAIR keeps its original doc verbatim, got %q", d.Doc)
		}
		if strings.HasPrefix(string(d.Code), "LOOP_WAVE_") && d.Doc == "" {
			t.Errorf("%s has no doc", d.Code)
		}
	}
	if CodeMinWidthRepair != "LOOP_MIN_WIDTH_REPAIR" || CodeWaveDispatchFailed != "LOOP_WAVE_DISPATCH_FAILED" || CodeWaveEmptyPlan != "LOOP_WAVE_EMPTY_PLAN" || CodeWaveAllLanesStale != "LOOP_WAVE_ALL_LANES_STALE" {
		t.Error("the code spellings are the registry's")
	}
}

// --- 12. the gate ---

func TestShouldRunWave_GateTable(t *testing.T) {
	cases := []struct {
		fc   policy.FleetConfig
		want bool
	}{
		{policy.FleetConfig{Count: 2, PlanSource: "triage"}, true},
		{policy.FleetConfig{Count: 2, PlanSource: "triage", Scheduling: "wave"}, true},
		{policy.FleetConfig{Count: 2, PlanSource: "triage", Scheduling: "pool"}, false},
		{policy.FleetConfig{Count: 2, PlanSource: "manual"}, false},
		{policy.FleetConfig{Count: 1, PlanSource: "triage"}, false},
		{policy.FleetConfig{Count: 0, PlanSource: "triage"}, false},
		{policy.FleetConfig{}, false},
	}
	for _, c := range cases {
		if got := ShouldRunWave(c.fc); got != c.want {
			t.Errorf("ShouldRunWave(%+v) = %v, want %v", c.fc, got, c.want)
		}
	}
}

// --- 13. the typed step error ---

func TestStepError_RendersTheThreeLiteralsAndUnwraps(t *testing.T) {
	cause := errors.New("boom")
	for step, want := range map[Step]string{
		StepPreflight: fmt.Errorf("wave %d: control-plane preflight: %w", 3, cause).Error(),
		StepPlan:      fmt.Errorf("wave %d: triage plan: %w", 3, cause).Error(),
		StepAdapt:     fmt.Errorf("wave %d: adapt triage plan: %w", 3, cause).Error(),
	} {
		e := &StepError{Wave: 3, Step: step, Err: cause}
		if e.Error() != want {
			t.Errorf("%s renders %q, want %q", step, e.Error(), want)
		}
		if !errors.Is(e, cause) || e.Unwrap() != cause {
			t.Errorf("%s unwraps to the cause", step)
		}
	}
	lines := golden(t, "wave_step_errors.golden.txt")
	for _, row := range strings.Split(strings.TrimSpace(lines), "\n") {
		step, text, _ := strings.Cut(row, "\t")
		var cause error
		switch step {
		case "preflight":
			cause = errors.New("dirty control plane")
		case "plan":
			cause = errors.New("plan exploded")
		default:
			_, _, err := fleet.PlanFromTriage([]byte("{not json"), nil, 2, nil)
			cause = err
		}
		if got := (&StepError{Wave: 3, Step: Step(step), Err: cause}).Error(); got != text {
			t.Errorf("golden %s: %q != %q", step, got, text)
		}
	}
}

// --- 14-16. Dispatch ---

func TestDispatch_GateOffTouchesNothing(t *testing.T) {
	h := newHarness(t)
	called := false
	l := &recorder{}
	out, err := h.e.Dispatch(context.Background(), request(1, waveFC(1), func() error { called = true; return nil },
		func(context.Context, int) ([]byte, []string, error) { called = true; return nil, nil, nil }, l))
	if err != nil || out.Ran || out.Specs != nil || out.Results != nil || called || len(l.calls) != 0 || len(*h.events) != 0 {
		t.Errorf("Count 1 never runs preflight, plan or launcher and emits nothing: %+v %v called=%v events=%v", out, err, called, h.codes())
	}
}

func TestDispatch_EachStepFailureEmitsOneCodedWaveWarnAndNoLaunch(t *testing.T) {
	refusal := errors.New("dirty control plane")
	planErr := errors.New("plan exploded")
	cases := []struct {
		step      Step
		preflight func() error
		plan      PlanFn
		planRuns  int
	}{
		{StepPreflight, func() error { return refusal }, floorsPlan, 0},
		{StepPlan, pass, func(context.Context, int) ([]byte, []string, error) { return nil, nil, planErr }, 1},
		{StepAdapt, pass, func(context.Context, int) ([]byte, []string, error) { return []byte("{not json"), nil, nil }, 1},
	}
	for _, c := range cases {
		h := newHarness(t)
		l := &recorder{}
		planRuns := 0
		plan := func(ctx context.Context, w int) ([]byte, []string, error) { planRuns++; return c.plan(ctx, w) }
		out, err := h.e.Dispatch(context.Background(), request(3, waveFC(2), c.preflight, plan, l))
		var se *StepError
		if out.Ran || err == nil || !errors.As(err, &se) || se.Step != c.step || se.Wave != 3 || len(l.calls) != 0 || planRuns != c.planRuns {
			t.Fatalf("%s: ran=%v err=%v launches=%d planRuns=%d", c.step, out.Ran, err, len(l.calls), planRuns)
		}
		ev := h.only(t, CodeWaveDispatchFailed)
		want := "wave 3 dispatch failed, falling back to sequential: " + err.Error()
		if ev.Kind != signalcenter.KindLoopWave || ev.Severity != signalcenter.SeverityWarn || ev.Origin != "Engine.Dispatch" || ev.Module != signalcenter.ModuleLoop ||
			ev.Reason != want || ev.Fields["wave"] != "3" || ev.Fields["path"] != "wave" || ev.Fields["step"] != string(c.step) || ev.Fields["error"] != se.Err.Error() {
			t.Errorf("%s: the event carries kind/severity/origin/reason/fields: %+v (want reason %q)", c.step, ev, want)
		}
		if !strings.Contains(h.console.String(), "LOOP_WAVE_DISPATCH_FAILED") || !strings.Contains(h.console.String(), want) {
			t.Errorf("%s: the console renders the WARN with the old sentence: %q", c.step, h.console.String())
		}
	}
}

func TestDispatch_EmptyPlanIsRanFalseWithNoSignalAndNoLaunch(t *testing.T) {
	h := newHarness(t)
	l := &recorder{}
	out, err := h.e.Dispatch(context.Background(), request(2, waveFC(2), pass,
		func(context.Context, int) ([]byte, []string, error) {
			return []byte(`{"committed_floors":[]}`), nil, nil
		}, l))
	if err != nil || out.Ran || len(l.calls) != 0 || len(*h.events) != 0 {
		t.Errorf("an empty plan is ran=false, silent, unlaunched: %+v %v %v", out, err, h.codes())
	}
	// A clean wave: two lanes, the caller's ctx, no event.
	ctx := context.WithValue(context.Background(), ctxKey{}, "probe")
	seen := ""
	plan := func(c context.Context, _ int) ([]byte, []string, error) {
		seen, _ = c.Value(ctxKey{}).(string)
		return twoCardPlan(c, 0)
	}
	out, err = h.e.Dispatch(ctx, request(2, waveFC(2), pass, plan, l))
	if err != nil || !out.Ran || len(out.Specs) != 2 || len(out.Results) != 2 || len(l.calls) != 1 || seen != "probe" || len(*h.events) != 0 {
		t.Errorf("a clean wave launches every disjoint lane with the caller's ctx and emits nothing: %+v %v seen=%q events=%v", out, err, seen, h.codes())
	}
}

type ctxKey struct{}

// --- 17-18. ForceOneLane and the shared body ---

func TestForceOneLane_CapsAtOneLaneUngatedAndSilent(t *testing.T) {
	h := newHarness(t)
	l := &recorder{}
	out, err := h.e.ForceOneLane(context.Background(), request(1, waveFC(1), pass, twoCardPlan, l))
	if err != nil || !out.Ran || len(out.Specs) != 1 || len(l.calls) != 1 || len(l.calls[0]) != 1 {
		t.Errorf("the repair dispatches ONE lane even under a Count-1 config: %+v %v %d", out, err, len(l.calls))
	}
	refusal := errors.New("dirty control plane")
	out, err = h.e.ForceOneLane(context.Background(), request(1, waveFC(1), func() error { return refusal }, twoCardPlan, l))
	var se *StepError
	if out.Ran || !errors.As(err, &se) || se.Step != StepPreflight || len(l.calls) != 1 || len(*h.events) != 0 {
		t.Errorf("a step error returns the StepError and emits nothing (the repair reports): %+v %v %v", out, err, h.codes())
	}
	out, err = h.e.ForceOneLane(context.Background(), request(1, waveFC(1), pass, floorsPlan, l))
	if err != nil || !out.Ran || len(l.calls) != 2 {
		t.Errorf("a floors decision yields one lane: %+v %v", out, err)
	}
	out, err = h.e.ForceOneLane(context.Background(), request(1, waveFC(2), pass, twoCardPlan, l))
	if err != nil || !out.Ran || len(out.Specs) != 1 || len(l.calls) != 3 || len(l.calls[2]) != 1 {
		t.Errorf("the cap is ONE lane even when the config wants two: %+v %v", out, err)
	}
}

func TestDispatchAndForceOneLane_ShareOneBody(t *testing.T) {
	h := newHarness(t)
	l1, l2 := &recorder{}, &recorder{}
	// One-card plan: the fan-out and the repair produce identical specs.
	one := func(context.Context, int) ([]byte, []string, error) {
		return []byte(`{"top_n":[{"id":"a","files":["a.go"]}]}`), nil, nil
	}
	a, err1 := h.e.Dispatch(context.Background(), request(1, waveFC(2), pass, one, l1))
	b, err2 := h.e.ForceOneLane(context.Background(), request(1, waveFC(2), pass, one, l2))
	if err1 != nil || err2 != nil || !a.Ran || !b.Ran || fmt.Sprint(a.Specs) != fmt.Sprint(b.Specs) || fmt.Sprint(a.Results) != fmt.Sprint(b.Results) {
		t.Errorf("identical fakes give identical outcomes: %+v %v / %+v %v", a, err1, b, err2)
	}
	src, err := os.ReadFile("dispatch.go")
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(src), "fleet.PlanFromTriage("); n != 1 {
		t.Errorf("exactly ONE fleet.PlanFromTriage( call in the package (the fold), got %d", n)
	}
	entries, _ := os.ReadDir(".")
	for _, e := range entries {
		if e.Name() == "dispatch.go" || strings.HasSuffix(e.Name(), "_test.go") || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		other, _ := os.ReadFile(e.Name())
		if strings.Contains(string(other), "fleet.PlanFromTriage(") {
			t.Errorf("%s re-introduces a second PlanFromTriage call", e.Name())
		}
	}
}

// --- 19. RepairMinWidth ---

func TestRepairMinWidth_FourBranchesAndTheirSignals(t *testing.T) {
	t.Run("guard not met", func(t *testing.T) {
		h := newHarness(t)
		l := &recorder{}
		touched := false
		handled := h.e.RepairMinWidth(context.Background(), policy.FleetConfig{Count: 1}, policy.FleetConfig{Count: 1},
			request(4, waveFC(1), func() error { touched = true; return nil }, func(context.Context, int) ([]byte, []string, error) { touched = true; return nil, nil, nil }, l))
		ev := h.only(t, CodeWaveEmptyPlan)
		if handled || touched || len(l.calls) != 0 || ev.Reason != "wave 4 planned zero lanes (empty triage plan), falling back to sequential" ||
			ev.Fields["cause"] != "empty_triage_plan" || ev.Fields["desired"] != "1" || ev.Fields["realized"] != "1" || ev.Fields["wave"] != "4" || ev.Origin != "Engine.RepairMinWidth" {
			t.Errorf("the guard WARNs empty_triage_plan and touches nothing: handled=%v touched=%v %+v", handled, touched, ev)
		}
		if !strings.Contains(h.console.String(), "empty triage plan") {
			t.Errorf("the console keeps the pinned words: %q", h.console.String())
		}
	})
	t.Run("dispatched", func(t *testing.T) {
		h := newHarness(t)
		l := &recorder{}
		handled := h.e.RepairMinWidth(context.Background(), policy.FleetConfig{Count: 3}, policy.FleetConfig{Count: 1}, request(4, waveFC(1), pass, floorsPlan, l))
		ev := h.only(t, CodeMinWidthRepair)
		if !handled || len(l.calls) != 1 || len(l.calls[0]) != 1 || ev.Reason != "wave 4: min-width repair dispatched 1/1 isolated lane (fleet.count=3 shrank to 1)" ||
			ev.Fields["desired"] != "3" || ev.Fields["realized"] != "1" || ev.Fields["wave"] != "4" || ev.Origin != "Engine.RepairMinWidth" || ev.Severity != signalcenter.SeverityWarn {
			t.Errorf("a dispatched lane is the LOOP_MIN_WIDTH_REPAIR WARN: handled=%v %+v", handled, ev)
		}
		if !strings.Contains(h.console.String(), "min-width repair dispatched") {
			t.Errorf("the console keeps the pinned words: %q", h.console.String())
		}
	})
	t.Run("failed lanes counted", func(t *testing.T) {
		h := newHarness(t)
		l := &recorder{failing: map[int]bool{0: true}}
		h.e.RepairMinWidth(context.Background(), policy.FleetConfig{Count: 2}, policy.FleetConfig{Count: 0}, request(1, waveFC(1), pass, floorsPlan, l))
		if ev := h.only(t, CodeMinWidthRepair); ev.Reason != "wave 1: min-width repair dispatched 0/1 isolated lane (fleet.count=2 shrank to 0)" {
			t.Errorf("the reason counts ok lanes: %q", ev.Reason)
		}
	})
	t.Run("empty backlog", func(t *testing.T) {
		h := newHarness(t)
		l := &recorder{}
		handled := h.e.RepairMinWidth(context.Background(), policy.FleetConfig{Count: 2}, policy.FleetConfig{Count: 1},
			request(4, waveFC(1), pass, func(context.Context, int) ([]byte, []string, error) {
				return []byte(`{"committed_floors":[]}`), nil, nil
			}, l))
		ev := h.only(t, CodeWaveEmptyPlan)
		if handled || len(l.calls) != 0 || ev.Reason != "wave 4 planned zero lanes (empty backlog), falling back to sequential" || ev.Fields["cause"] != "empty_backlog" || ev.Fields["desired"] != "2" || ev.Fields["realized"] != "1" {
			t.Errorf("an empty backlog WARNs empty_backlog: handled=%v %+v", handled, ev)
		}
		if !strings.Contains(h.console.String(), "empty backlog") {
			t.Errorf("the console keeps the pinned words: %q", h.console.String())
		}
	})
	t.Run("step error", func(t *testing.T) {
		h := newHarness(t)
		l := &recorder{}
		handled := h.e.RepairMinWidth(context.Background(), policy.FleetConfig{Count: 2}, policy.FleetConfig{Count: 1},
			request(4, waveFC(1), func() error { return errors.New("dirty control plane") }, floorsPlan, l))
		ev := h.only(t, CodeWaveDispatchFailed)
		want := "wave 4 min-width repair failed, falling back to sequential: wave 4: control-plane preflight: dirty control plane"
		if handled || len(l.calls) != 0 || ev.Reason != want || ev.Fields["path"] != "repair" || ev.Fields["step"] != "preflight" || ev.Fields["error"] != "dirty control plane" || ev.Fields["wave"] != "4" {
			t.Errorf("a step error is ONE LOOP_WAVE_DISPATCH_FAILED path=repair: handled=%v %+v", handled, ev)
		}
		if out := h.console.String(); !strings.Contains(out, "min-width repair failed") || !strings.Contains(out, "dirty control plane") {
			t.Errorf("the console keeps the pinned words and the refusal: %q", out)
		}
	})
}

// --- 20. FailedLanes ---

func TestFailedLanes_CountsErrOrNonZeroExit(t *testing.T) {
	results := []fleet.Result{{Err: errors.New("x")}, {ExitCode: 4}, {}, {Err: errors.New("y"), ExitCode: 1}}
	if got := FailedLanes(results); got != 3 {
		t.Errorf("FailedLanes = %d, want 3", got)
	}
	if FailedLanes(nil) != 0 {
		t.Error("no results, no failures")
	}
}

// --- 21-22. the fleet config loaders ---

func TestLoadFleetConfig_DefaultsOnAnyError(t *testing.T) {
	dir := t.TempDir()
	if got := LoadFleetConfig(dir); got.Count != 1 || got.PlanSource != "triage" {
		t.Errorf("absent policy defaults: %+v", got)
	}
	if err := os.WriteFile(paths.PolicyPath(dir), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := LoadFleetConfig(dir); got.Count != 1 {
		t.Errorf("malformed policy defaults to Count 1 (never holds): %+v", got)
	}
	if err := os.WriteFile(paths.PolicyPath(dir), []byte(`{"fleet":{"count":4,"min_lanes":2}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := LoadFleetConfig(dir); got.Count != 4 || got.MinLanes != 2 {
		t.Errorf("a fleet block resolves: %+v", got)
	}
}

// Review fold R3 — the roots are a Parameter Object with an invariant: both
// halves populated, or neither (the rootless engine of the pure dispatch
// facades). RootsOf derives EvolveDir from the project root the production
// way (paths.EvolveDirOf), for the host facades that carry a root alone.
func TestRootsOf_DerivesEvolveDirTheProductionWay(t *testing.T) {
	if got := RootsOf("/x"); got != (Roots{ProjectRoot: "/x", EvolveDir: paths.EvolveDirOf("/x")}) || got.EvolveDir != filepath.Join("/x", ".evolve") {
		t.Errorf("RootsOf = %+v", got)
	}
	if got := RootsOf(""); got != (Roots{}) {
		t.Errorf("no project root is the rootless zero Roots, never a CWD-relative half: %+v", got)
	}
}

func TestReloadFleetConfig_HoldsWithTheVerbatimWarnAndReportsOnlyChanges(t *testing.T) {
	prev := policy.FleetConfig{Count: 3, MinLanes: 2, PlanSource: "triage", Scheduling: "wave"}
	sections := map[string]string{}
	for _, s := range strings.Split(golden(t, "stderr_wave.golden.txt"), "== ") {
		name, body, _ := strings.Cut(s, "\n")
		sections[name] = body
	}
	run := func(t *testing.T, name, policyJSON string, want func(policy.FleetConfig) bool) {
		t.Helper()
		h := newHarness(t)
		if err := os.WriteFile(paths.PolicyPath(h.evolveDir), []byte(policyJSON), 0o644); err != nil {
			t.Fatal(err)
		}
		got := ReloadFleetConfig(h.evolveDir, prev, h.stderr)
		if !want(got) {
			t.Errorf("%s: resolved %+v", name, got)
		}
		if out := strings.ReplaceAll(h.stderr.String(), h.evolveDir, "{EVOLVE_DIR}"); out != sections[name] {
			t.Errorf("%s: stderr %q, golden %q", name, out, sections[name])
		}
		if len(*h.events) != 0 {
			t.Errorf("%s: the reload emits no event: %v", name, h.codes())
		}
	}
	run(t, "reload_changed", `{"fleet":{"count":5,"min_lanes":2,"plan_source":"weird"}}`, func(fc policy.FleetConfig) bool { return fc.Count == 5 && fc.PlanSource == "manual" })
	run(t, "reload_unchanged", `{"fleet":{"count":3,"min_lanes":2,"plan_source":"triage"}}`, func(fc policy.FleetConfig) bool { return fc.Count == 3 })
	run(t, "reload_unreadable", `{not json`, func(fc policy.FleetConfig) bool {
		return fc.Count == prev.Count && fc.MinLanes == prev.MinLanes && fc.PlanSource == prev.PlanSource
	})
	run(t, "reload_concurrency_only", `{"fleet":{"count":3,"min_lanes":2,"concurrency":9,"plan_source":"triage"}}`, func(fc policy.FleetConfig) bool { return fc.Concurrency == 9 })
}

// --- 26. the routed resolver ---

func TestRoutedResolver_RefusalPrintsTheVerbatimLineOnceAndPassesThrough(t *testing.T) {
	h := newHarness(t)
	writeJSON(t, filepath.Join(h.evolveDir, "inbox", "console.json"), map[string]any{"id": "console-item", "weight": 0.9, "files": []string{"go/protected/x.go"}})
	writeJSON(t, filepath.Join(h.evolveDir, "inbox", "lane.json"), map[string]any{"id": "lane-item", "weight": 0.8, "files": []string{"go/pkg/x.go"}})
	routed := h.e.RoutedResolver()
	if ok, reason := routed("console-item"); !ok || reason == "" {
		t.Errorf("a protected item is routed: %v %q", ok, reason)
	}
	want := "[fleet] WARN: plan-time gate refused console-routed item \"console-item\" (protected fix surface: go/protected/x.go) — operator-owned, worked at a batch boundary via manual ship (ADR-0074)\n"
	if h.stderr.String() != want {
		t.Errorf("the :158 line verbatim: %q", h.stderr.String())
	}
	if ok, _ := routed("lane-item"); ok {
		t.Error("a lane item passes")
	}
	if ok, _ := routed("unknown"); ok {
		t.Error("an unknown id is dispatchable")
	}
	if h.stderr.String() != want || len(*h.events) != 0 {
		t.Errorf("printed once, no event: %q %v", h.stderr.String(), h.codes())
	}
}

// --- 34-35. EmitWave and the Null Object ---

func TestEmitWave_InfoWithoutCodeWarnWithAndStampsWaveOnACopy(t *testing.T) {
	h := newHarness(t)
	fields := map[string]string{"lanes": "3"}
	EmitWave(h.center, 5, "loopBatchCoordinator.dispatchFleetIteration", "", "wave 5: 3/3 lanes ok", fields)
	EmitWave(h.center, 6, "Engine.RepairMinWidth", CodeMinWidthRepair, "repair", nil)
	if _, stamped := fields["wave"]; stamped || len(fields) != 1 {
		t.Errorf("the caller's map is never written: %v", fields)
	}
	if len(*h.events) != 2 {
		t.Fatalf("two events: %v", h.codes())
	}
	info, warn := (*h.events)[0], (*h.events)[1]
	if info.Severity != signalcenter.SeverityInfo || info.Code != "" || info.Fields["wave"] != "5" || info.Fields["lanes"] != "3" || info.Kind != signalcenter.KindLoopWave || info.Cycle != 0 {
		t.Errorf("INFO without a code: %+v", info)
	}
	if warn.Severity != signalcenter.SeverityWarn || warn.Code != CodeMinWidthRepair || warn.Fields["wave"] != "6" {
		t.Errorf("WARN with a code, the wave stamped even without fields: %+v", warn)
	}
	if strings.Contains(h.console.String(), "lanes ok") || !strings.Contains(h.console.String(), "LOOP_MIN_WIDTH_REPAIR") {
		t.Errorf("INFO stays off the console: %q", h.console.String())
	}
	EmitWave(nil, 1, "x", "", "nil center is the Null Object", nil)
}

func TestEngine_NullObjectAndSignalsWired(t *testing.T) {
	h := newHarness(t)
	var opt Option = WithSignals(nil)
	var shrink ShrinkFn = fleet.QuotaAwareCount
	var zero Outcome
	if opt == nil || shrink == nil || zero.Ran {
		t.Fatal("the option, shrink and outcome types are the declared shapes")
	}
	if !h.e.SignalsWired() {
		t.Error("the harness engine reaches a Center")
	}
	var stderr bytes.Buffer
	for _, e := range []*Engine{
		New(Roots{ProjectRoot: h.root, EvolveDir: h.evolveDir}, h.ports, &stderr),
		New(Roots{ProjectRoot: h.root, EvolveDir: h.evolveDir}, h.ports, &stderr, WithSignals(func() *signalcenter.Center { return nil })),
	} {
		if e.SignalsWired() {
			t.Error("no accessor / a nil Center is not wired")
		}
		l := &recorder{}
		if handled := e.RepairMinWidth(context.Background(), policy.FleetConfig{Count: 1}, policy.FleetConfig{Count: 1}, request(1, waveFC(1), pass, floorsPlan, l)); handled {
			t.Error("the guard branch still decides")
		}
		if _, err := e.Dispatch(context.Background(), request(1, waveFC(2), func() error { return errors.New("x") }, floorsPlan, l)); err == nil {
			t.Error("the step error still returns")
		}
		e.Launcher(1, 1, nil).Run(context.Background(), nil)
	}
	if stderr.Len() != 0 {
		t.Errorf("nothing hand-written reaches stderr for the coded conditions: %q", stderr.String())
	}
	// The accessor is read live: a Center installed later is seen.
	var c *signalcenter.Center
	e := New(Roots{}, h.ports, io.Discard, WithSignals(func() *signalcenter.Center { return c }))
	if e.SignalsWired() {
		t.Error("nil before")
	}
	c = signalcenter.New()
	if !e.SignalsWired() {
		t.Error("wired after")
	}
}
