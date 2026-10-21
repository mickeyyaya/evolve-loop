package core

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipwindow"
)

type windowView struct {
	held, marked bool
}

func shipWindowHeldAt(root string) bool {
	_, err := os.Stat(shipwindow.PathIn(filepath.Join(root, ".evolve")))
	return err == nil
}

func TestRecoverFromShipError_HoldsTheShipWindowFromTheRebaseThroughTheReShip(t *testing.T) {
	t.Parallel()
	lane := newStaleBaseLane(t, "peer.txt")
	audited, exit, err := gitCapture(context.Background(), lane.repo.Dir, "diff", lane.base, lane.laneTip)
	if err != nil || exit != 0 {
		t.Fatalf("audited diff: exit=%d err=%v", exit, err)
	}
	patchID, err := compositionPatchID([]byte(audited))
	if err != nil {
		t.Fatal(err)
	}
	var gates []bool
	var ships []windowView
	out := lane.resumeShip(t, staleBaseScenario{shipErr: landingDiverged(), shipFailures: 1,
		observeShip: func(ctx context.Context) {
			ships = append(ships, windowView{held: shipWindowHeldAt(lane.root), marked: shipwindow.Held(ctx)})
		},
		options: []Option{
			WithCompositionSnapshot(snapshotFromFixture([]byte(audited), patchID)),
			WithScopedMergeReviewer(func([]MergeHunk, string, string) ScopedMergeReviewOutcome {
				return ScopedMergeReviewOutcome{Disposition: ScopedMergeCompatible}
			}),
			WithCompositionGateRunner(func(context.Context, string) map[string]ciparity.GateOutcome {
				gates = append(gates, shipWindowHeldAt(lane.root))
				return allComposedGatesPass()
			}),
			WithCompositionVerdictWriter(func(string, CompositionVerdictInput) error { return nil }),
		}})

	if out.err != nil {
		t.Fatalf("precondition: the proven rebase ships, got %v", out.err)
	}
	requireOrder(t, out.order, "ship", "ship")
	if len(gates) != 1 || !gates[0] {
		t.Fatalf("composed-tree gates saw the ship window held = %v, want one gate run inside the window", gates)
	}
	if ships[0].held || ships[0].marked {
		t.Fatalf("the first ship saw %+v; the orchestrator holds no window before a recovery", ships[0])
	}
	if !ships[1].held || !ships[1].marked {
		t.Fatalf("the re-ship saw %+v, want the orchestrator's window held and marked so ship re-enters it", ships[1])
	}
	if shipWindowHeldAt(lane.root) {
		t.Fatal("the cycle ended with the ship window still held")
	}
}

func TestRecoverFromShipError_ARebaseRoutedToAuditReleasesTheShipWindow(t *testing.T) {
	t.Parallel()
	lane := newStaleBaseLane(t, "peer.txt")
	var audits []windowView
	out := lane.resumeShip(t, staleBaseScenario{shipErr: landingDiverged(), shipFailures: 1,
		observeAudit: func(ctx context.Context) {
			audits = append(audits, windowView{held: shipWindowHeldAt(lane.root), marked: shipwindow.Held(ctx)})
		}})

	if out.err != nil {
		t.Fatalf("precondition: the re-audited rebase ships, got %v", out.err)
	}
	requireOrder(t, out.order, "ship", "audit", "ship")
	if len(audits) != 1 || audits[0].held || audits[0].marked {
		t.Fatalf("the re-audit saw %+v; an LLM phase must never hold the ship window", audits)
	}
	if shipWindowHeldAt(lane.root) {
		t.Fatal("the cycle ended with the ship window still held")
	}
}

func TestRunPhase_OnlyTheShipRunsInsideAHeldWindowAndReleasesIt(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	o.holdShipWindow(context.Background(), root, 3, "run-3")
	var marked []bool
	probe := windowProbe(func(ctx context.Context) { marked = append(marked, shipwindow.Held(ctx)) })

	if _, err := o.runPhase(context.Background(), PhaseAudit, probe, PhaseRequest{}); err != nil {
		t.Fatal(err)
	}
	if !shipWindowHeldAt(root) {
		t.Fatal("a non-ship phase released the window that the coming re-ship needs")
	}
	if _, err := o.runPhase(context.Background(), PhaseShip, probe, PhaseRequest{}); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(marked) != "[false true]" {
		t.Fatalf("held marks per dispatch = %v, want [false true]: only the ship enters the window", marked)
	}
	if shipWindowHeldAt(root) {
		t.Fatal("the ship dispatch returned with the window still held")
	}
}

