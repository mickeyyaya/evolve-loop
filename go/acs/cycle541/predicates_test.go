//go:build acs

package cycle541

import (
	"encoding/json"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

func filesDisjoint(got []triagecap.FleetCandidate) (string, bool) {
	seen := map[string]string{}
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
