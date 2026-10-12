package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestParseCycleRunFlags_ARemovedBudgetFlagWarnsAndIsIgnored(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer

	f, ok := parseCycleRunFlags([]string{"--budget-usd", "5", "--goal-hash", "abcd1234", "--batch-cap-usd=9"}, &stderr)

	if !ok || f.goalHash != "abcd1234" {
		t.Fatalf("parseCycleRunFlags = (%+v, %v), want the goal hash parsed past the removed flags", f, ok)
	}
	if strings.Count(stderr.String(), "evolve cycle run: WARN: --budget-usd/--budget/--batch-cap-usd are removed") != 1 {
		t.Errorf("stderr = %q, want exactly one removal WARN", stderr.String())
	}
}

func TestRunCycleRun_AnUnknownFlagIsAUsageError(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer

	code := runCycleRun([]string{"--goal-hash", "abcd1234", "--no-such-flag"}, &out, &errb)

	if code != 10 || out.Len() != 0 || !strings.Contains(errb.String(), "flag provided but not defined: -no-such-flag") {
		t.Errorf("code=%d stdout=%q stderr=%q, want 10 and the flag error", code, out.String(), errb.String())
	}
}

func TestRunCycleRun_AnUnresolvableRootWarnsBeforeTheLaneCheck(t *testing.T) {
	prev := cycleRunAbsoluteRoot
	t.Cleanup(func() { cycleRunAbsoluteRoot = prev })
	cycleRunAbsoluteRoot = func(_, p string, warn func(string)) string {
		warn("could not resolve " + p)
		return p
	}
	t.Setenv("EVOLVE_FLEET", "1")
	var out, errb bytes.Buffer

	code := runCycleRun([]string{"--project-root", "rel/root", "--evolve-dir", filepath.Join(t.TempDir(), ".evolve"), "--goal-hash", "abcd1234", "--simulate"}, &out, &errb)

	if !strings.HasPrefix(errb.String(), "evolve cycle run: WARN: could not resolve rel/root\n") {
		t.Errorf("stderr = %q, want the resolve WARN first", errb.String())
	}
	if code != 10 {
		t.Errorf("code = %d, want the fleet lane refusal (10) after the WARN", code)
	}
}

func TestCycleRunErrorExit_AReleaseFailureWarnsAndStillDefers(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "inbox"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer

	code := cycleRunErrorExit(&core.LaneDeferral{Cycle: 7, Cause: errors.New("worktree busy")}, 7, root, filepath.Join(root, ".evolve"), &stderr, nil, nil)

	if code != 5 {
		t.Errorf("code = %d, want the deferral exit 5", code)
	}
	if !strings.Contains(stderr.String(), "evolve cycle run: WARN: could not release cycle 7 claims: ") {
		t.Errorf("stderr = %q, want the release WARN", stderr.String())
	}
	if !strings.Contains(stderr.String(), "worktree busy") {
		t.Errorf("stderr = %q, want the deferral named", stderr.String())
	}
}

func TestCatalogPublisher_NoSinkPublishesNothing(t *testing.T) {
	t.Parallel()
	if got := catalogPublisher(nil, nil); got != nil {
		t.Error("catalogPublisher(nil) must be nil so the orchestrator skips publishing")
	}
}

func TestProbeBuildHandoffFloor_AMalformedPolicyWarnsAndKeepsTheCompiledFloor(t *testing.T) {
	broken := t.TempDir()
	if err := os.MkdirAll(filepath.Join(broken, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, ".evolve", "policy.json"), []byte(`{"workflow":`), 0o644); err != nil {
		t.Fatal(err)
	}
	var floor core.BuildHandoffFloor

	stderr := captureProcessStderr(t, func() { floor = probeBuildHandoffFloor(broken) })

	want := probeBuildHandoffFloor(t.TempDir()).Names()
	if !reflect.DeepEqual(floor.Names(), want) {
		t.Errorf("floor = %v, want the compiled-default floor %v", floor.Names(), want)
	}
	if !strings.Contains(stderr, "[policy] WARN policy: parse ") || !strings.Contains(stderr, "the build handoff floor uses the compiled workflow defaults") {
		t.Errorf("stderr = %q, want the policy WARN", stderr)
	}
}

func TestRunCycleHealth_ArgumentRefusals(t *testing.T) {
	t.Parallel()
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"--bogus"}, "flag provided but not defined: -bogus"},
		{nil, "evolve cycle-health: missing <cycle-N> <workspace>\n"},
		{[]string{"3"}, "evolve cycle-health: missing <cycle-N> <workspace>\n"},
		{[]string{"0", "/ws"}, "evolve cycle-health: invalid cycle \"0\"\n"},
		{[]string{"-1", "/ws"}, "flag provided but not defined: -1"},
		{[]string{"x7", "/ws"}, "evolve cycle-health: invalid cycle \"x7\"\n"},
	}
	for _, tc := range cases {
		var out, errb bytes.Buffer
		code := runCycleHealth(tc.args, nil, &out, &errb)
		if code != 10 || out.Len() != 0 || !strings.Contains(errb.String(), tc.want) {
			t.Errorf("runCycleHealth(%q) = %d stdout=%q stderr=%q, want 10 and %q", tc.args, code, out.String(), errb.String(), tc.want)
		}
	}
}

