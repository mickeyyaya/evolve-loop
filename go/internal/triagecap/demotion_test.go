package triagecap

import (
	"context"
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
	t.Run("one-cycle scope: does NOT fire for cycle 304", func(t *testing.T) {
		if ok, _ := ShouldDemote(pair, 304); ok {
			t.Error("demotion is scoped to the cycle immediately after the pair — 304 must enforce again")
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
