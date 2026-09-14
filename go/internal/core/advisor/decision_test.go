package advisor

import (
	"regexp"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// Test 12 — the decision table projects the contract registry (the artifact
// the model writes is spelled ONCE, in phasecontract), the capture kind is a
// confined token (the former isSafeArtifactKind intent), and every per-call
// literal sits in one row. The zero value is the initial Plan (the former
// planStage's zero value).
func TestDecision_TableProjectsTheContractRegistry(t *testing.T) {
	safeKind := regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)
	rows := []struct {
		d                                                 decision
		contract, artifact, kind, completion, pfx, origin string
		depth                                             int
	}{
		{decisionPlan, "router", "routing-plan.json", "plan", "artifact", "phase advisor", "Advisor.Plan", 0},
		{decisionRePlan, "router-replan", "routing-replan.json", "replan", "artifact", "phase advisor", "Advisor.RePlan", 1},
		{decisionProposal, "router-proposal", "routing-proposal.json", "proposal", "artifact", "routing proposer", "Advisor.Propose", 0},
	}
	for _, r := range rows {
		if r.d.contractID() != r.contract || r.d.artifactFile() != r.artifact || r.d.captureKind() != r.kind ||
			r.d.completion() != r.completion || r.d.errPfx() != r.pfx || r.d.origin() != r.origin || r.d.replanDepth() != r.depth {
			t.Errorf("%s: row drifted: %q %q %q %q %q %q %d", r.kind, r.d.contractID(), r.d.artifactFile(), r.d.captureKind(), r.d.completion(), r.d.errPfx(), r.d.origin(), r.d.replanDepth())
		}
		if got := phasecontract.ArtifactName(r.d.contractID()); got == "" || got != r.d.artifactFile() {
			t.Errorf("%s: the artifact is the registry's (%q), not a second spelling", r.kind, got)
		}
		if !safeKind.MatchString(r.d.captureKind()) {
			t.Errorf("%s: the capture kind names files and must stay a confined token", r.d.captureKind())
		}
	}
	var zero decision
	if zero != decisionPlan {
		t.Fatalf("zero-value decision = %d, want the initial plan", zero)
	}
	if decisionForArtifact("routing-replan.json") != decisionRePlan || decisionForArtifact("routing-plan.json") != decisionPlan || decisionForArtifact("other.json") != decisionPlan {
		t.Error("an artifact maps back onto its decision: the re-plan artifact ⇒ RePlan, anything else ⇒ Plan")
	}
}