func TestHoldShipWindow_ACanceledWaitRecoversUnleasedWithAWarn(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sibling, err := shipwindow.Acquire(context.Background(), filepath.Join(root, ".evolve"), shipwindow.Options{OwnerPID: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sibling.Release() }()
	var stderr bytes.Buffer
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithStderr(&stderr))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	o.holdShipWindow(ctx, root, 3, "run-3")

	if !strings.Contains(stderr.String(), "[orchestrator] WARN ship-window lease not acquired (context canceled); the recovery proceeds unleased") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if shipwindow.Held(o.shipWindowContext(context.Background())) {
		t.Fatal("an unleased recovery marks the re-ship as held")
	}
	o.releaseShipWindow()
	if !shipWindowHeldAt(root) {
		t.Fatal("the unleased orchestrator released the sibling's lease")
	}
}

func TestHoldShipWindow_BreakingAHungHolderSignals(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	lock := shipwindow.PathIn(filepath.Join(root, ".evolve"))
	if err := os.MkdirAll(filepath.Dir(lock), 0o755); err != nil {
		t.Fatal(err)
	}
	stale := time.Now().Add(-2 * shipwindow.DefaultTTL).UTC().Format(time.RFC3339Nano)
	if err := os.WriteFile(lock, []byte(fmt.Sprintf(`{"owner_pid":%d,"heartbeat_at":%q,"token":"hung"}`+"\n", os.Getpid(), stale)), 0o644); err != nil {
		t.Fatal(err)
	}
	center, events := recordingCenter()
	var stderr bytes.Buffer
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(center), WithStderr(&stderr))

	o.holdShipWindow(context.Background(), root, 9, "run-9")
	defer o.releaseShipWindow()
	center.Flush()

	found := false
	for _, e := range *events {
		found = found || (e.Code == CodeShipWindowLeaseBroken && e.Cycle == 9 && e.Fields["owner_pid"] == strconv.Itoa(os.Getpid()) && e.Fields["heartbeat_at"] == stale)
	}
	if !found {
		t.Fatalf("signals = %+v, want %s naming the broken holder", *events, CodeShipWindowLeaseBroken)
	}
	if !strings.Contains(stderr.String(), fmt.Sprintf("[orchestrator] WARN ship-window lease of pid %d broken at the TTL", os.Getpid())) {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestReleaseShipWindow_AReleaseFailureWarnsAndClearsTheHold(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	var stderr bytes.Buffer
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithStderr(&stderr))
	o.holdShipWindow(context.Background(), root, 3, "run-3")
	o.holdShipWindow(context.Background(), root, 3, "run-3")
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.RemoveAll(evolveDir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evolveDir, []byte("now a file"), 0o644); err != nil {
		t.Fatal(err)
	}

	o.releaseShipWindow()

	if !strings.Contains(stderr.String(), "[orchestrator] WARN ship-window lease release: ") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if shipwindow.Held(o.shipWindowContext(context.Background())) {
		t.Fatal("a failed release left the hold in place")
	}
}

func TestKeepShipWindowOnlyForShip_KeepsTheHoldOnlyForAReShip(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		next Phase
		ok   bool
		keep bool
	}{{PhaseShip, true, true}, {PhaseAudit, true, false}, {PhaseBuild, true, false}, {PhaseShip, false, false}, {"", false, false}} {
		root := t.TempDir()
		o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
		o.holdShipWindow(context.Background(), root, 3, "run-3")
		next, ok := o.keepShipWindowOnlyForShip(tc.next, tc.ok)
		if next != tc.next || ok != tc.ok {
			t.Fatalf("route (%s,%v) came back as (%s,%v)", tc.next, tc.ok, next, ok)
		}
		if got := shipWindowHeldAt(root); got != tc.keep {
			t.Errorf("route (%s,%v): window held = %v, want %v", tc.next, tc.ok, got, tc.keep)
		}
		o.releaseShipWindow()
	}
}

type windowProbe func(context.Context)

func (p windowProbe) Name() string { return "probe" }

func (p windowProbe) Run(ctx context.Context, _ PhaseRequest) (PhaseResponse, error) {
	p(ctx)
	return PhaseResponse{Verdict: VerdictPASS}, nil
}