func TestRunCycleHealth_AnUnwritableWorkspaceIsAnError(t *testing.T) {
	root := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	ws := filepath.Join(root, "ws-file")
	if err := os.WriteFile(ws, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer

	code := runCycleHealth([]string{"4", ws}, nil, &out, &errb)

	if code != 1 || out.Len() != 0 || !strings.HasPrefix(errb.String(), "evolve cycle-health: cyclehealth: write report: ") {
		t.Errorf("code=%d stdout=%q stderr=%q, want 1 and the write error", code, out.String(), errb.String())
	}
}

func TestRunCycleHealth_AHealthyWorkspaceIsOK(t *testing.T) {
	root := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	ws := filepath.Join(root, ".evolve", "runs", "cycle-4")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range phasecontract.RequiredArtifacts() {
		if err := os.WriteFile(filepath.Join(ws, name), []byte("# report\n\n"+strings.Repeat("The phase recorded substantive findings for this cycle. ", 20)+"\nVerdict: PASS\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var ledgerLines []string
	for i, role := range phasecontract.RequiredRoles() {
		ledgerLines = append(ledgerLines, fmt.Sprintf(`{"cycle":4,"role":%q,"timestamp":%d}`, role, 1790000000+i))
	}
	if err := os.WriteFile(filepath.Join(ws, "ledger.jsonl"), []byte(strings.Join(ledgerLines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer

	code := runCycleHealth([]string{"4", ws}, nil, &out, &errb)

	if code != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q, want 0", code, out.String(), errb.String())
	}
	if !strings.HasPrefix(out.String(), "[cycle-health] cycle=4 workspace="+ws+"\n") || !strings.Contains(out.String(), "[cycle-health] verdict: OK (") {
		t.Errorf("stdout = %q, want the header and the OK verdict", out.String())
	}
}

func TestCycleHealthRoot(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	var stderr bytes.Buffer
	if got := cycleHealthRoot("/p/.evolve/runs/cycle-9", &stderr); got != "/p" {
		t.Errorf("cycleHealthRoot from the workspace = %q, want /p", got)
	}
	env := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", env)
	if got := cycleHealthRoot("/p/.evolve/runs/cycle-9", &stderr); got != env {
		t.Errorf("cycleHealthRoot = %q, want the env root %q", got, env)
	}
	prev := cycleHealthAbsoluteRoot
	t.Cleanup(func() { cycleHealthAbsoluteRoot = prev })
	cycleHealthAbsoluteRoot = func(label, p string, warn func(string)) string {
		warn("cannot resolve " + label)
		return p
	}
	if got := cycleHealthRoot("ws", &stderr); got != env || stderr.String() != "evolve cycle-health: WARN: cannot resolve EVOLVE_PROJECT_ROOT\n" {
		t.Errorf("cycleHealthRoot = %q stderr=%q, want the env root and one WARN", got, stderr.String())
	}
}

func TestStopReviewRecorder_AppendsOneStopReviewEntry(t *testing.T) {
	t.Parallel()
	fake := &fixtures.FakeLedger{}

	stopReviewRecorder(fake)(12, "build", "extend", "still writing")

	if len(fake.Entries) != 1 {
		t.Fatalf("entries = %+v, want one", fake.Entries)
	}
	e := fake.Entries[0]
	if e.Cycle != 12 || e.Role != "build" || e.Kind != "stop_review" || e.Action != "extend" || e.Message != "still writing" {
		t.Errorf("entry = %+v", e)
	}
	if at, err := time.Parse(time.RFC3339, e.TS); err != nil || time.Since(at) > time.Minute {
		t.Errorf("TS = %q (%v), want a current RFC3339 stamp", e.TS, err)
	}
}

func TestCycleDiagfAndBench_WriteToTheProcessStderr(t *testing.T) {
	stderr := captureProcessStderr(t, func() {
		cycleDiagf("[diag] %s=%d\n", "x", 3)
		cycleBenchOnEscalation(bridgechain.Escalation{Env: map[string]string{"EVOLVE_CLI_HEALTH": "0"}})
	})
	if stderr != "[diag] x=3\n" {
		t.Errorf("stderr = %q, want only the diag line (a disabled bench is silent)", stderr)
	}
}

func TestCatalogRefreshStage_ReadsThePolicyStage(t *testing.T) {
	t.Parallel()
	pol, err := policy.Parse([]byte(`{"catalog":{"refresh_stage":"enforce"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := catalogRefreshStage(pol)(); got != "enforce" {
		t.Errorf("stage = %q, want enforce", got)
	}
	if got, want := catalogRefreshStage(policy.Policy{})(), (policy.Policy{}).CatalogConfig().RefreshStage; got != want {
		t.Errorf("default stage = %q, want %q", got, want)
	}
}

func TestFailureCountReaderAndContinuationForScope_ReadTheProjectInbox(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	seedFailedCycleInbox(t, root, "counted", 3)
	item := filepath.Join(root, ".evolve", "inbox", "2026-09-13T00-00-00Z-counted.json")
	if err := os.WriteFile(item, []byte(`{"id":"counted","title":"t","kind":"bug","failure_count":2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	read := failureCountReader(root)
	if got := read("counted"); got != 2 {
		t.Errorf("failure count = %d, want the item's durable 2", got)
	}
	if got := read("absent-task"); got != 0 {
		t.Errorf("failure count of an absent task = %d, want 0", got)
	}
	if got := continuationForScope(root, 3, []string{"absent-task"}); got != nil {
		t.Errorf("continuation = %+v, want nil with no prior attempt", got)
	}
}

func TestWireOrchestratorDeps_AMalformedFailurePolicyWarnsAndKeepsTheDefaults(t *testing.T) {
	orig := runner.DefaultRouter
	t.Cleanup(func() { runner.DefaultRouter = orig })
	root, evolveDir := wiringRoot(t, `{"failure_policy":{"categories":{"made-up":{"level":"galaxy","action":"halt_and_diagnose"}}}}`)
	var d orchDeps

	stderr := captureProcessStderr(t, func() { d = wireOrchestratorDeps(root, evolveDir, io.Discard, routingRun{}) })

	if d.RoutingErr != nil || d.Orchestrator == nil {
		t.Fatalf("wiring = %v, want an orchestrator despite the bad failure_policy", d.RoutingErr)
	}
	if !strings.Contains(stderr, `[cycle] WARN: malformed failure_policy in policy.json (policy: failure_policy category "made-up" has unknown level "galaxy"`) {
		t.Errorf("stderr = %q, want the failure_policy WARN", stderr)
	}
}

func TestWireOrchestratorDeps_AShipFloorOverrideWiresWithoutAWarning(t *testing.T) {
	orig := runner.DefaultRouter
	t.Cleanup(func() { runner.DefaultRouter = orig })
	root, evolveDir := wiringRoot(t, `{"ship_floor":["build","audit"]}`)
	var console bytes.Buffer

	d := wireOrchestratorDeps(root, evolveDir, &console, routingRun{})

	if d.RoutingErr != nil || d.Orchestrator == nil || strings.Contains(console.String(), "[policy] WARN") {
		t.Errorf("wiring err=%v console=%q, want a clean wire with the floor override", d.RoutingErr, console.String())
	}
}

func TestWireOrchestratorDeps_AnInvalidUserPhaseIsWarnedAndGetsNoRunner(t *testing.T) {
	orig := runner.DefaultRouter
	t.Cleanup(func() { runner.DefaultRouter = orig })
	root, evolveDir := wiringRoot(t, "")
	phaseDir := filepath.Join(root, ".evolve", "phases", "loudcheck")
	if err := os.MkdirAll(phaseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(phaseDir, "phase.json"), []byte(`{"name":"loudcheck","optional":false,"catalog":"on-demand"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var d orchDeps

	stderr := captureProcessStderr(t, func() { d = wireOrchestratorDeps(root, evolveDir, io.Discard, routingRun{}) })

	if d.RoutingErr != nil {
		t.Fatalf("wiring err = %v", d.RoutingErr)
	}
	if !strings.Contains(stderr, "[phases] WARN phase loudcheck not routed (invalid): ") {
		t.Errorf("stderr = %q, want the invalid-phase WARN", stderr)
	}
	if _, wired := d.Runners[core.Phase("loudcheck")]; wired {
		t.Error("an invalid user phase must get no runner")
	}
}

func TestWireOrchestratorDeps_AnAdvisorWithoutARouteRefusesTheWiring(t *testing.T) {
	orig := runner.DefaultRouter
	t.Cleanup(func() { runner.DefaultRouter = orig })
	root, evolveDir := wiringRoot(t, `{"cli_routing":{"clis":["agy","claude"],"default":["agy","claude"]}}`)
	t.Setenv("EVOLVE_ROUTER_CLI", "codex-tmux")

	d := wireOrchestratorDeps(root, evolveDir, io.Discard, routingRun{})

	if d.RoutingErr == nil || !strings.HasPrefix(d.RoutingErr.Error(), "the routing advisor has no route: ") || d.Orchestrator != nil || d.Signals == nil {
		t.Errorf("wiring = (err=%v orch=%v signals=%v), want the advisor refusal with the Center kept", d.RoutingErr, d.Orchestrator, d.Signals)
	}
}
