package swarmrunner

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
)

// artifactBridge writes a per-worker report to the requested ArtifactPath so the
// reader fan-in has real worker summaries to synthesize.
type artifactBridge struct{ launches int32 }

func (b *artifactBridge) Launch(_ context.Context, r core.BridgeRequest) (core.BridgeResponse, error) {
	atomic.AddInt32(&b.launches, 1)
	if r.ArtifactPath != "" {
		_ = os.MkdirAll(filepath.Dir(r.ArtifactPath), 0o755)
		_ = os.WriteFile(r.ArtifactPath, []byte("summary from "+r.Agent), 0o644)
	}
	return core.BridgeResponse{ExitCode: 0, CostUSD: 0.01}, nil
}

func (b *artifactBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

// fakeInner is a core.PhaseRunner that records whether it ran.
type fakeInner struct {
	name string
	ran  int32
}

func (f *fakeInner) Name() string { return f.name }
func (f *fakeInner) Run(context.Context, core.PhaseRequest) (core.PhaseResponse, error) {
	atomic.AddInt32(&f.ran, 1)
	return core.PhaseResponse{Phase: f.name, Verdict: core.VerdictPASS, Signals: map[string]any{"inner": true}}, nil
}

// fakeBridge counts launches (one per dispatched worker).
type fakeBridge struct{ launches int32 }

func (b *fakeBridge) Launch(context.Context, core.BridgeRequest) (core.BridgeResponse, error) {
	atomic.AddInt32(&b.launches, 1)
	return core.BridgeResponse{ExitCode: 0, CostUSD: 0.01}, nil
}

func (b *fakeBridge) Probe(context.Context) (core.BridgeProbe, error) { return core.BridgeProbe{}, nil }

func writePlan(t *testing.T, ws string, mode swarm.Mode) {
	t.Helper()
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	plan := `{"swarm_plan":{"task_id":"t","mode":"` + string(mode) + `","partitionable":true,"workers":[` +
		`{"worker_id":"w0","cli":"claude","scope":"A"},` +
		`{"worker_id":"w1","cli":"codex","scope":"B"}]}}`
	if err := os.WriteFile(filepath.Join(ws, "swarm-plan.json"), []byte(plan), 0o644); err != nil {
		t.Fatal(err)
	}
}

func reqWith(ws string, env map[string]string) core.PhaseRequest {
	return core.PhaseRequest{Cycle: 1, ProjectRoot: ".", Workspace: ws, Env: env}
}

func reqWithRoot(root, ws string, env map[string]string) core.PhaseRequest {
	return core.PhaseRequest{Cycle: 1, ProjectRoot: root, Workspace: ws, Env: env}
}

// gitInitForTest makes a throwaway repo with one commit so writer-mode
// worktree provisioning never touches the developer's repository.
func gitInitForTest(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "t@t.local")
	run("config", "user.name", "T")
	run("config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "init")
	return root
}

func TestDecorator_ShadowDelegates(t *testing.T) {
	inner := &fakeInner{name: "scout"}
	bridge := &fakeBridge{}
	d := New(inner, bridge, swarm.ModeReader, Config{})
	ws := t.TempDir()
	writePlan(t, ws, swarm.ModeReader)

	// Config{} (Stage="") → stageOff → pure delegate, NO dispatch.
	resp, err := d.Run(context.Background(), reqWith(ws, map[string]string{}))
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&inner.ran) != 1 {
		t.Error("shadow must run the inner runner")
	}
	if atomic.LoadInt32(&bridge.launches) != 0 {
		t.Errorf("shadow must NOT dispatch workers, got %d launches", bridge.launches)
	}
	if resp.Signals["inner"] != true {
		t.Error("shadow must return the inner response verbatim")
	}
}

