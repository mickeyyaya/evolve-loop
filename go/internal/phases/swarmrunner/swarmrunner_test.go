package swarmrunner

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
)

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

func TestDecorator_Name(t *testing.T) {
	if New(&fakeInner{name: "build"}, &fakeBridge{}, swarm.ModeWriter).Name() != "build" {
		t.Error("Name must be transparent (inner phase name)")
	}
}
