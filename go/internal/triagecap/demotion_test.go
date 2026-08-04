package triagecap

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// demotion_test.go — ADR-0046 Layer 2: identical-rejection demotion for the
// triage capacity clamp (the one production heuristic gate). A heuristic
// gate rejecting with a byte-identical reason TEMPLATE across two
// consecutive cycles is a gate defect, not a work defect: real overpacking
// varies cycle to cycle; identical rejections are a determinism artifact
// (cycles 301/302, soak #2 — the phantom-floor counter re-rejected an
// honest commitment until both cycles burned their corrections and died).
//
// Two prior loop attempts at this slice failed and are pinned here:
//   - cycle 306: the hash erased ALL digits, collapsing "7 floors / cap 6"
//     with "700 floors / cap 600" — the template must be jitter-insensitive
//     but MAGNITUDE-sensitive (digit-run length survives, digit values do
//     not).
//   - cycle 307: the demotion helper existed but was never called from the
//     composition root. Demotion therefore lives INSIDE NewReviewer — there
//     is no separate constructor to forget.

// Verbatim summaries from state.json:failedApproaches, cycles 301/302.
const (
	summary301 = `cycle 301 failed during triage: review gate: phase "triage" deliverable rejected after 2 correction(s): triage overpacked: 6 committed coverage floors exceed the capacity cap 5 (= ceil(1.25×K), K=4 observed floors/turn over 1 shipped cycles). Re-emit the triage report keeping at most 5 coverage floors in ## top_n and move the remaining floor work to ## deferred — deferred items carry over to the next cycle automatically.`
	summary302 = `cycle 302 failed during triage: review gate: phase "triage" deliverable rejected after 2 correction(s): triage overpacked: 7 committed coverage floors exceed the capacity cap 5 (= ceil(1.25×K), K=4 observed floors/turn over 1 shipped cycles). Re-emit the triage report keeping at most 5 coverage floors in ## top_n and move the remaining floor work to ## deferred — deferred items carry over to the next cycle automatically.`
)

func TestReasonTemplateHash(t *testing.T) {
	t.Run("real 301 vs 302 summaries collapse (same-magnitude jitter)", func(t *testing.T) {
		if ReasonTemplateHash(summary301) != ReasonTemplateHash(summary302) {
			t.Error("cycles 301/302 carried the same rejection template (6 vs 7 floors, same cap) — hashes must match")
		}
	})
	t.Run("magnitude differences distinguish (cycle-306 lesson)", func(t *testing.T) {
		a := ReasonTemplateHash("declared 7 floors exceeds cap 6")
		b := ReasonTemplateHash("declared 700 floors exceeds cap 600")
		if a == b {
			t.Error("7-vs-700 differ by order of magnitude — hashes must differ (do not erase digits wholesale)")
		}
	})
	t.Run("same digit-count jitter collapses", func(t *testing.T) {
		a := ReasonTemplateHash("declared 12 floors exceeds cap 10")
		b := ReasonTemplateHash("declared 47 floors exceeds cap 31")
		if a != b {
			t.Error("two-digit jitter with two-digit cap is the same template — hashes must match")
		}
	})
	t.Run("different prose differs", func(t *testing.T) {
		if ReasonTemplateHash("triage overpacked: 6 floors") == ReasonTemplateHash("artifact missing: 6 floors") {
			t.Error("different reason prose must hash differently")
		}
	})
}

