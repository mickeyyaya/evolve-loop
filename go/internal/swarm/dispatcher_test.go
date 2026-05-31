package swarm

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeLauncher records launches, tracks max in-flight (for the cap test), and
// can be scripted to fail per-agent.
type fakeLauncher struct {
	mu        sync.Mutex
	launched  []string
	inFlight  int32
	maxInFl   int32
	exit      map[string]int
	failAgent map[string]bool
	delay     time.Duration
}

func (f *fakeLauncher) Launch(ctx context.Context, req LaunchRequest) (LaunchResult, error) {
	cur := atomic.AddInt32(&f.inFlight, 1)
	for {
		m := atomic.LoadInt32(&f.maxInFl)
		if cur <= m || atomic.CompareAndSwapInt32(&f.maxInFl, m, cur) {
			break
		}
	}
	defer atomic.AddInt32(&f.inFlight, -1)
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return LaunchResult{}, ctx.Err()
		}
	}
	f.mu.Lock()
	f.launched = append(f.launched, req.Agent)
	f.mu.Unlock()
	if f.failAgent[req.Agent] {
		return LaunchResult{}, errors.New("launch failed")
	}
	return LaunchResult{ExitCode: f.exit[req.Agent], PGID: 4242, TmuxSession: "sess-" + req.Agent}, nil
}

func twoWriterPlan() SwarmPlan {
	return SwarmPlan{
		Mode: ModeWriter, Partitionable: true, TaskID: "t1",
		IntegrationBranch: "cycle-1-integration",
		Workers: []WorkerSpec{
			{WorkerID: "w0", CLI: "claude", Branch: "cycle-1-w0", TargetFiles: []string{"a.go"}, Scope: "A"},
			{WorkerID: "w1", CLI: "codex", Branch: "cycle-1-w1", TargetFiles: []string{"b.go"}, DependsOn: []string{"w0"}, Scope: "B"},
		},
	}
}

type noopKiller struct{}

func (noopKiller) Kill(context.Context, SessionHandle) error { return nil }

func TestDispatch_LaunchesAllWorkersAndReaps(t *testing.T) {
	reg := NewSessionRegistry(filepath.Join(t.TempDir(), "s.json"), 1, "build", 1)
	fk := &fakeLauncher{}
	deps := Deps{Launcher: fk, Registry: reg, Killer: noopKiller{}, Concurrency: 2}

	res, err := Dispatch(context.Background(), twoWriterPlan(),
		DispatchRequest{ProjectRoot: ".", Cycle: 1, Workspace: t.TempDir()}, deps)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(res.Workers) != 2 || !res.AllOK() {
		t.Fatalf("want 2 OK workers, got %+v", res.Workers)
	}
	if len(fk.launched) != 2 {
		t.Errorf("both workers should launch, got %v", fk.launched)
	}
	if len(res.MergeOrder) != 2 || res.MergeOrder[0] != "w0" || res.MergeOrder[1] != "w1" {
		t.Errorf("merge order wrong: %v", res.MergeOrder)
	}
	if len(reg.Live()) != 0 {
		t.Errorf("sessions must be reaped after Dispatch, got live %v", reg.Live())
	}
}

func TestDispatch_ConcurrencyCapHonored(t *testing.T) {
	fk := &fakeLauncher{delay: 30 * time.Millisecond}
	plan := SwarmPlan{Mode: ModeReader, Partitionable: true, TaskID: "r", Workers: []WorkerSpec{
		{WorkerID: "w0"}, {WorkerID: "w1"}, {WorkerID: "w2"}, {WorkerID: "w3"},
	}}
	deps := Deps{Launcher: fk, Concurrency: 2}
	if _, err := Dispatch(context.Background(), plan, DispatchRequest{Workspace: t.TempDir()}, deps); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&fk.maxInFl); got > 2 {
		t.Errorf("concurrency cap=2 violated, max in-flight=%d", got)
	}
}

func TestDispatch_FatalErrorPropagates(t *testing.T) {
	fk := &fakeLauncher{failAgent: map[string]bool{"r-w0": true}}
	plan := SwarmPlan{Mode: ModeReader, Partitionable: true, TaskID: "r", Workers: []WorkerSpec{
		{WorkerID: "w0"}, {WorkerID: "w1"},
	}}
	res, err := Dispatch(context.Background(), plan, DispatchRequest{Workspace: t.TempDir()}, Deps{Launcher: fk, Concurrency: 2})
	if err == nil {
		t.Fatal("expected fatal error to propagate")
	}
	if res.AllOK() {
		t.Error("result should not be AllOK when a worker failed")
	}
}

func TestDispatch_ReaderNeedsNoProvisioner(t *testing.T) {
	fk := &fakeLauncher{}
	plan := SwarmPlan{Mode: ModeReader, Partitionable: true, TaskID: "r", Workers: []WorkerSpec{
		{WorkerID: "w0"}, {WorkerID: "w1"},
	}}
	res, err := Dispatch(context.Background(), plan, DispatchRequest{Workspace: t.TempDir()},
		Deps{Launcher: fk, Concurrency: 2})
	if err != nil {
		t.Fatalf("reader dispatch: %v", err)
	}
	if !res.AllOK() || res.MergeOrder != nil {
		t.Errorf("reader: want AllOK + nil MergeOrder, got %+v", res)
	}
}
