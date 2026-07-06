package triagecap

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
)

// topn_width_amplified_test.go — cycle-541 test-amplification lane for
// WidenTopNToFleetWidth (inbox triage-supply-disjoint-topn-for-fleet-width).
// Authored black-box from the exported godoc contract and test-report.md's
// builder contract (preserve committed verbatim; count<2 no-op; backfill
// highest-weight-first skipping duplicate-ID/file-overlap; never fabricate a
// colliding lane) — not from topn_width.go's implementation. These probe
// boundaries the RED predicates (TestC541_004..006) did not exercise:
// negative counts, nil/empty inputs, multi-file partial overlap, duplicate
// IDs inside the backlog itself, mutation safety, and large-scale backfills.

func c541ampCand(id string, weight float64, files ...string) FleetCandidate {
	return FleetCandidate{ID: id, Weight: weight, Files: append([]string(nil), files...)}
}

func c541ampFilesOverlap(a, b []string) bool {
	set := make(map[string]struct{}, len(a))
	for _, f := range a {
		set[f] = struct{}{}
	}
	for _, f := range b {
		if _, ok := set[f]; ok {
			return true
		}
	}
	return false
}

func c541ampAllDisjoint(t *testing.T, cands []FleetCandidate) {
	t.Helper()
	for i := 0; i < len(cands); i++ {
		for j := i + 1; j < len(cands); j++ {
			if c541ampFilesOverlap(cands[i].Files, cands[j].Files) {
				t.Fatalf("candidates %q and %q share a file, expected mutual disjointness: %v vs %v",
					cands[i].ID, cands[j].ID, cands[i].Files, cands[j].Files)
			}
		}
	}
}

func c541ampIDs(cands []FleetCandidate) []string {
	ids := make([]string, len(cands))
	for i, c := range cands {
		ids[i] = c.ID
	}
	return ids
}

func c541ampContainsID(cands []FleetCandidate, id string) bool {
	for _, c := range cands {
		if c.ID == id {
			return true
		}
	}
	return false
}

func TestC541Amp_CountBelowTwo_PreservesCommittedUnchanged(t *testing.T) {
	backlog := []FleetCandidate{
		c541ampCand("backlog-a", 0.9, "x/a.go"),
		c541ampCand("backlog-b", 0.8, "x/b.go"),
	}
	for _, count := range []int{-3, -1, 0, 1} {
		committed := []FleetCandidate{
			c541ampCand("committed-1", 0.5, "shared.go"),
			c541ampCand("committed-2", 0.4, "shared.go"), // committed items may overlap each other
		}
		got := WidenTopNToFleetWidth(committed, backlog, count)
		if !reflect.DeepEqual(got, committed) {
			t.Fatalf("count=%d: got %+v, want committed unchanged %+v", count, got, committed)
		}
	}
}

func TestC541Amp_NilAndEmptyInputsDoNotPanic(t *testing.T) {
	cases := []struct {
		name      string
		committed []FleetCandidate
		backlog   []FleetCandidate
		count     int
	}{
		{"all nil", nil, nil, 3},
		{"empty non-nil slices", []FleetCandidate{}, []FleetCandidate{}, 3},
		{"nil committed, populated backlog", nil, []FleetCandidate{c541ampCand("b1", 0.5, "f1.go")}, 3},
		{"populated committed, nil backlog", []FleetCandidate{c541ampCand("c1", 0.5, "f1.go")}, nil, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := WidenTopNToFleetWidth(tc.committed, tc.backlog, tc.count)
			c541ampAllDisjoint(t, got)
			if len(got) > tc.count {
				t.Fatalf("result length %d exceeds count %d", len(got), tc.count)
			}
			for _, c := range tc.committed {
				if !c541ampContainsID(got, c.ID) {
					t.Fatalf("committed candidate %q must never be dropped, got %v", c.ID, c541ampIDs(got))
				}
			}
		})
	}
}

