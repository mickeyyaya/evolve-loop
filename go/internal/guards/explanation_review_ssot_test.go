package guards

import (
	"os"
	"strings"
	"testing"
)

// explanationReviewGateCallee is package-level so the call-site scanner's vocabulary reads the same name.
const explanationReviewGateCallee = "explanationdocs.ValidateReviewedHandoff"

// explanationReviewGateFiles is package-level so the call-site scanner's vocabulary reads the same files.
var explanationReviewGateFiles = []string{
	"../phases/audit/explanation_review_gate.go",
	"../phases/retro/explanation_review_gate.go",
}

func TestExplanationReviewGates_ShareContractCore(t *testing.T) {
	for _, path := range explanationReviewGateFiles {
		t.Run(path, func(t *testing.T) {
			if !functionCalls(t, path, "validateExplanationReview", explanationReviewGateCallee) {
				t.Fatalf("validateExplanationReview must delegate the contract core to explanationdocs.ValidateReviewedHandoff")
			}
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(body), "must be VERIFIED or NEEDS_CORRECTION") {
				t.Fatalf("%s restates the status-enum belief; it must live only in explanationdocs", path)
			}
			// See ADR-0102.
			if !functionCalls(t, path, "validateExplanationReview", "reportdoc.ReasonedReview") {
				t.Fatalf("%s: validateExplanationReview must read the review through reportdoc.ReasonedReview", path)
			}
			for _, forked := range []string{"reportdoc.Section", "reportdoc.ReviewFields", "reportdoc.RequireReasoning"} {
				if functionCalls(t, path, "validateExplanationReview", forked) {
					t.Fatalf("%s: validateExplanationReview calls %s directly — the ladder lives in reportdoc.ReasonedReview", path, forked)
				}
			}
		})
	}
}