func TestShouldDemote(t *testing.T) {
	pair := []FailEntry{
		{Cycle: 301, Summary: summary301},
		{Cycle: 302, Summary: summary302},
	}
	t.Run("replay 301+302 fires for cycle 303", func(t *testing.T) {
		ok, detail := ShouldDemote(pair, 303)
		if !ok {
			t.Fatal("two consecutive identical-template rejections must demote the next cycle")
		}
		if !strings.Contains(detail, "301") || !strings.Contains(detail, "302") {
			t.Errorf("detail %q must name the evidence cycles", detail)
		}
	})
	// Adapted for F4 (cycle 459, inbox triagecap-prose-counter-defect):
	// ShouldDemote is now window-scoped so reset-sealed cycles between the
	// pair and the review are transparent gaps; the one-cycle relief bound
	// moved to the Review seam, which tracks consumption via the pair's
	// auto-filed defect marker (TestCapReviewer_ReliefIsOneCycleThenEnforces
	// pins that production behavior).
	t.Run("window scope: fires through a reset-sealed gap (cycle 304)", func(t *testing.T) {
		if ok, _ := ShouldDemote(pair, 304); !ok {
			t.Error("the 301/302 pair is within the demotion window of cycle 304 — a reset-sealed 303 must be a transparent gap")
		}
	})
	t.Run("beyond the demotion window does NOT fire", func(t *testing.T) {
		if ok, _ := ShouldDemote(pair, 306); ok {
			t.Error("cycle 306 is beyond the demotion window of the 301/302 pair — stale pairs must keep enforcing")
		}
	})
	t.Run("non-consecutive pair does not fire", func(t *testing.T) {
		gap := []FailEntry{{Cycle: 300, Summary: summary301}, {Cycle: 302, Summary: summary302}}
		if ok, _ := ShouldDemote(gap, 303); ok {
			t.Error("a PASS cycle between the rejections breaks the consecutive-identical signal")
		}
	})
	t.Run("different templates do not fire", func(t *testing.T) {
		diff := []FailEntry{
			{Cycle: 301, Summary: "cycle 301 failed during triage: review gate: triage overpacked: 6 committed coverage floors exceed the capacity cap 5"},
			{Cycle: 302, Summary: "cycle 302 failed during triage: review gate: triage overpacked: 700 committed coverage floors exceed the capacity cap 5"},
		}
		if ok, _ := ShouldDemote(diff, 303); ok {
			t.Error("different templates = real (varying) overpacking — must keep enforcing")
		}
	})
	t.Run("non-gate failures are ignored", func(t *testing.T) {
		other := []FailEntry{
			{Cycle: 301, Summary: "cycle 301 failed during build: tests red"},
			{Cycle: 302, Summary: "cycle 302 failed during build: tests red"},
		}
		if ok, _ := ShouldDemote(other, 303); ok {
			t.Error("only this gate's rejections (marker match) may demote it")
		}
	})
	t.Run("single entry does not fire", func(t *testing.T) {
		if ok, _ := ShouldDemote(pair[:1], 302); ok {
			t.Error("one rejection is not a pattern")
		}
	})
}

// overpackedArtifact builds a top_n that counts over any small cap: three
// floor-bearing bullets each naming a distinct known package.
const overpackedArtifact = `## top_n (commit to THIS cycle)
- coverage-a: push swarmrunner coverage to ≥98%
- coverage-b: push bridge coverage to ≥98%
- coverage-c: push evalgate coverage to ≥98%

## deferred (carry to NEXT cycle's carryoverTodos)
`

