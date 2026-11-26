package inboxmover

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const codeDeliveryFailed signalcenter.Code = "INBOX_DELIVERY_FAILED"

func failedDeliveryCases(root string) map[string]func(t *testing.T) {
	inbox := filepath.Join(root, ".evolve", "inbox")
	return map[string]func(t *testing.T){
		"the move errors": func(t *testing.T) {
			writeFile(t, filepath.Join(inbox, "delivered"), "not a dir")
		},
		"the move is a no-op that leaves the claim": func(t *testing.T) {
			dest := filepath.Join(inbox, "delivered", filepath.Base(claimItem(t, root, 1828, "mine")))
			if err := os.MkdirAll(filepath.Join(dest, "block"), 0o755); err != nil {
				t.Fatal(err)
			}
		},
	}
}

func TestApplyCycleOutcome_AFailedDeliveryMoveNeverReturnsTheItemToALane(t *testing.T) {
	for name := range failedDeliveryCases("") {
		t.Run(name, func(t *testing.T) {
			opts, root, _ := signalledOptions(t)
			opts.Ledger = &recordingAppender{}
			all := collectSignals(opts)
			claimItem(t, root, 1828, "mine")
			failedDeliveryCases(root)[name](t)

			_, err := ApplyCycleOutcome(opts, CycleOutcome{Cycle: 1828, Passed: true, Delivered: true, CommittedIDs: []string{"mine"}})

			if err == nil {
				t.Error("a failed delivery must be reported")
			}
			if d := ResolveDispatchability(opts, "mine"); d.Dispatchable {
				t.Errorf("dispatchability = %+v, want the item held back", d)
			}
			if _, cerr := ClaimDispatchable(opts, "mine", "1900"); cerr == nil {
				t.Error("a lane claimed an item whose delivery failed")
			}
			events := eventsWithCode(*all, codeDeliveryFailed)
			if len(events) != 1 || events[0].Fields["id"] != "mine" || events[0].Fields["end_state"] != StateQuarantine {
				t.Errorf("events = %+v, want one delivery-failed signal for mine that ends in quarantine", events)
			}
		})
	}
}

func TestApplyCycleOutcome_AFailedDeliveryAndAFailedHoldReportBoth(t *testing.T) {
	opts, root, _ := signalledOptions(t)
	opts.Ledger = &recordingAppender{}
	all := collectSignals(opts)
	claim := claimItem(t, root, 1828, "mine")
	writeFile(t, filepath.Join(root, ".evolve", "inbox", "delivered"), "not a dir")
	writeFile(t, filepath.Join(root, ".evolve", "inbox", "quarantine"), "not a dir")

	_, err := ApplyCycleOutcome(opts, CycleOutcome{Cycle: 1828, Passed: true, Delivered: true, CommittedIDs: []string{"mine"}})

	if !errors.Is(err, errDeliveryKept) || !errors.Is(err, errDeliveryFailed) {
		t.Errorf("ApplyCycleOutcome = %v, want the failed delivery and the kept id reported", err)
	}
	if d := ResolveDispatchability(opts, "mine"); d.Dispatchable {
		t.Errorf("dispatchability = %+v, want the item held back", d)
	}
	if _, cerr := ClaimDispatchable(opts, "mine", "1900"); cerr == nil {
		t.Error("a lane claimed an item whose delivery and hold both failed")
	}
	if _, serr := os.Stat(filepath.Join(root, ".evolve", "inbox", filepath.Base(claim))); serr == nil {
		t.Error("a root copy exists")
	}
	events := eventsWithCode(*all, codeDeliveryFailed)
	if len(events) != 1 || events[0].Fields["end_state"] != "processing" {
		t.Errorf("events = %+v, want one signal with end_state processing", events)
	}
}

func TestApplyCycleOutcome_AMissingCommittedItemIsNotAFailedDelivery(t *testing.T) {
	opts, _, _ := signalledOptions(t)
	opts.Ledger = &recordingAppender{}
	all := collectSignals(opts)

	if _, err := ApplyCycleOutcome(opts, CycleOutcome{Cycle: 1828, Passed: true, Delivered: true, CommittedIDs: []string{"never-filed"}}); err != nil {
		t.Fatalf("ApplyCycleOutcome = %v", err)
	}
	if got := eventsWithCode(*all, codeDeliveryFailed); len(got) != 0 {
		t.Errorf("events = %+v, want none", got)
	}
}

func TestReportOrphanDelivered_ANilReportedSetIsRefused(t *testing.T) {
	opts, _ := claimsFixture(t)
	if _, err := ReportOrphanDelivered(opts, func(string) bool { return false }, nil); !errors.Is(err, ErrBadArgs) {
		t.Errorf("err = %v, want ErrBadArgs", err)
	}
}

func TestApplyCycleOutcome_AHoldThatDoesNotMoveIsReported(t *testing.T) {
	opts, root, _ := signalledOptions(t)
	opts.Ledger = &recordingAppender{}
	claim := claimItem(t, root, 1828, "mine")
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeFile(t, filepath.Join(inbox, "delivered"), "not a dir")
	if err := os.MkdirAll(filepath.Join(inbox, "quarantine", filepath.Base(claim), "block"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := ApplyCycleOutcome(opts, CycleOutcome{Cycle: 1828, Passed: true, Delivered: true, CommittedIDs: []string{"mine"}})

	if !errors.Is(err, errHoldNoMove) {
		t.Errorf("ApplyCycleOutcome = %v, want the hold that did not move named", err)
	}
}