func TestDecorator_AdvisoryDispatchesButInnerAuthoritative(t *testing.T) {
	inner := &fakeInner{name: "scout"}
	bridge := &fakeBridge{}
	d := New(inner, bridge, swarm.ModeReader, Config{Stage: "advisory"})
	ws := t.TempDir()
	writePlan(t, ws, swarm.ModeReader)

	resp, err := d.Run(context.Background(), reqWith(ws, map[string]string{}))
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&bridge.launches) != 2 {
		t.Errorf("advisory must dispatch 2 reader workers, got %d", bridge.launches)
	}
	if atomic.LoadInt32(&inner.ran) != 1 {
		t.Error("advisory must still run the inner runner (authoritative)")
	}
	if resp.Signals["swarm.stage"] != "advisory" || resp.Signals["inner"] != true {
		t.Errorf("advisory response must carry both inner result + swarm signals: %+v", resp.Signals)
	}
	if resp.Signals["swarm.worker_count"] != 2 {
		t.Errorf("advisory should record 2 workers, got %v", resp.Signals["swarm.worker_count"])
	}
}

func TestDecorator_EnforceReaderDrivesResult(t *testing.T) {
	inner := &fakeInner{name: "scout"}
	bridge := &fakeBridge{}
	d := New(inner, bridge, swarm.ModeReader, Config{Stage: "enforce"})
	ws := t.TempDir()
	writePlan(t, ws, swarm.ModeReader)

	resp, err := d.Run(context.Background(), reqWith(ws, map[string]string{}))
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&bridge.launches) != 2 {
		t.Errorf("enforce must dispatch 2 workers, got %d", bridge.launches)
	}
	if atomic.LoadInt32(&inner.ran) != 0 {
		t.Error("enforce reader must NOT run the inner runner (swarm is authoritative)")
	}
	if resp.Verdict != core.VerdictPASS || resp.Signals["swarm.stage"] != "enforce" {
		t.Errorf("enforce response wrong: verdict=%s signals=%+v", resp.Verdict, resp.Signals)
	}
}

func TestDecorator_EnforceReaderSynthesizesArtifact(t *testing.T) {
	inner := &fakeInner{name: "scout"}
	bridge := &artifactBridge{}
	d := New(inner, bridge, swarm.ModeReader, Config{Stage: "enforce"})
	ws := t.TempDir()
	writePlan(t, ws, swarm.ModeReader)

	resp, err := d.Run(context.Background(), reqWith(ws, map[string]string{}))
	if err != nil {
		t.Fatal(err)
	}

	// Reader enforce must fold both workers' reports into ONE synthesized artifact
	// at the phase's canonical report path (<phase>-report.md).
	synth := filepath.Join(ws, "scout-report.md")
	data, rerr := os.ReadFile(synth)
	if rerr != nil {
		t.Fatalf("reader enforce must write synthesized %s: %v", synth, rerr)
	}
	body := string(data)
	for _, agent := range []string{"t-w0", "t-w1"} {
		if !strings.Contains(body, "summary from "+agent) {
			t.Errorf("synthesis missing worker %s content:\n%s", agent, body)
		}
	}
	if resp.ArtifactsDir != ws {
		t.Errorf("reader enforce ArtifactsDir = %q, want %q", resp.ArtifactsDir, ws)
	}
	if resp.Signals["swarm.synthesis"] != synth {
		t.Errorf("reader enforce must record synthesis path, got %v", resp.Signals["swarm.synthesis"])
	}
}

// envBridge captures the env the bridge received, to assert the per-worker overlay.
type envBridge struct{ gotEnv map[string]string }

func (b *envBridge) Launch(_ context.Context, r core.BridgeRequest) (core.BridgeResponse, error) {
	b.gotEnv = r.Env
	return core.BridgeResponse{ExitCode: 0}, nil
}