// newDemotionFixture wires a CapReviewer whose seams put it at enforce with
// cap 2 (K=1 window ⇒ cap ceil(1.25)=2) against a 3-floor artifact, and a
// failedApproaches history replaying the 301/302 pair. workspace run.json
// carries cycle_id so the reviewer knows "now".
func newDemotionFixture(t *testing.T, cycleID string, fails []FailEntry) (*CapReviewer, core.ReviewInput, string) {
	t.Helper()
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-303")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, TriageArtifactName()), []byte(overpackedArtifact), 0o644); err != nil {
		t.Fatal(err)
	}
	if cycleID != "" {
		if err := os.WriteFile(filepath.Join(ws, "run.json"), []byte(`{"cycle_id":`+cycleID+`}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r := newCapReviewer(config.StageEnforce)
	r.logf = func(string, ...any) {}
	r.pkgsFn = func(string) []string { return []string{"swarmrunner", "bridge", "evalgate"} }
	r.windowFn = func(string) []core.TriageThroughputEntry {
		return []core.TriageThroughputEntry{{Cycle: 300, Floors: 1}}
	}
	r.failsFn = func(string) []FailEntry { return fails }
	in := core.ReviewInput{Phase: "triage", Workspace: ws, ProjectRoot: root}
	return r, in, root
}

func TestCapReviewer_DemotesAfterIdenticalPair(t *testing.T) {
	pair := []FailEntry{{Cycle: 301, Summary: summary301}, {Cycle: 302, Summary: summary302}}
	r, in, root := newDemotionFixture(t, "303", pair)

	res := r.Review(context.Background(), in)
	if !res.Approve {
		t.Fatalf("demoted gate must approve (shadow semantics), got reject: %s", res.Reason)
	}

	// Exactly one auto-filed defect, idempotent across a second review.
	matches, _ := filepath.Glob(filepath.Join(root, ".evolve", "inbox", "auto-heuristic-demotion-*.json"))
	if len(matches) != 1 {
		t.Fatalf("demotion must auto-file exactly one inbox defect, found %d", len(matches))
	}
	_ = r.Review(context.Background(), in)
	matches, _ = filepath.Glob(filepath.Join(root, ".evolve", "inbox", "auto-heuristic-demotion-*.json"))
	if len(matches) != 1 {
		t.Errorf("re-review must not duplicate the auto-filed defect, found %d", len(matches))
	}
}

func TestCapReviewer_EnforcesWithoutPair(t *testing.T) {
	// Same overpacked artifact, no failure history → the clamp still BLOCKs.
	// Demotion must never weaken first-offense enforcement (cycle-307
	// composition-root lesson: this exercises the REAL production reviewer,
	// not a helper that wiring can forget).
	r, in, _ := newDemotionFixture(t, "303", nil)
	if res := r.Review(context.Background(), in); res.Approve {
		t.Fatal("no identical-rejection history: enforce must still reject an overpacked triage")
	}
}

func TestCapReviewer_NoRunJSONStaysEnforcing(t *testing.T) {
	// Missing run.json ⇒ unknown current cycle ⇒ demotion cannot prove the
	// one-cycle scope ⇒ fail toward enforcement.
	pair := []FailEntry{{Cycle: 301, Summary: summary301}, {Cycle: 302, Summary: summary302}}
	r, in, _ := newDemotionFixture(t, "", pair)
	if res := r.Review(context.Background(), in); res.Approve {
		t.Fatal("without a readable cycle_id the gate must keep enforcing")
	}
}

// --- cycle-1301: remedy_status on the demotion ledger record ---------------
//
// The auto-filed record is the DURABLE ledger entry for a demotion event, but
// it only ever carried a prose `action` narrative: nothing on it says whether a
// salvage of the suspected gate defect was ATTEMPTED or whether the loop
// concluded NO REMEDY was possible. Commit 29915424 had to explain two gate
// demotions in a queue chore commit body for exactly that reason. remedy_status
// is caller-declared (the writer has no way to infer it), defaults to `pending`
// at file time, and carries a CLOSED vocabulary — an unknown value normalises
// to pending rather than being written through, so a downstream reader can
// switch on three cases and no more.

func TestNormalizeRemedyStatus(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want RemedyStatus
	}{
		{"canonical pending", "pending", RemedyPending},
		{"canonical salvage_attempted", "salvage_attempted", RemedySalvageAttempted},
		{"canonical no_remedy_possible", "no_remedy_possible", RemedyNoRemedyPossible},
		{"blank is never written", "", RemedyPending},
		{"unknown value is not echoed", "gave-up-ish", RemedyPending},
		{"wrong case is not canonical", "No_Remedy_Possible", RemedyPending},
		{"padded value is not canonical", "\tpending\n", RemedyPending},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeRemedyStatus(tc.in); got != tc.want {
				t.Errorf("NormalizeRemedyStatus(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNewDemotionLedgerRecord_DeclaredOutcomes(t *testing.T) {
	const detail = "identical rejection template in cycles 301 and 302 (hash deadbeefdeadbeef)"

	t.Run("declared terminal outcomes reach the record", func(t *testing.T) {
		for _, want := range []RemedyStatus{RemedyPending, RemedySalvageAttempted, RemedyNoRemedyPossible} {
			rec := NewDemotionLedgerRecord(303, 301, 302, detail, want)
			if rec.RemedyStatus != want {
				t.Errorf("RemedyStatus = %q, want %q", rec.RemedyStatus, want)
			}
		}
	})

	t.Run("junk normalises to pending", func(t *testing.T) {
		rec := NewDemotionLedgerRecord(303, 301, 302, detail, RemedyStatus("nonsense"))
		if rec.RemedyStatus != RemedyPending {
			t.Errorf("RemedyStatus = %q, want %q — an unvalidated status must never reach the ledger", rec.RemedyStatus, RemedyPending)
		}
	})

	t.Run("identity and relief bookkeeping unchanged", func(t *testing.T) {
		var rec DemotionLedgerRecord = NewDemotionLedgerRecord(303, 301, 302, detail, RemedySalvageAttempted)
		if rec.ID != "auto-heuristic-demotion-triagecap-c301-c302" {
			t.Errorf("ID = %q, want the pair-identity slug", rec.ID)
		}
		if rec.RelievedCycle != 303 {
			t.Errorf("RelievedCycle = %d, want 303", rec.RelievedCycle)
		}
		if !strings.Contains(rec.EvidencePointer, "cycle-301") || !strings.Contains(rec.EvidencePointer, "cycle-302") {
			t.Errorf("EvidencePointer = %q, want it to name both evidence cycles", rec.EvidencePointer)
		}
		if !strings.Contains(rec.Action, detail) {
			t.Errorf("Action = %q, want it to quote the demotion detail", rec.Action)
		}
		if rec.Priority != "HIGH" || rec.Weight != 0.7 || rec.InjectedBy != "triagecap-demotion" || rec.InjectedAt == "" {
			t.Errorf("record metadata drifted: priority=%q weight=%v injectedBy=%q injectedAt=%q",
				rec.Priority, rec.Weight, rec.InjectedBy, rec.InjectedAt)
		}
	})
}

func TestCapReviewer_LedgerRecordDefaultsToPendingRemedy(t *testing.T) {
	// Wiring proof through the REAL reviewer: the production call site must
	// thread a status, and at file time the honest value is `pending`.
	pair := []FailEntry{{Cycle: 301, Summary: summary301}, {Cycle: 302, Summary: summary302}}
	r, in, root := newDemotionFixture(t, "303", pair)
	if res := r.Review(context.Background(), in); !res.Approve {
		t.Fatalf("demoted gate must approve (shadow semantics), got reject: %s", res.Reason)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".evolve", "inbox", "auto-heuristic-demotion-triagecap-c301-c302.json"))
	if err != nil {
		t.Fatalf("reading the auto-filed ledger record: %v", err)
	}
	var got struct {
		RemedyStatus  *string `json:"remedy_status"`
		RelievedCycle *int    `json:"relieved_cycle"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("ledger record must be valid JSON: %v", err)
	}
	if got.RemedyStatus == nil {
		t.Fatalf("ledger record carries no remedy_status field: %s", raw)
	}
	if *got.RemedyStatus != string(RemedyPending) {
		t.Errorf("remedy_status = %q, want %q at file time", *got.RemedyStatus, RemedyPending)
	}
	if got.RelievedCycle == nil || *got.RelievedCycle != 303 {
		t.Errorf("relieved_cycle regressed: %v", got.RelievedCycle)
	}
}
