package swarmrunner

import (
	"context"
	"os"
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

func TestDecorator_ShadowDelegates(t *testing.T) {
	inner := &fakeInner{name: "scout"}
	bridge := &fakeBridge{}
	d := New(inner, bridge, swarm.ModeReader)
	ws := t.TempDir()
	writePlan(t, ws, swarm.ModeReader)

	// No EVOLVE_SWARM_STAGE → off → pure delegate, NO dispatch.
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

func TestDecorator_NoPlanFallsBackToInner(t *testing.T) {
	inner := &fakeInner{name: "build"}
	bridge := &fakeBridge{}
	d := New(inner, bridge, swarm.ModeWriter)
	ws := t.TempDir() // no swarm-plan.json written

	resp, err := d.Run(context.Background(), reqWith(ws, map[string]string{"EVOLVE_SWARM_STAGE": "enforce"}))
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&inner.ran) != 1 || atomic.LoadInt32(&bridge.launches) != 0 {
		t.Errorf("missing plan must fall back to inner N=1 (ran=%d launches=%d)", inner.ran, bridge.launches)
	}
	_ = resp
}

func TestDecorator_AdvisoryDispatchesButInnerAuthoritative(t *testing.T) {
	inner := &fakeInner{name: "scout"}
	bridge := &fakeBridge{}
	d := New(inner, bridge, swarm.ModeReader)
	ws := t.TempDir()
	writePlan(t, ws, swarm.ModeReader)

	resp, err := d.Run(context.Background(), reqWith(ws, map[string]string{"EVOLVE_SWARM_STAGE": "advisory"}))
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
	d := New(inner, bridge, swarm.ModeReader)
	ws := t.TempDir()
	writePlan(t, ws, swarm.ModeReader)

	resp, err := d.Run(context.Background(), reqWith(ws, map[string]string{"EVOLVE_SWARM_STAGE": "enforce"}))
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
	d := New(inner, bridge, swarm.ModeReader)
	ws := t.TempDir()
	writePlan(t, ws, swarm.ModeReader)

	resp, err := d.Run(context.Background(), reqWith(ws, map[string]string{"EVOLVE_SWARM_STAGE": "enforce"}))
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
	d := New(inner, bridge, swarm.ModeReader)
	ws := t.TempDir()
	writePlan(t, ws, swarm.ModeReader)

	if _, err := d.Run(context.Background(), reqWith(ws, map[string]string{"EVOLVE_SWARM_STAGE": "enforce"})); err != nil {
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

func TestPortBaseFromEnv(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want int
	}{
		{"unset → 0 (dispatcher applies default)", map[string]string{}, 0},
		{"valid override", map[string]string{"EVOLVE_SWARM_PORT_BASE": "60000"}, 60000},
		{"invalid → 0", map[string]string{"EVOLVE_SWARM_PORT_BASE": "not-a-port"}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := portBaseFromEnv(tc.env); got != tc.want {
				t.Errorf("portBaseFromEnv(%v) = %d, want %d", tc.env, got, tc.want)
			}
		})
	}
}

func TestDecorator_Name(t *testing.T) {
	if New(&fakeInner{name: "build"}, &fakeBridge{}, swarm.ModeWriter).Name() != "build" {
		t.Error("Name must be transparent (inner phase name)")
	}
}