func (b *envBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func TestBridgeLauncher_MergesPerWorkerEnvOverPhaseEnv(t *testing.T) {
	b := &envBridge{}
	l := bridgeLauncher{bridge: b, env: map[string]string{"A": "1", "PORT": "9000"}}

	if _, err := l.Launch(context.Background(), swarm.LaunchRequest{
		Env: map[string]string{"PORT": "52000"}, // per-worker overlay
	}); err != nil {
		t.Fatal(err)
	}
	if b.gotEnv["A"] != "1" {
		t.Error("phase-level env must be preserved")
	}
	if b.gotEnv["PORT"] != "52000" {
		t.Errorf("per-worker env must override phase env, got PORT=%q", b.gotEnv["PORT"])
	}
}

func TestBridgeLauncher_NoOverlayKeepsPhaseEnv(t *testing.T) {
	b := &envBridge{}
	phaseEnv := map[string]string{"A": "1"}
	l := bridgeLauncher{bridge: b, env: phaseEnv}

	if _, err := l.Launch(context.Background(), swarm.LaunchRequest{}); err != nil {
		t.Fatal(err)
	}
	if b.gotEnv["A"] != "1" {
		t.Errorf("no overlay must pass phase env through, got %v", b.gotEnv)
	}
	// Must not mutate the caller's phase-env map.
	if _, leaked := phaseEnv["PORT"]; leaked {
		t.Error("merge must not mutate the shared phase-env map")
	}
}

func TestDecorator_EnforceReaderMarksMissingArtifact(t *testing.T) {
	inner := &fakeInner{name: "scout"}
	bridge := &fakeBridge{} // succeeds but writes NO report file
	d := New(inner, bridge, swarm.ModeReader, Config{Stage: "enforce"})
	ws := t.TempDir()
	writePlan(t, ws, swarm.ModeReader)

	if _, err := d.Run(context.Background(), reqWith(ws, map[string]string{})); err != nil {
		t.Fatal(err)
	}

	// A 0-exit worker that produced no report must be surfaced loudly in the
	// synthesis, not silently dropped (Rule 12 — fail loudly).
	data, rerr := os.ReadFile(filepath.Join(ws, "scout-report.md"))
	if rerr != nil {
		t.Fatalf("synthesis must still be written: %v", rerr)
	}
	if !strings.Contains(string(data), "[no artifact") {
		t.Errorf("missing worker report must be marked, got:\n%s", data)
	}
}

func TestConfigPortBase_FlowsThroughToDispatchDeps(t *testing.T) {
	cases := []struct {
		name     string
		portBase int
		want     int
	}{
		{"zero (dispatcher applies default)", 0, 0},
		{"valid override", 60000, 60000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := New(&fakeInner{name: "build"}, &fakeBridge{}, swarm.ModeWriter, Config{PortBase: tc.portBase})
			deps := d.dispatchDeps(core.PhaseRequest{Cycle: 1, Workspace: t.TempDir(), Env: map[string]string{}})
			if deps.PortBase != tc.want {
				t.Errorf("Config{PortBase: %d} → deps.PortBase = %d, want %d", tc.portBase, deps.PortBase, tc.want)
			}
		})
	}
}

func TestDecorator_Name(t *testing.T) {
	if New(&fakeInner{name: "build"}, &fakeBridge{}, swarm.ModeWriter, Config{}).Name() != "build" {
		t.Error("Name must be transparent (inner phase name)")
	}
}

// errBridge simulates a total transport failure (bridge is unreachable / crashed).
type errBridge struct{}

func (b *errBridge) Launch(_ context.Context, _ core.BridgeRequest) (core.BridgeResponse, error) {
	return core.BridgeResponse{}, errors.New("transport down")
}

