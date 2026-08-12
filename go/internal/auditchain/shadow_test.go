package auditchain

// shadow_test.go — the record a soak wave is judged on. If this is wrong the
// promotion decision is made on bad data, which is worse than not measuring.

import "testing"

// Names ShadowRecordFile, Shadow and ShadowRecord, and pins the property the
// whole stage rests on: the record distinguishes the three states a rollout
// actually produces — agreement, disagreement, and no-chain-at-all.
func TestShadow_RecordsTheThreeStatesARolloutProduces(t *testing.T) {
	t.Parallel()
	if ShadowRecordFile == "" {
		t.Fatal("the record needs a name to be found by the operator reading a wave")
	}
	full := RequiredEvidence("audit")

	// 1. Agreement.
	var agree ShadowRecord = Shadow(7, "audit", RenderChainBlock(fullChain()), "PASS", full)
	if !agree.ChainPresent || !agree.Agrees || agree.ChainVerdict != string(VerdictPASS) {
		t.Errorf("a coherent chain beside a PASS narrative must record agreement, got %+v", agree)
	}

	// 2. Disagreement — the datum the soak exists to collect.
	broken := setLink(fullChain(), LinkNarrative, StatusIncoherent, "claims a fix the diff lacks")
	dis := Shadow(7, "audit", RenderChainBlock(broken), "PASS", full)
	if dis.Agrees || dis.ChainVerdict != string(VerdictFAIL) {
		t.Errorf("a PASS narrative over an incoherent link must record DISAGREEMENT, got %+v", dis)
	}
	if len(dis.Diagnoses) == 0 {
		t.Error("the disagreement must carry the human-recognisable name, or the operator re-derives it by hand")
	}

	// 3. No chain at all — expected during rollout, and not the same as empty.
	absent := Shadow(7, "audit", "# Audit Report\n\n## Verdict\n**PASS**\n", "PASS", full)
	if absent.ChainPresent || absent.Absence == "" {
		t.Errorf("an absent chain must be recorded as absent with its reason, got %+v", absent)
	}
	if absent.Agrees {
		t.Error("absence must never be recorded as agreement — that would count un-measured cycles as successes")
	}

	// A blind judge is recorded as blind even when it agrees: a chain that
	// agreed without the evidence is not evidence that the chain works.
	blind := Shadow(7, "audit", RenderChainBlock(fullChain()), "PASS", []string{"build-report.md"})
	if len(blind.MissingEvidence) == 0 {
		t.Error("the record must name what the judge was never given")
	}
	// UPDATED with the fix for the review BLOCK, and declared: ChainVerdict is
	// now the PURE conclusion — what the auditor's reasoning entails on its own
	// terms — because folding the evidence downgrade into it pinned Agrees to
	// the state of an artifact lookup table and made every clean cycle record a
	// disagreement. The entitlement gap is still reported, in its own field.
	if blind.ChainVerdict != string(VerdictPASS) {
		t.Errorf("ChainVerdict must report the auditor's own reasoning, got %s", blind.ChainVerdict)
	}
	if blind.EvidenceAdjustedVerdict == "" {
		t.Error("a chain concluded without its evidence must carry the adjusted verdict, or the entitlement has no teeth in the soak data")
	}
	if !blind.Agrees {
		t.Error("agreement measures reasoning against narrative; a plumbing gap must not masquerade as auditor disagreement")
	}
}