func TestC541Amp_BackfillsHighestWeightFirstAmongDisjoint(t *testing.T) {
	committed := []FleetCandidate{c541ampCand("committed-1", 1.0, "core/committed.go")}
	backlog := []FleetCandidate{
		// intentionally NOT weight-sorted in input order
		c541ampCand("low", 0.2, "pkg/low.go"),
		c541ampCand("high", 0.9, "pkg/high.go"),
		c541ampCand("mid", 0.5, "pkg/mid.go"),
	}
	got := WidenTopNToFleetWidth(committed, backlog, 3)
	if len(got) != 3 {
		t.Fatalf("want 3 candidates (1 committed + 2 backfilled), got %d: %v", len(got), c541ampIDs(got))
	}
	if got[0].ID != "committed-1" {
		t.Fatalf("committed candidate must lead the result verbatim, got %+v", got[0])
	}
	tail := got[1:]
	if tail[0].ID != "high" || tail[1].ID != "mid" {
		t.Fatalf("backfill must be highest-weight-first among disjoint candidates, got tail %v", c541ampIDs(tail))
	}
	if c541ampContainsID(got, "low") {
		t.Fatalf("low-weight candidate must not be added once count is satisfied by higher-weight disjoint picks: %v", c541ampIDs(got))
	}
	c541ampAllDisjoint(t, got)
}

func TestC541Amp_SkipsBacklogCandidateOverlappingCommittedFiles(t *testing.T) {
	committed := []FleetCandidate{c541ampCand("committed-1", 1.0, "shared/pkg.go")}
	backlog := []FleetCandidate{
		c541ampCand("colliding-high-weight", 0.95, "shared/pkg.go"), // overlaps committed
		c541ampCand("disjoint-lower-weight", 0.4, "other/pkg.go"),
	}
	got := WidenTopNToFleetWidth(committed, backlog, 2)
	if c541ampContainsID(got, "colliding-high-weight") {
		t.Fatalf("candidate overlapping a committed file must never be added, even at higher weight: %v", c541ampIDs(got))
	}
	if !c541ampContainsID(got, "disjoint-lower-weight") {
		t.Fatalf("disjoint lower-weight candidate should fill the open lane instead: %v", c541ampIDs(got))
	}
	c541ampAllDisjoint(t, got)
}

func TestC541Amp_SkipsOverlapAmongBackfillCandidatesThemselves(t *testing.T) {
	committed := []FleetCandidate{c541ampCand("committed-1", 1.0, "core/committed.go")}
	backlog := []FleetCandidate{
		c541ampCand("backlog-high", 0.9, "pkg/shared.go"),
		c541ampCand("backlog-also-high", 0.85, "pkg/shared.go"), // overlaps backlog-high, not committed
	}
	got := WidenTopNToFleetWidth(committed, backlog, 3) // room for both if disjoint, but they collide
	if c541ampContainsID(got, "backlog-high") && c541ampContainsID(got, "backlog-also-high") {
		t.Fatalf("two backlog candidates sharing a file must never both be added: %v", c541ampIDs(got))
	}
	if !c541ampContainsID(got, "backlog-high") {
		t.Fatalf("higher-weight backlog candidate should win the shared-file slot: %v", c541ampIDs(got))
	}
	if len(got) != 2 {
		t.Fatalf("backlog exhausted after 1 disjoint pick; want len 2 (committed + 1 backfill), got %d: %v", len(got), c541ampIDs(got))
	}
	c541ampAllDisjoint(t, got)
}

func TestC541Amp_SkipsDuplicateIDAgainstCommitted(t *testing.T) {
	committed := []FleetCandidate{c541ampCand("dup-id", 0.5, "committed/file.go")}
	backlog := []FleetCandidate{
		c541ampCand("dup-id", 0.99, "backlog/other-file.go"), // same ID, DIFFERENT files
		c541ampCand("fresh", 0.3, "backlog/fresh.go"),
	}
	got := WidenTopNToFleetWidth(committed, backlog, 3)
	count := 0
	for _, c := range got {
		if c.ID == "dup-id" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("duplicate ID must appear exactly once (the committed entry), got %d occurrences: %v", count, got)
	}
	if !c541ampContainsID(got, "fresh") {
		t.Fatalf("distinct-ID disjoint candidate should still be backfilled: %v", c541ampIDs(got))
	}
}