func (b *errBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

// writeWriterPlan writes a valid WRITER swarm plan (disjoint files, two workers).
func writeWriterPlan(t *testing.T, ws string) {
	t.Helper()
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	plan := `{"swarm_plan":{"task_id":"t","mode":"writer","partitionable":true,` +
		`"workers":[{"worker_id":"w0","cli":"claude","target_files":["a.go"]},` +
		`{"worker_id":"w1","cli":"codex","target_files":["b.go"]}]}}`
	if err := os.WriteFile(filepath.Join(ws, "swarm-plan.json"), []byte(plan), 0o644); err != nil {
		t.Fatal(err)
	}
}

// branchByID must return a worker_id → branch map for every worker in the plan.
func TestBranchByID_Empty(t *testing.T) {
	if m := branchByID(swarm.SwarmPlan{}); len(m) != 0 {
		t.Errorf("empty plan must produce empty map, got %v", m)
	}
}

// annotate must emit swarm.error only when derr is non-nil.
func TestAnnotate_RecordsDispatchError(t *testing.T) {
	resp := &core.PhaseResponse{Signals: map[string]any{}}
	annotate(resp, swarm.SwarmPlan{Mode: swarm.ModeReader}, swarm.SwarmResult{},
		errors.New("transport failed"), "advisory")
	if _, ok := resp.Signals["swarm.error"]; !ok {
		t.Error("dispatch error must be recorded under swarm.error signal")
	}
}

func TestAnnotate_NoDispatchError_NoErrorSignal(t *testing.T) {
	resp := &core.PhaseResponse{Signals: map[string]any{}}
	annotate(resp, swarm.SwarmPlan{Mode: swarm.ModeReader}, swarm.SwarmResult{}, nil, "enforce")
	if _, ok := resp.Signals["swarm.error"]; ok {
		t.Error("no dispatch error must NOT produce swarm.error signal")
	}
}

// bridgeLauncher must propagate launch errors from the underlying bridge.
func TestBridgeLauncher_PropagatesLaunchError(t *testing.T) {
	l := bridgeLauncher{bridge: &errBridge{}, env: map[string]string{}}
	_, err := l.Launch(context.Background(), swarm.LaunchRequest{})
	if err == nil {
		t.Error("bridgeLauncher must propagate bridge launch errors")
	}
}

// dispatchDeps: writer mode must inject a WorkerProvisioner; reader must not.
func TestDecorator_DispatchDeps_WriterMode_SetsProvisioner(t *testing.T) {
	d := New(&fakeInner{name: "build"}, &fakeBridge{}, swarm.ModeWriter, Config{})
	deps := d.dispatchDeps(reqWith(t.TempDir(), map[string]string{}))
	if deps.Provisioner == nil {
		t.Error("writer mode dispatchDeps must set a non-nil WorkerProvisioner")
	}
}

func TestDecorator_DispatchDeps_ReaderMode_NoProvisioner(t *testing.T) {
	d := New(&fakeInner{name: "scout"}, &fakeBridge{}, swarm.ModeReader, Config{})
	deps := d.dispatchDeps(reqWith(t.TempDir(), map[string]string{}))
	if deps.Provisioner != nil {
		t.Error("reader mode dispatchDeps must NOT set a WorkerProvisioner")
	}
}

// enforce must return FAIL when all workers fail due to transport errors.
func TestDecorator_EnforceReader_WorkerFailureReturnsFail(t *testing.T) {
	d := New(&fakeInner{name: "scout"}, &errBridge{}, swarm.ModeReader, Config{Stage: "enforce"})
	ws := t.TempDir()
	writePlan(t, ws, swarm.ModeReader)

	resp, err := d.Run(context.Background(), reqWith(ws, map[string]string{}))
	if err == nil {
		t.Fatal("enforce with all-worker transport failure must return a non-nil error")
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("enforce must produce VerdictFAIL on dispatch error, got %s", resp.Verdict)
	}
}

// A non-partitionable plan must collapse to the inner runner even in enforce mode.
func TestDecorator_ValidateCollapse_FallsBackToInner(t *testing.T) {
	inner := &fakeInner{name: "build"}
	d := New(inner, &fakeBridge{}, swarm.ModeWriter, Config{Stage: "enforce"})
	ws := t.TempDir()
	// Non-partitionable → Validate returns Collapse=true → inner must run.
	plan := `{"swarm_plan":{"task_id":"t","mode":"writer","partitionable":false,` +
		`"workers":[{"worker_id":"w0","cli":"claude"},{"worker_id":"w1","cli":"codex"}]}}`
	_ = os.MkdirAll(ws, 0o755)
	_ = os.WriteFile(filepath.Join(ws, "swarm-plan.json"), []byte(plan), 0o644)

	_, err := d.Run(context.Background(), reqWith(ws, map[string]string{}))
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&inner.ran) != 1 {
		t.Error("non-partitionable plan must fall back to the inner runner exactly once")
	}
}

