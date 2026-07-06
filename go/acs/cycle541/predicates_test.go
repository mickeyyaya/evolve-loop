//go:build acs

// Package cycle541 materialises the cycle-541 acceptance criteria for the single
// triage-committed (`## top_n`) task: triage-supply-disjoint-topn-for-fleet-width.
//
// TASK BINDING (R9.3 — predicates bind ONLY to triage `## top_n` work):
//
//	triage-report.md commits exactly ONE task to this lane:
//	  triage-supply-disjoint-topn-for-fleet-width (H) — C541_001..006
//	Every `## deferred` item (fix-treediff-guard-knowledge-base-carveout,
//	backfill-commit-orphaned-cycle-dossiers, and the rest of the out-of-fleet-
//	scope backlog) gets ZERO predicates here.
//
// FEATURE CONTEXT
//
//	Cycle-503 triage committed exactly ONE top_n task and starved the fleet wave
//	planner of the >=2 file-disjoint tasks it needs to fan out `fleet.count`
//	concurrent lanes. triagecap.SelectFleetWidthTopN (the SSOT greedy disjoint
//	packer) and its inbox-seed caller triagecap.SelectWaveSeedTopN already exist
//	and are GREEN, BUT they are wired ONLY into the wave planner's FALLBACK path
//	(cmd_loop_wave.go seedWavePlanFromInbox) — the branch that fires only when
//	the prior cycle's triage-decision.json is entirely ABSENT.
//
//	The GAP this cycle closes: a prior triage decision that IS present but
//	NARROW (fewer than `fleet.count` disjoint top_n items — e.g. THIS very
//	cycle's triage-report.md committed exactly 1) is read AS-IS by
//	productionWavePlanFn and never widened, so the fleet still collapses to a
//	single lane. The fix supplies a pure, single-sourced widening seam,
//	triagecap.WidenTopNToFleetWidth(committed, backlog, count), that backfills a
//	narrow committed set from the inbox backlog up to `count` MUTUALLY
//	FILE-DISJOINT lanes — never fabricating an overlapping pair — and wires it
//	into productionWavePlanFn so a narrow prior decision is widened before
//	fleet.PlanFromTriage partitions it.
//
// PREDICATE QUALITY (cycle-85): every load-bearing predicate EXERCISES the SUT —
// it CALLS the real triagecap / fleet functions and asserts on the returned
// selection / plan, never a "source file contains text X" grep.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive : C541_001 SelectFleetWidthTopN packs 2 disjoint candidates to 2.
//   - Negative : C541_002 all-overlapping candidates → widest disjoint set is 1,
//     never a fabricated overlapping pair (anti-no-op for the packer).
//   - E2E      : C541_003 a fleet-width decision → 2 disjoint CycleSpecs.
//   - Positive : C541_004 (RED driver) a NARROW committed set (1) + a disjoint
//     backlog candidate is WIDENED to 2 disjoint lanes — the un-starve.
//   - Negative : C541_005 (RED driver) widening a committed item with an
//     OVERLAPPING backlog item must NOT fabricate a 2nd lane — stays 1. This is
//     the strongest anti-no-op: a "pad to count regardless" impl FAILS here.
//   - Edge     : C541_006 (RED driver) count<2 preserves the committed set
//     byte-identically (legacy single-focus — no widening).
package cycle541

