package guards

import (
	"os"
	"strings"
	"testing"
)

// TestExplanationReviewGates_ShareContractCore pins the single-sourcing of the
// explanation-review contract (architecture review 2026-09-01, CRITICAL): the
// status enum, build-status match, required/not_applicable document switch,
// and evidence-reference check were stated twice — once per phase gate — with
// nothing binding the copies. Both gates must now delegate the shared core to
// explanationdocs.ValidateReviewedHandoff, and the belief literal must not
// reappear in either phase package. Per-phase policy (heading, missing-handoff
// verdict, NEEDS_CORRECTION disposition) legitimately stays in the gates.
func TestExplanationReviewGates_ShareContractCore(t *testing.T) {
	gates := []string{
		"../phases/audit/explanation_review_gate.go",
		"../phases/retro/explanation_review_gate.go",
	}
	for _, path := range gates {
		t.Run(path, func(t *testing.T) {
			if !functionCalls(t, path, "validateExplanationReview", "explanationdocs.ValidateReviewedHandoff") {
				t.Fatalf("validateExplanationReview must delegate the contract core to explanationdocs.ValidateReviewedHandoff")
			}
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			// The enum belief lives in explanationdocs only; a copy here means
			// the contract forked again.
			if strings.Contains(string(body), "must be VERIFIED or NEEDS_CORRECTION") {
				t.Fatalf("%s restates the status-enum belief; it must live only in explanationdocs", path)
			}
		})
	}
}
