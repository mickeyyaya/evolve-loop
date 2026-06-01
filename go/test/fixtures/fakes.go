package fixtures

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// This file is the single source of truth for the orchestrator-surface test
// doubles. Before it existed, FakeStorage/FakeLedger/FakeRunner were defined
// three times (internal/routingtest, internal/core, cmd/evolve) with subtly
// different feature sets. The canonical versions here are SUPERSETS: a simple
// test leaves the injection fields zero-valued; a complex test sets them. One
// implementation covers every call site.

// FakeStorage is an in-memory core.Storage. Error-injection and lock-simulation
// are opt-in via the exported *Err / *FailAt fields (zero value = never fails).
// StateLog / CycleStateLog record every write in order for assertions.
type FakeStorage struct {
	mu sync.Mutex

	State      core.State
	CycleState core.CycleState

	StateLog      []core.State
	CycleStateLog []core.CycleState

	// Injection seams (zero value = disabled).
	ReadStateErr          error
	WriteStateErr         error
	ReadCycleStateErr     error
	WriteCycleStateErr    error
	LockErr               error
	WriteCycleStateFailAt int // 0 = never; N = the N-th WriteCycleState call fails

	// Observability.
	LockCount    int
	writeCSCalls int
	lockHeld     bool
}

func (f *FakeStorage) ReadState(context.Context) (core.State, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ReadStateErr != nil {
		return core.State{}, f.ReadStateErr
	}
	return f.State, nil
}

func (f *FakeStorage) WriteState(_ context.Context, s core.State) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.WriteStateErr != nil {
		return f.WriteStateErr
	}
	f.State = s
	f.StateLog = append(f.StateLog, s)
	return nil
}

func (f *FakeStorage) ReadCycleState(context.Context) (core.CycleState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ReadCycleStateErr != nil {
		return core.CycleState{}, f.ReadCycleStateErr
	}
	return f.CycleState, nil
}

func (f *FakeStorage) WriteCycleState(_ context.Context, cs core.CycleState) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writeCSCalls++
	if f.WriteCycleStateErr != nil {
		return f.WriteCycleStateErr
	}
	if f.WriteCycleStateFailAt > 0 && f.writeCSCalls == f.WriteCycleStateFailAt {
		return errors.New("fixtures: WriteCycleState forced fail at N")
	}
	f.CycleState = cs
	// Defensive copy of the slice — the orchestrator keeps mutating cs.
	csCopy := cs
	csCopy.CompletedPhases = append([]string(nil), cs.CompletedPhases...)
	f.CycleStateLog = append(f.CycleStateLog, csCopy)
	return nil
}

func (f *FakeStorage) AcquireLock(context.Context) (func() error, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.LockErr != nil {
		return nil, f.LockErr
	}
	if f.lockHeld {
		return nil, core.ErrLockHeld
	}
	f.lockHeld = true
	f.LockCount++
	return func() error {
		f.mu.Lock()
		f.lockHeld = false
		f.mu.Unlock()
		return nil
	}, nil
}

// FakeLedger records appended entries and serves them back through a working
// slice-backed iterator (replacing the old "Iter unused" stubs and the bespoke
// sliceIter in cmd/evolve).
type FakeLedger struct {
	mu        sync.Mutex
	Entries   []core.LedgerEntry
	AppendErr error
	VerifyErr error
}

func (f *FakeLedger) Append(_ context.Context, e core.LedgerEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.AppendErr != nil {
		return f.AppendErr
	}
	f.Entries = append(f.Entries, e)
	return nil
}

func (f *FakeLedger) Verify(context.Context) error { return f.VerifyErr }

func (f *FakeLedger) Iter(context.Context) (core.LedgerIterator, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	snap := append([]core.LedgerEntry(nil), f.Entries...)
	return &sliceLedgerIter{entries: snap}, nil
}

type sliceLedgerIter struct {
	entries []core.LedgerEntry
	i       int
}

func (it *sliceLedgerIter) Next() (core.LedgerEntry, bool, error) {
	if it.i >= len(it.entries) {
		return core.LedgerEntry{}, false, nil
	}
	e := it.entries[it.i]
	it.i++
	return e, true, nil
}

func (it *sliceLedgerIter) Close() error { return nil }

// FakeRunner is a no-LLM core.PhaseRunner. It returns Verdict (default PASS),
// records every request, and can model a transient failure for the self-heal
// retry path: the first FailUntil calls return FailErr, then it succeeds.
type FakeRunner struct {
	PhaseName string
	Verdict   string
	FailErr   error
	FailUntil int

	Calls    int
	Requests []core.PhaseRequest
}

func (f *FakeRunner) Name() string { return f.PhaseName }

func (f *FakeRunner) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	f.Calls++
	f.Requests = append(f.Requests, req)
	if f.FailErr != nil && f.Calls <= f.FailUntil {
		return core.PhaseResponse{}, f.FailErr
	}
	v := f.Verdict
	if v == "" {
		v = core.VerdictPASS
	}
	return core.PhaseResponse{Phase: f.PhaseName, Verdict: v, ArtifactsDir: req.Workspace}, nil
}

// orchestratorPhases is the full runnable set the orchestrator may sequence.
var orchestratorPhases = []core.Phase{
	core.PhaseIntent, core.PhaseScout, core.PhaseTriage, core.PhaseTDD,
	core.PhaseBuildPlanner, core.PhaseBuild, core.PhaseAudit, core.PhaseShip, core.PhaseRetro,
}

// BuildRunners returns a runner map for every orchestrator phase. verdicts may
// override individual phase verdicts; a nil/missing entry defaults to PASS.
func BuildRunners(verdicts map[core.Phase]string) map[core.Phase]core.PhaseRunner {
	out := make(map[core.Phase]core.PhaseRunner, len(orchestratorPhases))
	for _, p := range orchestratorPhases {
		out[p] = &FakeRunner{PhaseName: string(p), Verdict: verdicts[p]}
	}
	return out
}

// FakeBridge is an in-memory core.Bridge. It captures the last request and,
// when WriteArtifact is set, materializes it at req.ArtifactPath — mimicking
// what claude-p does so the runner's artifact-poll path exercises real I/O.
type FakeBridge struct {
	Resp          core.BridgeResponse
	Err           error
	ProbeResp        core.BridgeProbe
	WriteArtifact string

	GotReq core.BridgeRequest
}

func (f *FakeBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	f.GotReq = req
	if f.WriteArtifact != "" && req.ArtifactPath != "" {
		_ = os.MkdirAll(filepath.Dir(req.ArtifactPath), 0o755)
		_ = os.WriteFile(req.ArtifactPath, []byte(f.WriteArtifact), 0o644)
		f.Resp.Stdout = f.WriteArtifact
	}
	return f.Resp, f.Err
}

func (f *FakeBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return f.ProbeResp, nil
}