import (
	"encoding/json"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

// filesDisjoint reports whether no two candidates in got share a file — the
// core "safe to fan out 1:1 into concurrent lanes" invariant. A fake-disjoint
// implementation that returns two candidates claiming the same file fails here.
func filesDisjoint(got []triagecap.FleetCandidate) (string, bool) {
	seen := map[string]string{} // file -> owning candidate id
	for _, c := range got {
		for _, f := range c.Files {
			if owner, ok := seen[f]; ok {
				return f + " claimed by both " + owner + " and " + c.ID, false
			}
			seen[f] = c.ID
		}
	}
	return "", true
}

func idSet(got []triagecap.FleetCandidate) map[string]bool {
	s := map[string]bool{}
	for _, c := range got {
		s[c.ID] = true
	}
	return s
}

// C541_001 — REGRESSION (packer positive). SelectFleetWidthTopN with two
// already-disjoint candidates and fleet.count=2 returns BOTH, mutually
// file-disjoint: the baseline supply the fleet needs to fan out 2 lanes.
func TestC541_001_FleetWidthTopN_PacksTwoDisjoint(t *testing.T) {
	got := triagecap.SelectFleetWidthTopN([]triagecap.FleetCandidate{
		{ID: "task-a", Weight: 0.9, Files: []string{"go/internal/pkga/a.go"}},
		{ID: "task-b", Weight: 0.8, Files: []string{"go/internal/pkgb/b.go"}},
	}, 2)
	if len(got) != 2 {
		t.Fatalf("SelectFleetWidthTopN(2 disjoint, count=2) = %d item(s), want 2: %+v", len(got), got)
	}
	if msg, ok := filesDisjoint(got); !ok {
		t.Fatalf("top_n not mutually file-disjoint: %s", msg)
	}
}

// C541_002 — REGRESSION (packer negative / anti-no-op). When every candidate
// shares one file, the widest disjoint set is 1 — the packer must NEVER
// fabricate an overlapping pairing that would collide two concurrent lanes on
// the same tree. A length-only or overlap-tolerant impl fails here.
func TestC541_002_FleetWidthTopN_AllOverlap_NeverFabricatesPair(t *testing.T) {
	got := triagecap.SelectFleetWidthTopN([]triagecap.FleetCandidate{
		{ID: "task-x", Weight: 0.9, Files: []string{"go/internal/shared/s.go"}},
		{ID: "task-y", Weight: 0.7, Files: []string{"go/internal/shared/s.go"}},
	}, 2)
	if len(got) != 1 {
		t.Fatalf("all candidates share one file — widest disjoint set is 1, got %d: %+v", len(got), got)
	}
	if got[0].ID != "task-x" {
		t.Fatalf("got %+v, want only the single highest-weight candidate task-x, never a fabricated overlap", got)
	}
}

// C541_003 — REGRESSION (end-to-end). A fleet-width triage decision (2 disjoint
// top_n cards carrying real files) flows through fleet.PlanFromTriage into 2
// CycleSpecs whose Scope sets are disjoint — the partition adapter "sees the
// disjoint set" (the task's 4th sub-deliverable), so two lanes launch without a
// cross-lane scope collision.
func TestC541_003_PlanFromTriage_FleetWidthDecision_TwoDisjointSpecs(t *testing.T) {
	decision, err := json.Marshal(map[string]any{
		"top_n": []map[string]any{
			{"id": "task-a", "files": []string{"go/internal/pkga/a.go"}},
			{"id": "task-b", "files": []string{"go/internal/pkgb/b.go"}},
		},
	})
	if err != nil {
		t.Fatalf("marshal decision: %v", err)
	}
	specs, err := fleet.PlanFromTriage(decision, nil, 2)
	if err != nil {
		t.Fatalf("PlanFromTriage: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("PlanFromTriage(2 disjoint top_n, count=2) = %d spec(s), want 2: %+v", len(specs), specs)
	}
	scopeOwner := map[string]int{}
	for i, s := range specs {
		for _, id := range s.Scope {
			if prev, ok := scopeOwner[id]; ok {
				t.Fatalf("scope id %q owned by spec %d and %d — lanes not disjoint", id, prev, i)
			}
			scopeOwner[id] = i
		}
	}
}

// C541_004 — RED DRIVER (widen positive / un-starve). A NARROW committed
// decision (exactly 1 top_n item, the fleet-starvation shape) plus a backlog
// carrying one file-disjoint candidate and fleet.count=2 must be WIDENED to 2
// mutually file-disjoint lanes, keeping the committed item and adding the
// disjoint backlog item. This is the behaviour productionWavePlanFn was missing:
// it read a narrow prior decision as-is and ran 1 lane.
func TestC541_004_WidenTopNToFleetWidth_NarrowCommitted_WidensToTwo(t *testing.T) {
	committed := []triagecap.FleetCandidate{
		{ID: "committed-a", Weight: 0.94, Files: []string{"go/internal/pkga/a.go"}},
	}
	backlog := []triagecap.FleetCandidate{
		{ID: "backlog-b", Weight: 0.90, Files: []string{"go/internal/pkgb/b.go"}},
		{ID: "backlog-c", Weight: 0.60, Files: []string{"go/internal/pkgc/c.go"}},
	}
	got := triagecap.WidenTopNToFleetWidth(committed, backlog, 2)
	if len(got) != 2 {
		t.Fatalf("WidenTopNToFleetWidth(1 committed + disjoint backlog, count=2) = %d item(s), want 2 (fleet must not starve): %+v", len(got), got)
	}
	if msg, ok := filesDisjoint(got); !ok {
		t.Fatalf("widened top_n not mutually file-disjoint: %s", msg)
	}
	ids := idSet(got)
	if !ids["committed-a"] {
		t.Errorf("widened set %+v dropped the already-committed item committed-a — widening must PRESERVE committed work", got)
	}
	if !ids["backlog-b"] {
		t.Errorf("widened set %+v = want the highest-weight disjoint backlog item backlog-b as the 2nd lane", got)
	}
}

// C541_005 — RED DRIVER (widen negative / anti-no-op). When the ONLY backlog
// candidate OVERLAPS the committed item's file, widening must NOT fabricate a
// second lane — it stays at 1, because two lanes sharing a file cannot run
// concurrently. An implementation that blindly pads the committed set up to
// `count` regardless of file overlap FAILS here. This is the highest-leverage
// predicate: it fails on both an empty repo (WidenTopNToFleetWidth absent) AND
// on a pad-to-count fake, so passing it REQUIRES the real disjoint-aware widen.
func TestC541_005_WidenTopNToFleetWidth_OverlappingBacklog_NeverFabricates(t *testing.T) {
	committed := []triagecap.FleetCandidate{
		{ID: "committed-a", Weight: 0.94, Files: []string{"go/internal/shared/s.go"}},
	}
	backlog := []triagecap.FleetCandidate{
		{ID: "backlog-overlaps-a", Weight: 0.90, Files: []string{"go/internal/shared/s.go"}},
	}
	got := triagecap.WidenTopNToFleetWidth(committed, backlog, 2)
	if len(got) != 1 {
		t.Fatalf("only backlog item overlaps committed's file — widest disjoint set is 1, got %d (fabricated a colliding lane): %+v", len(got), got)
	}
	if got[0].ID != "committed-a" {
		t.Fatalf("got %+v, want only the committed item committed-a — never a fabricated overlapping pair", got)
	}
}

// C541_006 — RED DRIVER (widen edge / legacy preserve). count<2 is the
// pre-fleet single-focus mode: widening must be a no-op, returning the committed
// set unchanged regardless of any disjoint backlog available. Pins that the
// fleet-width path never perturbs the legacy 1-lane behaviour.
func TestC541_006_WidenTopNToFleetWidth_CountBelowTwo_PreservesCommitted(t *testing.T) {
	committed := []triagecap.FleetCandidate{
		{ID: "committed-a", Weight: 0.94, Files: []string{"go/internal/pkga/a.go"}},
	}
	backlog := []triagecap.FleetCandidate{
		{ID: "backlog-b", Weight: 0.90, Files: []string{"go/internal/pkgb/b.go"}},
	}
	for _, count := range []int{0, 1} {
		got := triagecap.WidenTopNToFleetWidth(committed, backlog, count)
		if len(got) != 1 || got[0].ID != "committed-a" {
			t.Errorf("WidenTopNToFleetWidth(count=%d) = %+v, want exactly the committed set [committed-a] (legacy single-focus, no widening)", count, got)
		}
	}
}
