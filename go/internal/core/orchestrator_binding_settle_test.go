package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/defectledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

type settleCall struct {
	cycle         int
	workspace     string
	triageCalls   int
	buildCalls    int
	scopeAtSettle []string
}

func TestRunCycle_FreshCycleSettlesRegistryBindingAfterTriage(t *testing.T) {
	t.Parallel()
	root, _ := initContinuationRepo(t, 1090)
	runners := buildRunners(nil)
	triage := runners[PhaseTriage].(*fakeRunner)
	build := runners[PhaseBuild].(*fakeRunner)
	var calls []settleCall
	storage := &fakeStorage{}
	o := NewOrchestrator(storage, &fakeLedger{}, runners,
		WithContinuationResolver(func(string, int, []string) *continuation.Continuation { return nil }),
		WithBindingSettler(func(cycle int, ws string) *continuation.Continuation {
			calls = append(calls, settleCall{cycle: cycle, workspace: ws, triageCalls: triage.calls, buildCalls: build.calls, scopeAtSettle: LaneScopeIDs(ws)})
			return nil
		}))

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g", Env: map[string]string{ipcenv.FleetScopeKey: "scope-s"}})

	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("settler calls = %d; want 1", len(calls))
	}
	got := calls[0]
	if got.cycle != res.Cycle || got.workspace != storage.cycleState.WorkspacePath || got.workspace == "" {
		t.Errorf("settler got cycle %d, workspace %q; want cycle %d, workspace %q", got.cycle, got.workspace, res.Cycle, storage.cycleState.WorkspacePath)
	}
	if got.triageCalls != 1 || got.buildCalls != 0 {
		t.Errorf("settler ran with triage calls %d, build calls %d; want after triage and before build", got.triageCalls, got.buildCalls)
	}
	if len(got.scopeAtSettle) != 1 || got.scopeAtSettle[0] != "scope-s" {
		t.Errorf("lane pin at settle = %v; want [scope-s]", got.scopeAtSettle)
	}
}

func TestWithBindingSettler_IgnoresANilFunction(t *testing.T) {
	t.Parallel()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithBindingSettler(nil))
	if o.bindingSettler != nil {
		t.Error("a nil settler must stay unset")
	}
}

var errInjectedSeeding = errors.New("injected seeding failure")

type failingSeedProvisioner struct{ gitWorktree }

func (failingSeedProvisioner) CreateFrom(string, int, string) (string, error) {
	return "", errInjectedSeeding
}

func TestRunCycle_NoSettlerWhenAdoptionFails(t *testing.T) {
	t.Parallel()
	root, wt := initContinuationRepo(t, 86)
	stampedContinuation(t, root, wt, 86)
	seedStampedInboxItem(t, root, 86, "task-a")
	runners := buildRunners(nil)
	runners[PhaseTriage] = &claimingTriageRunner{fakeRunner: runners[PhaseTriage].(*fakeRunner), root: root, taskID: "task-a"}
	settled := 0
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners,
		WithContinuationResolver(productionResolver(t)),
		WithWorktreeProvisioner(failingSeedProvisioner{}),
		WithBindingSettler(func(int, string) *continuation.Continuation { settled++; return nil }))

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})

	if !errors.Is(err, errInjectedSeeding) {
		t.Fatalf("RunCycle err = %v; want the adoption failure", err)
	}
	if settled != 0 {
		t.Errorf("settler calls = %d; want 0 after a failed adoption", settled)
	}
}

func recordedAncestor(t *testing.T, root string) *continuation.Continuation {
	t.Helper()
	ancestorWS := RunWorkspacePath(root, 77)
	if err := os.MkdirAll(ancestorWS, 0o755); err != nil {
		t.Fatal(err)
	}
	ledger := `{"origin_cycle":77,"entries":[{"id":"d1","text":"first defect","status":"OPEN"}]}`
	if err := os.WriteFile(filepath.Join(ancestorWS, defectledger.LedgerFile), []byte(ledger), 0o644); err != nil {
		t.Fatal(err)
	}
	findings := filepath.Join(ancestorWS, "findings.txt")
	if err := os.WriteFile(findings, []byte("inherited finding text"), 0o644); err != nil {
		t.Fatal(err)
	}
	return &continuation.Continuation{Cycle: 77, FindingsPath: findings}
}

func TestRunCycle_RecordedSettlementServesTheInheritedDebtToTheBuilder(t *testing.T) {
	t.Parallel()
	for _, recorded := range []bool{true, false} {
		name := "nil result seeds nothing"
		if recorded {
			name = "recorded result seeds the skeleton and the findings"
		}
		t.Run(name, func(t *testing.T) {
			root, _ := initContinuationRepo(t, 1091)
			ancestor := recordedAncestor(t, root)
			runners := buildRunners(nil)
			build := runners[PhaseBuild].(*fakeRunner)
			o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners,
				WithBindingSettler(func(int, string) *continuation.Continuation {
					if recorded {
						return ancestor
					}
					return nil
				}))

			if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g", Env: map[string]string{ipcenv.FleetScopeKey: "scope-s"}}); err != nil {
				t.Fatalf("RunCycle: %v", err)
			}

			req := build.requests[0]
			_, statErr := os.Stat(filepath.Join(req.Workspace, defectledger.DispositionsFile))
			findings := req.Context["continuation_findings"]
			if recorded {
				if statErr != nil {
					t.Errorf("dispositions skeleton missing: %v", statErr)
				}
				if !strings.Contains(findings, "inherited finding text") {
					t.Errorf("builder findings = %q; want the inherited findings", findings)
				}
				return
			}
			if statErr == nil || findings != "" {
				t.Errorf("a nil settlement seeded a skeleton (%v) or findings %q", statErr, findings)
			}
		})
	}
}

func TestBindingSettleProbe_ReportsWhetherASettlerIsWired(t *testing.T) {
	t.Parallel()
	if _, wired := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil)).BindingSettleProbe(1, "/ws"); wired {
		t.Error("no settler option means not wired")
	}
	want := &continuation.Continuation{Cycle: 7}
	var gotCycle int
	var gotWS string
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithBindingSettler(func(cycle int, ws string) *continuation.Continuation {
		gotCycle, gotWS = cycle, ws
		return want
	}))
	got, wired := o.BindingSettleProbe(3, "/ws")
	if !wired || got != want || gotCycle != 3 || gotWS != "/ws" {
		t.Errorf("got %+v wired %v cycle %d ws %q; the probe must call the settler with its arguments", got, wired, gotCycle, gotWS)
	}
}
