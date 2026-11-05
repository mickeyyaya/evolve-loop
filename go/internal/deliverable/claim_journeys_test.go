package deliverable

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

func productionLaneMenu() core.Option {
	return core.WithLaneMenu(func(inboxRoot string, items []inboxbatch.Item) []inboxbatch.Item {
		return inboxmover.PartitionLaneMenu(inboxmover.Options{InboxDir: inboxRoot, Stderr: io.Discard}, items, nil).Ready
	})
}

func writeJourneyItem(t *testing.T, root, name, body string) {
	t.Helper()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func journeyTriage(t *testing.T, deferFrom int, pin bool) (map[core.Phase]core.PhaseRunner, *stubPhase) {
	t.Helper()
	runners, triage := effectRunners(t, 0, true)
	commit := triage.onRun
	triage.onRun = func(n int, req core.PhaseRequest) {
		commit(n, req)
		if pin && n == 1 {
			if err := committedset.WriteRecord(req.ProjectRoot, committedset.Record{Cycle: req.Cycle, ItemIDs: []string{"x"}, GoalHash: "g"}); err != nil {
				t.Fatal(err)
			}
		}
		if deferFrom > 0 && n >= deferFrom {
			body := `{"cycle":` + strconv.Itoa(req.Cycle) + `,"top_n":[],"deferred":[{"id":"x"}]}`
			if err := os.WriteFile(filepath.Join(req.Workspace, "triage-decision.json"), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return runners, triage
}

func claimerFailing(t *testing.T, failures int32) core.Option {
	t.Helper()
	var calls atomic.Int32
	claim := func(inboxDir string, cycle int, ids []string) error {
		if calls.Add(1) <= failures {
			return errors.New("disk full")
		}
		return inboxmover.ClaimPending(inboxmover.Options{InboxDir: inboxDir, Stderr: io.Discard, Ledger: ledger.New(filepath.Dir(inboxDir))}, cycle, ids)
	}
	return core.WithHostEffects(NewHostEffects(effectCatalog(t), claim, nil))
}

func runJourney(t *testing.T, root string, runners map[core.Phase]core.PhaseRunner, opts ...core.Option) (core.CycleResult, error) {
	t.Helper()
	o := effectOrchestrator(t, root, runners, append(opts, productionLaneMenu())...)
	return o.RunCycle(context.Background(), core.CycleRequest{ProjectRoot: root, GoalHash: "g", DisableWorkspaceGuard: true})
}

func TestJourney_ADependencyWaitAnsweredByDeferralEndsAsPlannedNoWork(t *testing.T) {
	root := gitRepoWithOneCommit(t)
	writeJourneyItem(t, root, "2026-09-12T00-00-00Z-x.json", `{"id":"x","title":"fixture","weight":0.9,"deps":["y"]}`)
	writeJourneyItem(t, root, "2026-09-12T00-00-01Z-y.json", `{"id":"y","title":"first","weight":0.5}`)
	runners, triage := journeyTriage(t, 2, true)

	result, err := runJourney(t, root, runners, claimerFailing(t, 0))

	if err != nil {
		t.Fatalf("a deferral is the answer the gate names: %v", err)
	}
	if len(triage.requests) != 2 || !strings.Contains(triage.requests[1].CorrectionDirective, `committed item "x" waits on the dependency "y", so the host did not claim it.`) {
		t.Fatalf("one correction names the dependency: %q", directives(triage.requests))
	}
	if strings.Contains(triage.requests[1].CorrectionDirective, "ORCHESTRATOR_HOST_EFFECT_FAILED") {
		t.Fatalf("a dependency wait is not a host fault: %q", directives(triage.requests))
	}
	if !core.IsTriageNoWorkResult(result) {
		t.Fatalf("want planned no-work, got verdict=%q termination=%q phases=%v", result.FinalVerdict, result.TerminationReason, result.PhasesRun)
	}
	for _, p := range []core.Phase{core.PhaseBuild, core.PhaseShip} {
		if n := len(runners[p].(*stubPhase).requests); n != 0 {
			t.Errorf("%s ran %d time(s)", p, n)
		}
	}
}

func TestJourney_AFailedClaimerRepeatingTheSameAnswerIsRejectedAfterThreeRuns(t *testing.T) {
	root := gitRepoWithOneCommit(t)
	seedPendingItem(t, root)
	runners, triage := journeyTriage(t, 0, false)

	_, err := runJourney(t, root, runners, claimerFailing(t, 99))

	if err == nil {
		t.Fatal("a commitment that the host cannot claim must not pass")
	}
	if len(triage.requests) != 3 {
		t.Fatalf("a host fault consumes corrections: want 3 runs, got %d: %q", len(triage.requests), directives(triage.requests))
	}
	want := `review gate: the host tried to route and claim committed item "x", and neither landed. If the item can be claimed, an answer of deferred ends this cycle as FAIL. Next: the console reads the error with evolve signals tail --cycle 1 --code RUNNER_HOST_EFFECT_FAILED,ORCHESTRATOR_HOST_EFFECT_FAILED.`
	for _, r := range triage.requests[1:] {
		if !strings.Contains(r.CorrectionDirective, want) {
			t.Fatalf("directive must carry the host-fault text: %q", r.CorrectionDirective)
		}
	}
}

func TestJourney_AFailedClaimerThenDeferredIsAFailNamingClaimableWork(t *testing.T) {
	root := gitRepoWithOneCommit(t)
	seedPendingItem(t, root)
	runners, triage := journeyTriage(t, 2, false)

	result, err := runJourney(t, root, runners, claimerFailing(t, 99))

	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if len(triage.requests) != 2 {
		t.Fatalf("one correction, then the deferral: %q", directives(triage.requests))
	}
	if result.TerminationReason != "triage-empty-commitment-claimable-work" || result.FinalVerdict == core.VerdictPASS || core.IsTriageNoWorkResult(result) {
		t.Fatalf("a deferral over claimable work is never planned no-work: verdict=%q termination=%q", result.FinalVerdict, result.TerminationReason)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".evolve", "inbox", pendingItemFile)); statErr != nil {
		t.Fatalf("the item stays pending: %v", statErr)
	}
}

func TestJourney_AShortHostFaultHealsOnTheCorrectionRun(t *testing.T) {
	root := gitRepoWithOneCommit(t)
	seedPendingItem(t, root)
	runners, triage := journeyTriage(t, 0, false)

	if _, err := runJourney(t, root, runners, claimerFailing(t, 1)); err != nil {
		t.Fatalf("each correction runs the host claim again, so a short fault heals: %v", err)
	}
	if len(triage.requests) != 2 {
		t.Fatalf("want 2 triage runs, got %d: %q", len(triage.requests), directives(triage.requests))
	}
	if ship := runners[core.PhaseShip].(*stubPhase); len(ship.requests) != 1 {
		t.Fatalf("the healed cycle ships once, got %d", len(ship.requests))
	}
}
