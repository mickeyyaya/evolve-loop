package ledger

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
)

func greenGates() map[string]string {
	out := map[string]string{}
	for _, g := range ciparity.RequiredComposedGates {
		out[g] = "pass"
	}
	return out
}

func TestWriteCompositionVerdict_IdenticalRebaseRecordsTheAuditedTreeAndReadsBack(t *testing.T) {
	dir := t.TempDir()
	ledgerPath := filepath.Join(dir, "ledger.jsonl")
	patchID, err := PatchID([]byte(compTestDiff))
	if err != nil {
		t.Fatal(err)
	}
	write := func(ref, audited, composed string) {
		t.Helper()
		if err := WriteCompositionVerdict(ledgerPath, CompositionVerdictInput{
			Cycle: 1715, Method: IdenticalRebaseMethod, LaneAuditRef: ref, PatchID: patchID,
			AuditedBase: "base0", GitHead: "base1", TreeStateSHA: composed, AuditedTreeSHA: audited,
			GateResults: greenGates(), AuditedDiff: []byte(compTestDiff), ComposedDiff: []byte(compTestDiff),
		}); err != nil {
			t.Fatalf("WriteCompositionVerdict: %v", err)
		}
	}
	write("audit-a", "tree-a0", "tree-a1")
	write("audit-b", "tree-b0", "tree-b1")
	write("audit-a", "tree-a0", "tree-a2")

	got, found, err := LatestCompositionVerdict(ledgerPath, IdenticalRebaseMethod, "audit-a")

	if err != nil || !found {
		t.Fatalf("LatestCompositionVerdict = found %v, err %v", found, err)
	}
	if got.AuditedTreeSHA != "tree-a0" || got.TreeStateSHA != "tree-a2" || got.AuditedBase != "base0" || got.GitHead != "base1" ||
		got.Cycle != 1715 || got.PatchID != patchID || got.GateResults["compile"] != "pass" || got.AuditedDiffPath == "" || got.ComposedDiffPath == "" {
		t.Errorf("the newest record of the audit carries what ship must re-prove: %+v", got)
	}
	if _, found, err := LatestCompositionVerdict(ledgerPath, TrivialRebaseMethod, "audit-a"); err != nil || found {
		t.Errorf("another method never answers for a carry: found=%v err=%v", found, err)
	}
	if _, found, err := LatestCompositionVerdict(ledgerPath, IdenticalRebaseMethod, "audit-c"); err != nil || found {
		t.Errorf("an unknown audit has no carry: found=%v err=%v", found, err)
	}
	if err := New(dir).Verify(context.Background()); err != nil {
		t.Errorf("the chain still verifies with the new field: %v", err)
	}
}

func TestLatestCompositionVerdict_AbsentLedgerIsNoCarry(t *testing.T) {
	if _, found, err := LatestCompositionVerdict(filepath.Join(t.TempDir(), "ledger.jsonl"), IdenticalRebaseMethod, "x"); err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
}
