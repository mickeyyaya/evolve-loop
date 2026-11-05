package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
)

func TestIdentityCarry_AGateWriteTheFenceRevertedNeverCarriesTheVerdict(t *testing.T) {
	t.Parallel()

	for name, gateWrite := range map[string]func(h *carryHarness){
		"a gate writes an unstaged file": func(h *carryHarness) {
			h.fx.write("gate-output.txt", "written by a gate\n")
		},
		"a gate stages a file and deletes it on disk": func(h *carryHarness) {
			h.fx.write("gate-output.txt", "staged then deleted by a gate\n")
			h.fx.git("add", "gate-output.txt")
			if err := os.Remove(filepath.Join(h.fx.worktree, "gate-output.txt")); err != nil {
				t.Fatal(err)
			}
		},
		"a gate edits a tracked file": func(h *carryHarness) {
			h.fx.write("lane.txt", "edited by a gate\n")
		},
	} {
		t.Run(name, func(t *testing.T) {
			h := pendedIdenticalLane(t)
			h.duringGates = func() { gateWrite(h) }

			next, recovering, _ := h.route(t, h.auditRow())

			if !recovering || next != PhaseAudit {
				t.Fatalf("route=(%s,%v), want Audit: gate results from a tree that a gate changed are no proof, however the fence reverted it", next, recovering)
			}
			if len(h.written) != 0 {
				t.Errorf("no record is written for a carry whose gates changed the tree: %+v", h.written)
			}
		})
	}
}

func TestIdentityCarry_AGateThatLeavesTheTreeAloneStillCarries(t *testing.T) {
	t.Parallel()

	h := pendedIdenticalLane(t)
	h.duringGates = func() {}

	next, recovering, _ := h.route(t, h.auditRow())

	if !recovering || next != PhaseShip {
		t.Fatalf("route=(%s,%v), want Ship: gates that write nothing keep the carry", next, recovering)
	}
	if len(h.written) != 1 {
		t.Errorf("one record is written for the proven carry: %d", len(h.written))
	}
}

func TestGatesOnIntactTree_ATreeThatIsNotTheWorktreeTreeReportsNoGates(t *testing.T) {
	t.Parallel()

	r := committedRepoE4(t, map[string]string{"a.txt": "a\n"})
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil),
		WithCompositionGateRunner(func(context.Context, string) map[string]ciparity.GateOutcome {
			return allComposedGatesPass()
		}))

	gates, outcomes, ok := o.gatesOnIntactTree(context.Background(), r.Dir, "not-the-worktree-tree")

	if ok || gates != nil || outcomes != nil {
		t.Fatalf("gatesOnIntactTree = (%v, %v, %v), want nothing reported for a tree the worktree does not hold", gates, outcomes, ok)
	}
}