// parseSwarmStage must map "shadow" (explicit) and any unknown value to stageOff.
func TestParseSwarmStage_ShadowAndUnknownMapToOff(t *testing.T) {
	for _, input := range []string{"shadow", "SHADOW", "", "unknown_value"} {
		if st := parseSwarmStage(input); st != stageOff {
			t.Errorf("parseSwarmStage(%q) = %v, want stageOff", input, st)
		}
	}
}

// loadPlan must return ok=false for a workspace with neither swarm-plan.json nor
// swarm-plan.md, ensuring a graceful N=1 fallback rather than a panic.
func TestDecorator_NoPlanFile_FallsBackToInner(t *testing.T) {
	inner := &fakeInner{name: "build"}
	bridge := &fakeBridge{}
	d := New(inner, bridge, swarm.ModeWriter, Config{Stage: "enforce"})
	ws := t.TempDir() // no plan file written

	_, err := d.Run(context.Background(), reqWith(ws, map[string]string{}))
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&inner.ran) != 1 {
		t.Error("missing plan must fall back to inner runner")
	}
	if atomic.LoadInt32(&bridge.launches) != 0 {
		t.Error("missing plan must NOT dispatch any swarm workers")
	}
}

// mergeEnv must produce a new map (not alias the base or overlay) so concurrent
// workers that receive the same base env can't corrupt each other's copies.
func TestMergeEnv_NoAliasOfBase(t *testing.T) {
	base := map[string]string{"A": "1", "B": "2"}
	overlay := map[string]string{"B": "override", "C": "3"}
	out := mergeEnv(base, overlay)

	if out["A"] != "1" {
		t.Errorf("base key A not preserved: %v", out)
	}
	if out["B"] != "override" {
		t.Errorf("overlay must win for key B, got %v", out["B"])
	}
	if out["C"] != "3" {
		t.Errorf("overlay-only key C missing: %v", out)
	}
	// Mutating the output must not affect the base (no aliasing).
	out["A"] = "mutated"
	if base["A"] != "1" {
		t.Error("mergeEnv must not alias the base map (mutation leaked)")
	}
}

func TestMergeEnv_NilBase(t *testing.T) {
	out := mergeEnv(nil, map[string]string{"X": "1"})
	if out["X"] != "1" {
		t.Errorf("mergeEnv with nil base must include overlay, got %v", out)
	}
}

func TestMergeEnv_NilOverlay(t *testing.T) {
	out := mergeEnv(map[string]string{"X": "1"}, nil)
	if out["X"] != "1" {
		t.Errorf("mergeEnv with nil overlay must include base, got %v", out)
	}
}

// annotate must initialise Signals if the response had a nil map (defensive
// nil-safety for callers that produce a bare PhaseResponse).
func TestAnnotate_InitialisesNilSignals(t *testing.T) {
	resp := &core.PhaseResponse{} // Signals is nil
	annotate(resp, swarm.SwarmPlan{Mode: swarm.ModeWriter}, swarm.SwarmResult{}, nil, "enforce")
	if resp.Signals == nil {
		t.Error("annotate must initialise a nil Signals map")
	}
	if resp.Signals["swarm.stage"] != "enforce" {
		t.Errorf("swarm.stage not set after nil-init, signals=%v", resp.Signals)
	}
}

// parseSwarmStage must correctly parse advisory and enforce, and default to stageOff
// for any other input (fail-safe: a typo or env mismatch never silently enables swarm).
func TestParseSwarmStage_AdvisoryAndEnforce(t *testing.T) {
	if st := parseSwarmStage("advisory"); st != stageAdvisory {
		t.Errorf("parseSwarmStage(\"advisory\") = %v, want stageAdvisory", st)
	}
	if st := parseSwarmStage("ADVISORY"); st != stageAdvisory {
		t.Errorf("parseSwarmStage(\"ADVISORY\") = %v, want stageAdvisory", st)
	}
	if st := parseSwarmStage("enforce"); st != stageEnforce {
		t.Errorf("parseSwarmStage(\"enforce\") = %v, want stageEnforce", st)
	}
	if st := parseSwarmStage("ENFORCE"); st != stageEnforce {
		t.Errorf("parseSwarmStage(\"ENFORCE\") = %v, want stageEnforce", st)
	}
}

