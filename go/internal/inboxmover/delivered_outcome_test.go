package inboxmover

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestDeliveredJourney_TwoDeliveredMembersStayOutOfASecondLane(t *testing.T) {
	opts, root := claimsFixture(t)
	opts.Ledger = &recordingAppender{}
	writeHolderRun(t, root, holderRun{cycle: 1828, goal: "g", phase: "ship"})
	ids := []string{"member-a", "member-b"}
	for _, id := range ids {
		claimItem(t, root, 1828, id)
	}

	res, err := ApplyCycleOutcome(opts, CycleOutcome{Cycle: 1828, Passed: true, Delivered: true, CommittedIDs: ids})
	if err != nil || len(res.Promoted) != 2 || len(res.Released) != 0 {
		t.Fatalf("delivered outcome = %+v, %v; want both moved to delivered and none released", res, err)
	}
	for _, id := range ids {
		mustExist(t, deliveredFile(root, id), "delivered record of "+id)
		item := inboxbatch.Item{ID: id}
		if place, reason := PlaceOnLaneMenu(opts, item, nil); place != MenuWaiting || !strings.Contains(reason, "delivery") {
			t.Errorf("menu place of %s = %v %q, want waiting on its delivery", id, place, reason)
		}
		if _, err := ClaimDispatchable(opts, id, "1900"); err == nil {
			t.Errorf("a second lane claimed %s", id)
		}
	}
	released, err := ReleaseStaleClaims(opts, "boundary")
	if err != nil || len(released) != 0 {
		t.Errorf("sweep = %+v, %v; want nothing released", released, err)
	}
	for _, id := range ids {
		mustExist(t, deliveredFile(root, id), "the sweep keeps "+id)
	}
}

func TestApplyCycleOutcome_DeliveredDrainsTheResidualAndRefusesAForeignClaim(t *testing.T) {
	opts, root := claimsFixture(t)
	opts.Ledger = &recordingAppender{}
	claimItem(t, root, 1828, "mine")
	claimItem(t, root, 1828, "residual")
	claimItem(t, root, 1829, "foreign")

	res, err := ApplyCycleOutcome(opts, CycleOutcome{Cycle: 1828, Passed: true, Delivered: true, CommittedIDs: []string{"mine", "foreign", "never-filed"}})
	if err != nil {
		t.Fatalf("ApplyCycleOutcome = %v", err)
	}
	if len(res.Promoted) != 1 || res.Promoted[0] != "mine" || len(res.Released) != 1 {
		t.Errorf("result = %+v, want mine delivered and the residual released", res)
	}
	mustExist(t, filepath.Join(root, ".evolve", "inbox", "processing", "cycle-1829", "2026-09-30T11-07-44Z-foreign.json"), "a foreign claim stays")
}

func TestApplyCycleOutcome_DeliveredReportsAFailedMove(t *testing.T) {
	opts, root := claimsFixture(t)
	opts.Ledger = &recordingAppender{}
	claimItem(t, root, 1828, "mine")
	writeFile(t, filepath.Join(root, ".evolve", "inbox", "delivered"), "not a dir")

	_, err := ApplyCycleOutcome(opts, CycleOutcome{Cycle: 1828, Passed: true, Delivered: true, CommittedIDs: []string{"mine"}})
	if !errors.Is(err, errDeliveryFailed) || errors.Is(err, errDeliveryKept) {
		t.Errorf("ApplyCycleOutcome = %v, want a failed delivery that the hold kept out of the drain", err)
	}
}

func TestApplyCycleOutcome_DeliveredReportsAFailedResidualDrain(t *testing.T) {
	root := t.TempDir()
	seedInboxItemF4(t, root, "a")
	blockInboxDirF4(t, root, "processing")

	res, err := ApplyCycleOutcome(outcomeOptsF4(root), CycleOutcome{Cycle: 7, Passed: true, Delivered: true, CommittedIDs: []string{"a"}})

	if !errors.Is(err, errResidualDrain) || errors.Is(err, errDeliveryFailed) || len(res.Promoted) != 1 {
		t.Errorf("ApplyCycleOutcome = %+v, %v; want a residual drain error after one delivery", res, err)
	}
}

func TestApplyCycleOutcome_DeliveredFailsOnAnUnresolvableRoot(t *testing.T) {
	if _, err := ApplyCycleOutcome(Options{}, CycleOutcome{Passed: true, Delivered: true}); err == nil {
		t.Error("an options value with no root must fail")
	}
}

func TestReportOrphanDelivered_ReportsEachOrphanOnceAndKeepsIt(t *testing.T) {
	opts, root, _ := signalledOptions(t)
	all := collectSignals(opts)
	writeFile(t, deliveredFile(root, "in-wave"), `{"id":"in-wave"}`)
	writeFile(t, deliveredFile(root, "orphan"), `{"id":"orphan"}`)
	writeFile(t, filepath.Join(root, ".evolve", "inbox", "delivered", "notes.txt"), "x")
	writeFile(t, filepath.Join(root, ".evolve", "inbox", "delivered", "broken.json"), "{broken")
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox", "delivered", "sub.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	reported := map[string]bool{}
	inWave := func(id string) bool { return id == "in-wave" }

	first, err := ReportOrphanDelivered(opts, inWave, reported)
	second, err2 := ReportOrphanDelivered(opts, inWave, reported)

	if err != nil || err2 != nil || len(first) != 1 || first[0] != "orphan" || len(second) != 0 {
		t.Fatalf("reports = %v %v, %v %v; want orphan once", first, second, err, err2)
	}
	events := eventsWithCode(*all, "INBOX_DELIVERED_ORPHAN")
	if len(events) != 1 || events[0].Severity != signalcenter.SeverityIncident || events[0].Fields["id"] != "orphan" || !signalcenter.ValidOrigin(events[0].Origin) {
		t.Errorf("events = %+v, want one incident for orphan", events)
	}
	mustExist(t, deliveredFile(root, "orphan"), "an orphan is never released")
}

func TestReportOrphanDelivered_FaultsAndQuietPaths(t *testing.T) {
	opts, root := claimsFixture(t)
	if got, err := ReportOrphanDelivered(opts, func(string) bool { return false }, map[string]bool{}); err != nil || got != nil {
		t.Errorf("no delivered dir = %v, %v; want nothing", got, err)
	}
	if _, err := ReportOrphanDelivered(Options{}, nil, nil); err == nil {
		t.Error("no root must fail")
	}
	writeFile(t, filepath.Join(root, ".evolve", "inbox", "delivered"), "not a dir")
	if _, err := ReportOrphanDelivered(opts, func(string) bool { return false }, map[string]bool{}); err == nil {
		t.Error("an unreadable delivered dir must fail")
	}
	opts.Signals = nil
	opts.emitOrphanDelivered("x")
}

func TestApplyCycleOutcome_DeliveredWithNoCycleDeliversARootItem(t *testing.T) {
	opts, root := claimsFixture(t)
	opts.Ledger = &recordingAppender{}
	rootItem(t, root, "root-item", `{"id":"root-item"}`)

	res, err := ApplyCycleOutcome(opts, CycleOutcome{Passed: true, Delivered: true, CommittedIDs: []string{"root-item"}})

	if err != nil || len(res.Promoted) != 1 {
		t.Fatalf("ApplyCycleOutcome = %+v, %v; want the root item delivered", res, err)
	}
	mustExist(t, deliveredFile(root, "root-item"), "the delivered record")
}
