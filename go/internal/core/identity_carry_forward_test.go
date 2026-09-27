package core

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
)

type carryHarness struct {
	fx          *shippedLane
	gatesOn     string
	written     []CompositionVerdictInput
	writtenTo   string
	gates       map[string]string
	writeErr    error
	snapErr     error
	duringGates func()
}

func allComposedGatesPass() map[string]string {
	out := map[string]string{}
	for _, g := range ciparity.RequiredComposedGates {
		out[g] = "pass"
	}
	return out
}

// pendedIdenticalLane is a lane whose audited change is pended, byte for byte, on the new base after a clean rebase.
func pendedIdenticalLane(t *testing.T) *carryHarness {
	t.Helper()
	fx := rebasedCommittedLane(t)
	fx.git("reset", "-q", "--soft", fx.newBase)
	return &carryHarness{fx: fx, gates: allComposedGatesPass()}
}

func (h *carryHarness) auditRow() LedgerEntry {
	row := h.fx.auditRow()
	row.ArtifactSHA256 = "audit-ref"
	row.GitHEAD = h.fx.base
	return row
}

func (h *carryHarness) route(t *testing.T, rows ...LedgerEntry) (Phase, bool, CycleState) {
	t.Helper()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{entries: rows}, buildRunners(nil),
		WithCompositionSnapshot(func(context.Context, string, string) (CompositionAuditSnapshot, error) {
			return CompositionAuditSnapshot{LaneAuditRef: "audit-ref"}, nil
		}),
		WithCompositionGateRunner(func(_ context.Context, worktree string) map[string]string {
			h.gatesOn = worktree
			if h.duringGates != nil {
				h.duringGates()
			}
			return h.gates
		}),
		WithCompositionVerdictWriter(func(path string, in CompositionVerdictInput) error {
			h.writtenTo = path
			h.written = append(h.written, in)
			return h.writeErr
		}),
	)
	cs := h.fx.cycleState()
	next, recovering := o.routeRebasedExplanation(context.Background(), h.fx.root, unwindCycle, &cs)
	return next, recovering, cs
}

func TestRouteRebasedExplanation_AByteIdenticalRebaseShipsOnTheCarriedVerdict(t *testing.T) {
	h := pendedIdenticalLane(t)
	tree1 := h.fx.git("write-tree")

	next, recovering, cs := h.route(t, h.auditRow())

	if !recovering || next != PhaseShip {
		t.Fatalf("route=(%s,%v), want Ship: the audited verdict carries across a byte-identical rebase", next, recovering)
	}
	if cs.WorktreeBaseSHA != h.fx.newBase {
		t.Errorf("the base is rebound: %s want %s", cs.WorktreeBaseSHA, h.fx.newBase)
	}
	if h.gatesOn != h.fx.worktree || len(h.written) != 1 || h.writtenTo != filepath.Join(h.fx.root, ".evolve", "ledger.jsonl") {
		t.Fatalf("the gates ran on the worktree (%q) and one record went to the root ledger ship reads (%d, %q)", h.gatesOn, len(h.written), h.writtenTo)
	}
	rec := h.written[0]
	if rec.Method != identicalRebaseMethod || rec.LaneAuditRef != "audit-ref" || rec.AuditedTreeSHA != h.fx.auditedTree || rec.TreeStateSHA != tree1 ||
		rec.AuditedBase != h.fx.base || rec.GitHead != h.fx.newBase || rec.Cycle != unwindCycle || !bytes.Equal(rec.AuditedDiff, rec.ComposedDiff) || rec.PatchID == "" {
		t.Errorf("the record names the audit, both trees, both bases and the equal diffs: %+v", rec)
	}
	if got := h.fx.git("write-tree"); got != tree1 {
		t.Errorf("the tree the gates ran on is the tree ship commits: %s want %s", got, tree1)
	}
}

func TestRouteRebasedExplanation_ACarryThatCannotBeProvenReturnsToAudit(t *testing.T) {
	for name, tc := range map[string]struct {
		arrange func(h *carryHarness) []LedgerEntry
	}{
		"the audited tree is not the pended change": {func(h *carryHarness) []LedgerEntry {
			row := h.auditRow()
			row.WorktreeTreeSHA = h.fx.git("rev-parse", h.fx.base+"^{tree}")
			return []LedgerEntry{row}
		}},
		"the audit was bound on another base": {func(h *carryHarness) []LedgerEntry {
			row := h.auditRow()
			row.GitHEAD = h.fx.newBase
			return []LedgerEntry{row}
		}},
		"the audit row names no artifact": {func(h *carryHarness) []LedgerEntry {
			row := h.auditRow()
			row.ArtifactSHA256 = ""
			return []LedgerEntry{row}
		}},
		"no audit row names a tree": {func(h *carryHarness) []LedgerEntry { return nil }},
		"a composed gate is red": {func(h *carryHarness) []LedgerEntry {
			h.gates["test"] = "fail"
			return []LedgerEntry{h.auditRow()}
		}},
		"the record cannot be written": {func(h *carryHarness) []LedgerEntry {
			h.writeErr = errors.New("ledger sealed")
			return []LedgerEntry{h.auditRow()}
		}},
		"a gate stages a change into the tree": {func(h *carryHarness) []LedgerEntry {
			h.duringGates = func() {
				h.fx.write("gate-output.txt", "written by a gate\n")
				h.fx.git("add", "gate-output.txt")
			}
			return []LedgerEntry{h.auditRow()}
		}},
	} {
		t.Run(name, func(t *testing.T) {
			h := pendedIdenticalLane(t)
			rows := tc.arrange(h)

			next, recovering, cs := h.route(t, rows...)

			if !recovering || next != PhaseAudit {
				t.Fatalf("route=(%s,%v), want Audit: a carry the host cannot prove is re-audited, never shipped", next, recovering)
			}
			if cs.WorktreeBaseSHA != h.fx.newBase {
				t.Errorf("the rebind stands even when the carry does not: %s", cs.WorktreeBaseSHA)
			}
			if name != "the record cannot be written" && len(h.written) != 0 {
				t.Errorf("no record is written for a carry that failed its proof: %+v", h.written)
			}
		})
	}
}
