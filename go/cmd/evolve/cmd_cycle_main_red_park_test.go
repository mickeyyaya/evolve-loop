package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/loopchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func mainRedPark() error {
	dup := []baseattr.Key{{Package: "internal/dup", Test: "TestCount"}}
	se := core.NewShipError(shiperr.CodeMainRedPark, core.ShipClassPrecondition, core.StageAtomicShip, "red on main",
		baseattr.Verdict{BaseRed: dup, MainRed: dup, Main: "9a2bbe0e1c0ffee"}.Debug()...)
	return fmt.Errorf("phase ship: %w", se)
}

type shipParkRunner struct{}

func (shipParkRunner) Name() string { return "ship" }
func (shipParkRunner) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	if req.PreviousPhase == string(core.PhaseAudit) {
		return core.PhaseResponse{}, mainRedPark()
	}
	return noopRunner{name: string(core.PhaseBuild)}.Run(ctx, req)
}

type parkedResumeRunner struct{}

func (parkedResumeRunner) RunCycle(context.Context, core.CycleRequest) (core.CycleResult, error) {
	return core.CycleResult{}, errors.New("RunCycle is not a resume")
}

func (parkedResumeRunner) RunCycleFromPhase(context.Context, core.CycleRequest, *core.ResumePoint) (core.CycleResult, error) {
	return core.CycleResult{Cycle: 7}, mainRedPark()
}

func (parkedResumeRunner) SignalSummary() signalcenter.Summary { return signalcenter.Summary{} }

func assertClaimKept(t *testing.T, claim, pending string) {
	t.Helper()
	if _, err := os.Stat(claim); err != nil {
		t.Errorf("a parked cycle lost the claim that its resume needs: %v", err)
	}
	if _, err := os.Stat(pending); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a parked cycle put its item back in the queue: %v", err)
	}
}

func TestCycleRunMainRedExit_AFleetLaneReleasesItsClaimAndChargesNobody(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	claim, pending := claimedInboxItem(t, root, 2249, "parked-fleet-lane-releases-its-claim")
	lifecycle := newFakeLedger()
	var stdout, stderr bytes.Buffer

	code := cycleRunMainRedExit(2249, root, true, &stdout, &stderr, lifecycle, nil)

	if code != fleet.ExitDeferred {
		t.Errorf("exit = %d, want fleet.ExitDeferred=%d", code, fleet.ExitDeferred)
	}
	if _, err := os.Stat(claim); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the parked lane still holds its claim, so no lane can adopt the cycle: %v", err)
	}
	body, err := os.ReadFile(pending)
	if err != nil {
		t.Fatalf("the item is not back in the queue: %v", err)
	}
	if strings.Contains(string(body), "failure_count") {
		t.Errorf("a main-red park charged the task: %s", body)
	}
	if !lifecycleNotes(lifecycle, laneMainRedParkedReason) || lifecycleNotes(lifecycle, laneWallDeferredReason) {
		t.Errorf("the lifecycle record does not name only %q: %+v", laneMainRedParkedReason, lifecycle.lifecycle)
	}
	if !strings.Contains(stderr.String(), "cycle 2249 claims are pending again (lane-main-red-parked)") || strings.Contains(stderr.String(), "evolve loop --resume") {
		t.Errorf("stderr does not tell the operator that the pool resumes the lane: %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), `"main-red-parked"`) {
		t.Errorf("the stdout envelope does not carry the main-red-parked stop reason: %q", stdout.String())
	}
}