// enforce reader with a valid plan and workers that write their reports must
// synthesise a combined report at <phase>-report.md in the workspace.
func TestDecorator_EnforceReaderSynthesisPath(t *testing.T) {
	inner := &fakeInner{name: "scout"}
	bridge := &artifactBridge{}
	d := New(inner, bridge, swarm.ModeReader, Config{Stage: "enforce"})
	ws := t.TempDir()
	writePlan(t, ws, swarm.ModeReader)

	resp, err := d.Run(context.Background(), reqWith(ws, map[string]string{}))
	if err != nil {
		t.Fatalf("reader enforce with artifact bridge must not error: %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("reader enforce with all OK workers must PASS, got %s", resp.Verdict)
	}
	synthPath := filepath.Join(ws, "scout-report.md")
	if _, err := os.Stat(synthPath); err != nil {
		t.Errorf("synthesis must be written to scout-report.md, stat err=%v", err)
	}
	if resp.Signals["swarm.synthesis"] != synthPath {
		t.Errorf("swarm.synthesis signal must point to %s, got %v", synthPath, resp.Signals["swarm.synthesis"])
	}
}

// Enforce writer path: when all workers fail → FAIL verdict + non-nil error.
func TestDecorator_EnforceWriter_WorkerFailureReturnsFail(t *testing.T) {
	inner := &fakeInner{name: "build"}
	root := gitInitForTest(t)
	// Exercise the Config.WorktreeBase DI seam (replaces the former
	// EVOLVE_WORKTREE_BASE env read; flag-reduction, ADR-0064).
	d := New(inner, &errBridge{}, swarm.ModeWriter, Config{Stage: "enforce", WorktreeBase: filepath.Join(root, ".evolve", "worktrees")})
	ws := t.TempDir()
	writeWriterPlan(t, ws)

	resp, err := d.Run(context.Background(), reqWithRoot(root, ws, map[string]string{}))
	if err == nil {
		t.Fatal("enforce writer with transport failure must return error")
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("enforce writer transport failure must produce VerdictFAIL, got %s", resp.Verdict)
	}
	// inner runner must NOT have run (swarm is authoritative in enforce mode).
	if atomic.LoadInt32(&inner.ran) != 0 {
		t.Error("enforce writer must not run the inner runner")
	}
}

// TestBranchByID verifies the pure plan→branchMap helper.
func TestBranchByID(t *testing.T) {
	plan := swarm.SwarmPlan{Workers: []swarm.WorkerSpec{
		{WorkerID: "w0", Branch: "cycle-1-w0"},
		{WorkerID: "w1", Branch: "cycle-1-w1"},
	}}
	got := branchByID(plan)
	if got["w0"] != "cycle-1-w0" || got["w1"] != "cycle-1-w1" {
		t.Errorf("branchByID wrong: %v", got)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 entries, got %d", len(got))
	}
}

// TestDecorator_Enforce_WorkerFail covers the !sr.AllOK() path in enforce.
func TestDecorator_Enforce_WorkerFail(t *testing.T) {
	d := New(&fakeInner{name: "scout"}, &fakeBridge{}, swarm.ModeReader, Config{})
	plan := swarm.SwarmPlan{Mode: swarm.ModeReader, Workers: []swarm.WorkerSpec{{WorkerID: "w0"}}}
	sr := swarm.SwarmResult{Workers: []swarm.WorkerResult{{WorkerID: "w0", ExitCode: 1}}}
	resp, _ := d.enforce(context.Background(), core.PhaseRequest{}, plan, sr, nil)
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("enforce verdict = %q, want FAIL when worker fails", resp.Verdict)
	}
}
