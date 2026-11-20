package inboxmover

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const codeClaimStale signalcenter.Code = "INBOX_CLAIM_STALE"

func collectSignals(opts Options) *[]signalcenter.Event {
	var got []signalcenter.Event
	opts.Signals.Subscribe(func(e signalcenter.Event) { got = append(got, e) })
	return &got
}

func eventsWithCode(all []signalcenter.Event, code signalcenter.Code) []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range all {
		if e.Code == code {
			out = append(out, e)
		}
	}
	return out
}

func TestStaleClaimSweep_SealedCycleStillInProcessingWarns(t *testing.T) {
	opts, root, _ := signalledOptions(t)
	all := collectSignals(opts)
	writeHolderRun(t, root, holderRun{cycle: 1828, goal: "g77", phase: "retro", checkpoint: "phase-complete", worktree: true, lease: "dead"})
	writeFile(t, filepath.Join(root, "knowledge-base", "cycles", "cycle-1828.json"), "{}")
	claimItem(t, root, 1828, "sealed-item")

	released, err := ReleaseStaleClaims(opts, "lane-exit")
	if err != nil || len(released) != 1 || released[0].ID != "sealed-item" {
		t.Fatalf("ReleaseStaleClaims = %+v, %v; want the sealed cycle's claim released", released, err)
	}

	warned := eventsWithCode(*all, codeClaimStale)
	if len(warned) != 1 {
		t.Fatalf("%s events = %+v, want exactly one for the sealed cycle's claim", codeClaimStale, warned)
	}
	w := warned[0]
	if w.Cycle != 1828 || w.Module != signalcenter.ModuleInbox || w.Severity != signalcenter.SeverityWarn || w.Fields["id"] != "sealed-item" {
		t.Errorf("%s = cycle %d module %s severity %s fields %v, want a WARN inbox event for cycle 1828 naming sealed-item", codeClaimStale, w.Cycle, w.Module, w.Severity, w.Fields)
	}
	if w.Reason == "" || !signalcenter.ValidOrigin(w.Origin) {
		t.Errorf("%s reason %q origin %q, want a reason and a valid origin", codeClaimStale, w.Reason, w.Origin)
	}
}

func TestStaleClaimSweep_LiveHolderEmitsNoStaleWarning(t *testing.T) {
	opts, root, _ := signalledOptions(t)
	all := collectSignals(opts)
	writeHolderRun(t, root, holderRun{cycle: 1837, goal: "g81", phase: "build", lease: "live"})
	claimItem(t, root, 1837, "live-item")

	released, err := ReleaseStaleClaims(opts, "lane-exit")
	if err != nil || len(released) != 0 {
		t.Fatalf("ReleaseStaleClaims = %+v, %v; want a live holder's claim kept", released, err)
	}
	if warned := eventsWithCode(*all, codeClaimStale); len(warned) != 0 {
		t.Errorf("%s events = %+v, want none for a live holder", codeClaimStale, warned)
	}
}
