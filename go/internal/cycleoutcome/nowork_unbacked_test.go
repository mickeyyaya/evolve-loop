package cycleoutcome

import (
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

func TestApplyNoWork_RetiresAPinnedIDNoInboxItemBacks(t *testing.T) {
	root, ws := seedLane(t, []string{"backed"}, []string{"ghost", "backed", "silent"},
		`{"top_n":[],"dropped":[{"id":"ghost","reason":"already-shipped: 51edfb7f"},{"id":"backed","reason":"stale: closed by #535"}]}`)
	opts := inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}

	var res NoWorkResult
	res, err := ApplyNoWork(NoWorkInputs{ProjectRoot: root, Workspace: ws, Cycle: 7, Stderr: io.Discard})

	if err != nil {
		t.Fatalf("ApplyNoWork: %v", err)
	}
	if len(res.Routed) != 1 || res.Routed[0] != "backed" || strings.Join(res.Retired, ",") != "ghost,silent" {
		t.Fatalf("routed = %v, retired = %v; want the inbox item routed and both unbacked pins retired in scope order", res.Routed, res.Retired)
	}
	rejected := filepath.Join(root, ".evolve", "inbox", "rejected", "cycle-7")
	if ds := inboxmover.ResolveDispatchState(opts, "ghost"); ds.State != inboxmover.StateRejected || ds.Detail != "cycle-7" {
		t.Errorf("the answered pin is rejected in cycle-7: %+v", ds)
	}
	if rec := itemRecord(t, rejected, "ghost"); rec["unbacked"] != true || !strings.Contains(rec["retired_reason"].(string), "lane triage (cycle 7) dropped: already-shipped: 51edfb7f") {
		t.Errorf("the record carries the lane's own answer: %v", rec)
	}
	if rec := itemRecord(t, rejected, "silent"); !strings.Contains(rec["retired_reason"].(string), "planned no-work (cycle 7): the lane's triage answered for nothing") {
		t.Errorf("an unanswered pin says so: %v", rec)
	}
	if ds := inboxmover.ResolveDispatchState(opts, "backed"); ds.State != inboxmover.StatePending {
		t.Errorf("the inbox item is routed in place, never retired by an unshipped lane: %+v", ds)
	}
}
