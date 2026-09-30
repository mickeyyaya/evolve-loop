package auditchain

import "testing"

func TestShadow_RecordsTheThreeStatesARolloutProduces(t *testing.T) {
	t.Parallel()
	if ShadowRecordFile == "" {
		t.Fatal("the record needs a name to be found by the operator reading a wave")
	}
	full := RequiredEvidence("audit")

	var agree ShadowRecord = Shadow(7, "audit", RenderChainBlock(fullChain()), "PASS", full)
	if !agree.ChainPresent || !agree.Agrees || agree.ChainVerdict != string(VerdictPASS) {
		t.Errorf("a coherent chain beside a PASS narrative must record agreement, got %+v", agree)
	}

	broken := withLink(fullChain(), LinkNarrative, StatusIncoherent, "claims a fix the diff lacks")
	dis := Shadow(7, "audit", RenderChainBlock(broken), "PASS", full)
	if dis.Agrees || dis.ChainVerdict != string(VerdictFAIL) {
		t.Errorf("a PASS narrative over an incoherent link must record DISAGREEMENT, got %+v", dis)
	}
	if len(dis.Diagnoses) == 0 {
		t.Error("the disagreement must carry the human-recognisable name, or the operator re-derives it by hand")
	}

	absent := Shadow(7, "audit", "# Audit Report\n\n## Verdict\n**PASS**\n", "PASS", full)
	if absent.ChainPresent || absent.Absence == "" {
		t.Errorf("an absent chain must be recorded as absent with its reason, got %+v", absent)
	}
	if absent.Agrees {
		t.Error("absence must never be recorded as agreement — that would count un-measured cycles as successes")
	}

	blind := Shadow(7, "audit", RenderChainBlock(fullChain()), "PASS", []string{"build-report.md"})
	if len(blind.MissingEvidence) == 0 {
		t.Error("the record must name what the judge was never given")
	}
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