func TestCycleRunMainRedExit_ANonFleetCycleKeepsItsClaimForResume(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	claim, pending := claimedInboxItem(t, root, 2250, "sequential-park-keeps-claim")
	lifecycle := newFakeLedger()
	var stdout, stderr bytes.Buffer

	code := cycleRunMainRedExit(2250, root, false, &stdout, &stderr, lifecycle, nil)

	if code != fleet.ExitDeferred || len(lifecycle.lifecycle) != 0 {
		t.Errorf("exit = %d with lifecycle %+v, want fleet.ExitDeferred and no release", code, lifecycle.lifecycle)
	}
	assertClaimKept(t, claim, pending)
	if !strings.Contains(stderr.String(), "evolve cycle run: cycle 2250 is parked on a red main") || !strings.Contains(stderr.String(), "evolve loop --resume") {
		t.Errorf("stderr does not name the park and the resume path: %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), `"main-red-parked"`) {
		t.Errorf("the stdout envelope does not carry the main-red-parked stop reason: %q", stdout.String())
	}
}

func TestCycleRunMainRedExit_AReleaseErrorIsReportedAndTheLaneStillDefers(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 directory, so the release cannot fail")
	}
	root := t.TempDir()
	processing := filepath.Join(root, ".evolve", "inbox", "processing")
	if err := os.MkdirAll(processing, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(processing, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(processing, 0o755) })
	var stdout, stderr bytes.Buffer

	code := cycleRunMainRedExit(2251, root, true, &stdout, &stderr, newFakeLedger(), nil)

	if code != fleet.ExitDeferred {
		t.Errorf("exit = %d, want fleet.ExitDeferred=%d", code, fleet.ExitDeferred)
	}
	if !strings.Contains(stderr.String(), "could not release cycle 2251 claims") || strings.Contains(stderr.String(), "pending again") {
		t.Errorf("the release error was swallowed: %q", stderr.String())
	}
}

func TestCycleRunRoot_AFleetLaneParkedOnARedMainReleasesItsClaimAndChargesNobody(t *testing.T) {
	t.Setenv(ipcenv.FleetKey, "1")
	var claim, pending string
	rc, fake, _, stderr := runCycleRunWith(t, shipParkRunner{}, false, func(root string) {
		claim, pending = claimedInboxItem(t, root, 1, "parked-lane-item")
	})
	if rc != fleet.ExitDeferred {
		t.Fatalf("rc = %d, want fleet.ExitDeferred; stderr=%s", rc, stderr)
	}
	if _, err := os.Stat(claim); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the parked fleet lane still holds its claim: %v; stderr=%s", err, stderr)
	}
	body, err := os.ReadFile(pending)
	if err != nil || strings.Contains(string(body), "failure_count") {
		t.Errorf("the parked item is not pending again with no charge: %v %s", err, body)
	}
	if !lifecycleNotes(fake, laneMainRedParkedReason) {
		t.Errorf("no lifecycle record names the main-red park: %+v", fake.lifecycle)
	}
}

func TestCycleRunRoot_ACycleRunOutsideAFleetKeepsItsClaimOnAMainRedPark(t *testing.T) {
	t.Setenv(ipcenv.FleetKey, "")
	var claim, pending string
	rc, fake, _, stderr := runCycleRunWith(t, shipParkRunner{}, false, func(root string) {
		claim, pending = claimedInboxItem(t, root, 1, "manual-parked-item")
	})
	if rc != fleet.ExitDeferred || lifecycleNotes(fake, "cycle-failure-release") || lifecycleNotes(fake, laneMainRedParkedReason) {
		t.Fatalf("rc = %d with lifecycle %+v, want fleet.ExitDeferred and no release; stderr=%s", rc, fake.lifecycle, stderr)
	}
	assertClaimKept(t, claim, pending)
}

func TestHandleCycleError_ASequentialMainRedParkStopsTheBatchAndChargesNobody(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := seedClaimedCycle(t, root)
	ledger := newFakeLedger()
	c, got := recordedCenter()
	var stdout, stderr bytes.Buffer
	b := &loopBatchCoordinator{cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, deps: orchDeps{Ledger: ledger, Signals: c}, result: &loopResult{}, stdout: &stdout, stderr: &stderr}

	d := b.handleCycleError(core.CycleResult{Cycle: 7}, mainRedPark())

	if d.flow != batchReturn || d.exitCode != fleet.ExitDeferred || b.result.StopReason != mainRedParkedStopReason {
		t.Fatalf("decision = %+v with stop reason %q, want a return with exit %d and %q", d, b.result.StopReason, fleet.ExitDeferred, mainRedParkedStopReason)
	}
	if len(ledger.lifecycle) != 0 || b.result.RecoverableFailures != 0 || len(*got) != 0 {
		t.Errorf("a park ran the failure walk: lifecycle %+v, failures %d, signals %+v", ledger.lifecycle, b.result.RecoverableFailures, *got)
	}
	if !strings.Contains(stderr.String(), "evolve loop: cycle 7 is parked on a red main") {
		t.Errorf("stderr does not name the park: %q", stderr.String())
	}
}

