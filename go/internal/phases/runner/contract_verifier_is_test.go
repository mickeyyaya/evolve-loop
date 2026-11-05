package runner

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
)

func TestBaseRunnerContractVerifierIs_ComparesTheBoundVerifierAtCallTime(t *testing.T) {
	gate := deliverable.NewReviewerStage(config.StageEnforce, config.StageEnforce)
	other := deliverable.NewReviewerStage(config.StageEnforce, config.StageEnforce)
	var bound ContractVerifier = deliverable.PlainVerifier{PhaseIO: config.StageEnforce}
	r := New(Options{Hooks: &fakeHooks{phase: "audit"}, ContractVerifier: func() ContractVerifier { return bound }})
	if r.ContractVerifierIs(gate) {
		t.Fatal("before the late bind the runner holds the plain verifier, not the gate")
	}
	bound = gate
	if !r.ContractVerifierIs(gate) {
		t.Fatal("after the late bind the runner judges with the gate it was bound to")
	}
	if r.ContractVerifierIs(other) {
		t.Fatal("an equal but distinct reviewer is not the bound verifier")
	}
	if New(Options{Hooks: &fakeHooks{phase: "audit"}}).ContractVerifierIs(nil) {
		t.Fatal("a runner with no verifier reports no identity, not a match with nil")
	}
}
