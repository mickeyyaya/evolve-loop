package swarm

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

// fakeLauncher records launches, tracks max in-flight (for the cap test), and
// can be scripted to fail per-agent.
type fakeLauncher struct {
	mu          sync.Mutex
	launched    []string
	portByAgent map[string]string // captures the injected PORT env per worker
	inFlight    int32
	maxInFl     int32
	exit        map[string]int
	failAgent   map[string]bool
	delay       time.Duration
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
	if f.portByAgent == nil {
		f.portByAgent = map[string]string{}
	}
	f.portByAgent[req.Agent] = req.Env["PORT"] // nil-map read is "" — fine
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

// recordingKiller captures the SessionHandles it was asked to reap.
type recordingKiller struct {
	mu     sync.Mutex
	killed []SessionHandle
}

func (k *recordingKiller) Kill(_ context.Context, h SessionHandle) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.killed = append(k.killed, h)
	return nil
}

// Orphan-on-cancel hardening: a tmux worker whose LAUNCH FAILS must STILL be
// reapable — the dispatcher pre-registers a deterministic named session BEFORE
// the launch, so the post-wg Reap kills it by name even though Launch never
// returned a session identity.
func TestDispatch_TmuxWorker_PreRegisteredAndReapedOnLaunchFailure(t *testing.T) {
	reg := NewSessionRegistry(filepath.Join(t.TempDir(), "s.json"), 1, "scout", 1)
	fk := &fakeLauncher{failAgent: map[string]bool{"r-w0": true}}
	rk := &recordingKiller{}
	plan := SwarmPlan{Mode: ModeReader, Partitionable: true, TaskID: "r", Workers: []WorkerSpec{
		{WorkerID: "w0", CLI: "claude-tmux"},
	}}
	_, err := Dispatch(context.Background(), plan, DispatchRequest{Workspace: t.TempDir()},
		Deps{Launcher: fk, Registry: reg, Killer: rk, Concurrency: 1})
	if err == nil {
		t.Fatal("expected launch failure to propagate")
	}
	if len(rk.killed) != 1 {
		t.Fatalf("the pre-registered session must be reaped despite launch failure, got %d kills", len(rk.killed))
	}
	want := bridge.NamedSessionName("swarm-c0-w0") // sessionName = swarm-c<cycle>-<workerID>
	if rk.killed[0].TmuxSession != want {
		t.Errorf("reaped session name = %q, want the pre-pinned %q", rk.killed[0].TmuxSession, want)
	}
}

// Headless workers create no tmux session: they register with an empty session
// name (the killer is a benign no-op; ctx-cancel kills the subprocess).
func TestDispatch_HeadlessWorker_NoTmuxSession(t *testing.T) {
	reg := NewSessionRegistry(filepath.Join(t.TempDir(), "s.json"), 1, "scout", 1)
	fk := &fakeLauncher{}
	plan := SwarmPlan{Mode: ModeReader, Partitionable: true, TaskID: "r", Workers: []WorkerSpec{
		{WorkerID: "w0", CLI: "claude-p"},
	}}
	if _, err := Dispatch(context.Background(), plan, DispatchRequest{Workspace: t.TempDir()},
		Deps{Launcher: fk, Registry: reg, Killer: noopKiller{}, Concurrency: 1}); err != nil {
		t.Fatal(err)
	}
	// The session was recorded (manifest completeness) but with no tmux name.
	snap := reg.Snapshot()
	if len(snap) != 1 || snap[0].TmuxSession != "" {
		t.Errorf("headless worker must register with empty tmux session, got %+v", snap)
	}
}

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

func TestDispatch_CapturesWorkerArtifactPath(t *testing.T) {
	fk := &fakeLauncher{}
	ws := t.TempDir()
	res, err := Dispatch(context.Background(), twoWriterPlan(),
		DispatchRequest{ProjectRoot: ".", Cycle: 1, Workspace: ws}, Deps{Launcher: fk, Concurrency: 2})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	// Each worker result must record the artifact the dispatcher told it to write,
	// so the reader fan-in can fold them without re-deriving the path convention.
	for _, w := range res.Workers {
		want := filepath.Join(ws, w.Agent, w.Agent+"-report.md")
		if w.ArtifactPath != want {
			t.Errorf("worker %s ArtifactPath = %q, want %q", w.WorkerID, w.ArtifactPath, want)
		}
	}
}

func TestDispatch_WriterWorkersGetUniquePorts(t *testing.T) {
	fk := &fakeLauncher{}
	_, err := Dispatch(context.Background(), twoWriterPlan(),
		DispatchRequest{ProjectRoot: ".", Cycle: 1, Workspace: t.TempDir()},
		Deps{Launcher: fk, Concurrency: 2, PortBase: 52000})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	// base + worker index → collision-free within the plan, so concurrent dev
	// servers in isolated worktrees never clash.
	if fk.portByAgent["t1-w0"] != "52000" || fk.portByAgent["t1-w1"] != "52001" {
		t.Errorf("writer ports not isolated: %v", fk.portByAgent)
	}
}

func TestDispatch_WriterPortDefaultsWhenBaseUnset(t *testing.T) {
	fk := &fakeLauncher{}
	_, err := Dispatch(context.Background(), twoWriterPlan(),
		DispatchRequest{ProjectRoot: ".", Cycle: 1, Workspace: t.TempDir()},
		Deps{Launcher: fk, Concurrency: 2}) // PortBase 0 → DefaultPortBase
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if got := fk.portByAgent["t1-w0"]; got != strconv.Itoa(DefaultPortBase) {
		t.Errorf("unset base must default to %d, got %q", DefaultPortBase, got)
	}
}

func TestDispatch_ReaderWorkersGetNoPort(t *testing.T) {
	fk := &fakeLauncher{}
	plan := SwarmPlan{Mode: ModeReader, Partitionable: true, TaskID: "r", Workers: []WorkerSpec{
		{WorkerID: "w0"}, {WorkerID: "w1"},
	}}
	if _, err := Dispatch(context.Background(), plan, DispatchRequest{Workspace: t.TempDir()},
		Deps{Launcher: fk, Concurrency: 2}); err != nil {
		t.Fatal(err)
	}
	// Readers share the read-only tree and run no dev server → no port to isolate.
	if got := fk.portByAgent["r-w0"]; got != "" {
		t.Errorf("reader must NOT get an isolated port, got %q", got)
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

func TestDispatch_CancelWhileQueuedOnSemaphore(t *testing.T) {
	for attempt := 0; attempt < 20; attempt++ {
		fk := &fakeLauncher{
			delay:     20 * time.Millisecond,
			failAgent: map[string]bool{"r-w0": true},
		}
		plan := SwarmPlan{Mode: ModeReader, Partitionable: true, TaskID: "r", Workers: []WorkerSpec{
			{WorkerID: "w0"}, {WorkerID: "w1"}, {WorkerID: "w2"},
		}}

		res, err := Dispatch(context.Background(), plan, DispatchRequest{Workspace: t.TempDir()}, Deps{
			Launcher: fk, Concurrency: 1,
		})
		if err == nil {
			t.Fatal("expected fatal error from w0")
		}
		if len(res.Workers) != len(plan.Workers) {
			t.Fatalf("all workers must receive a result, got %d want %d", len(res.Workers), len(plan.Workers))
		}
		for _, wr := range res.Workers {
			if errors.Is(wr.Err, context.Canceled) {
				return
			}
		}
	}
	t.Fatal("expected at least one queued worker to receive context.Canceled")
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