func TestC541Amp_SkipsDuplicateIDWithinBacklogItself(t *testing.T) {
	committed := []FleetCandidate{c541ampCand("committed-1", 1.0, "core/c.go")}
	backlog := []FleetCandidate{
		c541ampCand("stale-dup", 0.9, "pkg/a.go"),
		c541ampCand("stale-dup", 0.9, "pkg/b.go"), // same ID reappears (disjoint files, stale inbox re-seed)
	}
	got := WidenTopNToFleetWidth(committed, backlog, 3)
	count := 0
	for _, c := range got {
		if c.ID == "stale-dup" {
			count++
		}
	}
	if count > 1 {
		t.Fatalf("duplicate backlog ID must be backfilled at most once, got %d occurrences: %v", count, got)
	}
}

func TestC541Amp_NeverAddsOverlapToReachCount(t *testing.T) {
	committed := []FleetCandidate{c541ampCand("committed-1", 1.0, "shared.go")}
	backlog := []FleetCandidate{
		c541ampCand("colliding-1", 0.9, "shared.go"),
		c541ampCand("colliding-2", 0.8, "shared.go"),
	}
	got := WidenTopNToFleetWidth(committed, backlog, 3)
	if len(got) != 1 {
		t.Fatalf("no disjoint backlog candidate exists; result must stay at len(committed)=1, got %d: %v", len(got), c541ampIDs(got))
	}
	c541ampAllDisjoint(t, got)
}

func TestC541Amp_CommittedInternalOverlapPreservedVerbatim(t *testing.T) {
	committed := []FleetCandidate{
		c541ampCand("committed-a", 0.6, "shared/file.go"),
		c541ampCand("committed-b", 0.5, "shared/file.go"), // overlaps committed-a; still authoritative
	}
	backlog := []FleetCandidate{
		c541ampCand("backlog-disjoint", 0.99, "pkg/other.go"),
	}
	got := WidenTopNToFleetWidth(committed, backlog, 3)
	if len(got) < 2 || got[0].ID != "committed-a" || got[1].ID != "committed-b" {
		t.Fatalf("both overlapping committed candidates must be preserved verbatim and in order, got %v", c541ampIDs(got))
	}
	if !c541ampContainsID(got, "backlog-disjoint") {
		t.Fatalf("disjoint backlog candidate should still backfill the remaining lane: %v", c541ampIDs(got))
	}
}

func TestC541Amp_EmptyFilesCandidatesNeverCollide(t *testing.T) {
	committed := []FleetCandidate{c541ampCand("committed-1", 1.0)} // no declared files
	backlog := []FleetCandidate{
		c541ampCand("no-files-a", 0.9),
		c541ampCand("no-files-b", 0.8),
	}
	got := WidenTopNToFleetWidth(committed, backlog, 3)
	if len(got) != 3 {
		t.Fatalf("candidates with no declared files never overlap; all 3 should backfill, got %d: %v", len(got), c541ampIDs(got))
	}
}

func TestC541Amp_ResultNeverExceedsCount(t *testing.T) {
	committed := []FleetCandidate{c541ampCand("committed-1", 1.0, "c.go")}
	backlog := make([]FleetCandidate, 0, 50)
	for i := 0; i < 50; i++ {
		backlog = append(backlog, c541ampCand(fmt.Sprintf("b-%d", i), float64(50-i), fmt.Sprintf("pkg/f%d.go", i)))
	}
	for _, count := range []int{2, 5, 10, 49} {
		got := WidenTopNToFleetWidth(committed, backlog, count)
		if len(got) > count {
			t.Fatalf("count=%d: result length %d exceeds requested width", count, len(got))
		}
		c541ampAllDisjoint(t, got)
	}
}

func TestC541Amp_BacklogExhaustedBeforeReachingCount(t *testing.T) {
	committed := []FleetCandidate{c541ampCand("committed-1", 1.0, "c.go")}
	backlog := []FleetCandidate{c541ampCand("only-one", 0.5, "only.go")}
	got := WidenTopNToFleetWidth(committed, backlog, 10)
	if len(got) != 2 {
		t.Fatalf("backlog exhausted after 1 pick; want len 2, got %d: %v", len(got), c541ampIDs(got))
	}
}