func TestRunResumeBatch_AResumedCycleThatParksOnARedMainIsNotAFailedCycle(t *testing.T) {
	root, evolveDir := seedQuotaPause(t, 0)
	ledger := newFakeLedger()
	var out, errOut bytes.Buffer
	lr := loopResult{}
	cfg := loopConfig{ProjectRoot: root, EvolveDir: evolveDir, Resume: true}

	rc := runResumeBatch(context.Background(), cfg, parkedResumeRunner{}, ledger, nil, map[string]string{}, map[string]string{}, &lr, &out, &errOut)

	if rc != fleet.ExitDeferred || lr.StopReason != mainRedParkedStopReason {
		t.Fatalf("rc = %d with stop reason %q, want exit %d and %q\n%s", rc, lr.StopReason, fleet.ExitDeferred, mainRedParkedStopReason, errOut.String())
	}
	if len(ledger.lifecycle) != 0 {
		t.Errorf("a park ran the failure walk: %+v", ledger.lifecycle)
	}
	if !strings.Contains(errOut.String(), "evolve loop: resume: cycle 7 is parked on a red main") {
		t.Errorf("stderr does not name the park: %q", errOut.String())
	}
}

func TestRunLoopChain_ABatchThatParksOnARedMainGivesAChainRecordThatNamesThePark(t *testing.T) {
	cfg := chainTestEnv(t, 3, "")
	seen := stubBatches(t, func(_ int, c loopConfig) int {
		cs := core.CycleState{CycleID: 7, RunID: "parked-run", Phase: "ship", WorkspacePath: cycleWorkspace(c.ProjectRoot, 7)}
		if err := storage.New(c.EvolveDir).WriteCycleState(context.Background(), cs); err != nil {
			t.Fatal(err)
		}
		if err := core.MainRedBoundaryCheckpointer(cs, c.ProjectRoot, time.Now()); err != nil {
			t.Fatal(err)
		}
		return fleet.ExitDeferred
	})

	rc, res, stderr := runChain(t, cfg, policy.ChainConfig{Enabled: true, MaxBatches: 10})

	if rc != fleet.ExitDeferred || res.StopReason != loopchain.StopQuotaDefer || len(*seen) != 1 {
		t.Fatalf("rc = %d, stop reason %q, %d batches; want one deferred batch", rc, res.StopReason, len(*seen))
	}
	if !strings.Contains(stderr, "batch 1 ended at a main-red park: the cycle is parked at ship and its lane is not blamed (cycle=7 wake-at= source=main-red)") {
		t.Errorf("the chain record does not name the park: %s", stderr)
	}
	if strings.Contains(stderr, "a wall") || strings.Contains(stderr, "no cause was recorded") {
		t.Errorf("the chain reports a park as a wall: %s", stderr)
	}
	if !strings.Contains(stderr, "next: evolve loop --resume, when main is green.") {
		t.Errorf("the chain does not give the resume step of a park: %s", stderr)
	}
	signals, err := os.ReadFile(filepath.Join(cfg.EvolveDir, "signals.ndjson"))
	if err != nil || !strings.Contains(string(signals), `"pause":"a main-red park"`) {
		t.Errorf("the durable signal does not carry the pause kind: %v %s", err, signals)
	}
}

func TestChainQuotaPause_AWallCheckpointIsStillReportedAsAWall(t *testing.T) {
	_, evolveDir := seedQuotaPause(t, 0)

	got, ok := chainQuotaPause(evolveDir)

	if !ok || got.Kind != loopchain.PauseWall || got.Cycle != 7 {
		t.Errorf("pause = %+v ok=%v, want a wall for cycle 7", got, ok)
	}
}