func TestC541Amp_DoesNotMutateCallerSlices(t *testing.T) {
	committed := []FleetCandidate{c541ampCand("committed-1", 0.9, "c.go")}
	backlog := []FleetCandidate{
		c541ampCand("b1", 0.8, "b1.go"),
		c541ampCand("b2", 0.7, "b2.go"),
	}
	committedSnapshot := append([]FleetCandidate(nil), committed...)
	backlogSnapshot := append([]FleetCandidate(nil), backlog...)

	_ = WidenTopNToFleetWidth(committed, backlog, 3)

	if !reflect.DeepEqual(committed, committedSnapshot) {
		t.Fatalf("committed slice was mutated by WidenTopNToFleetWidth: got %v, want %v", committed, committedSnapshot)
	}
	if !reflect.DeepEqual(backlog, backlogSnapshot) {
		t.Fatalf("backlog slice was mutated by WidenTopNToFleetWidth: got %v, want %v", backlog, backlogSnapshot)
	}
}

func TestC541Amp_LargeScaleDisjointBackfill(t *testing.T) {
	const n = 300
	committed := []FleetCandidate{c541ampCand("committed-1", 1000, "committed/only.go")}
	backlog := make([]FleetCandidate, 0, n)
	for i := 0; i < n; i++ {
		backlog = append(backlog, c541ampCand(fmt.Sprintf("b-%d", i), float64(i), fmt.Sprintf("pkg%d/f.go", i)))
	}
	const count = 51 // 1 committed + 50 backfilled
	got := WidenTopNToFleetWidth(committed, backlog, count)
	if len(got) != count {
		t.Fatalf("large disjoint backlog should fully satisfy count=%d, got %d", count, len(got))
	}
	c541ampAllDisjoint(t, got)

	tail := got[1:]
	sorted := append([]FleetCandidate(nil), tail...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Weight > sorted[j].Weight })
	if !reflect.DeepEqual(tail, sorted) {
		t.Fatalf("backfilled tail must stay in descending-weight order, got %v", c541ampIDs(tail))
	}
	if sorted[0].ID != fmt.Sprintf("b-%d", n-1) {
		t.Fatalf("highest-weight backlog candidate must be picked first, got %s", sorted[0].ID)
	}
}

func TestC541Amp_CommittedAlreadyAtCount_BacklogNotConsumed(t *testing.T) {
	committed := []FleetCandidate{
		c541ampCand("committed-1", 0.1, "a.go"),
		c541ampCand("committed-2", 0.1, "b.go"),
	}
	backlog := []FleetCandidate{
		c541ampCand("higher-weight-disjoint", 99.0, "z.go"), // higher weight, fully disjoint
	}
	got := WidenTopNToFleetWidth(committed, backlog, 2)
	if len(got) != 2 || c541ampContainsID(got, "higher-weight-disjoint") {
		t.Fatalf("committed already fills count=2; backlog must not be consulted: %v", c541ampIDs(got))
	}
	if got[0].ID != "committed-1" || got[1].ID != "committed-2" {
		t.Fatalf("committed candidates must be preserved verbatim and in order: %v", c541ampIDs(got))
	}
}

func TestC541Amp_MultiFileCandidatePartialOverlapStillBlocks(t *testing.T) {
	committed := []FleetCandidate{c541ampCand("committed-1", 1.0, "shared/only.go")}
	backlog := []FleetCandidate{
		// touches 3 files; only ONE collides with committed -> whole candidate must be skipped
		c541ampCand("multi-file-partial-overlap", 0.99, "pkg/a.go", "shared/only.go", "pkg/b.go"),
		c541ampCand("fully-disjoint", 0.5, "pkg/c.go", "pkg/d.go"),
	}
	got := WidenTopNToFleetWidth(committed, backlog, 2)
	if c541ampContainsID(got, "multi-file-partial-overlap") {
		t.Fatalf("a candidate sharing even ONE file with a claimed file must be entirely skipped: %v", c541ampIDs(got))
	}
	if !c541ampContainsID(got, "fully-disjoint") {
		t.Fatalf("the fully disjoint multi-file candidate should backfill instead: %v", c541ampIDs(got))
	}
}
