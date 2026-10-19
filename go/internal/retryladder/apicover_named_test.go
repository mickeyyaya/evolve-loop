package retryladder_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

func TestExportsAreNamed(t *testing.T) {
	facts := retryladder.Facts{
		ExitCode:           85,
		QuotaBench:         true,
		TeardownError:      false,
		AuditClass:         "code-build-fail",
		ProvenBlocker:      false,
		LaneScope:          []string{"go/internal/foo"},
		Floors:             []retryladder.FloorFailure{{Path: "go/internal/foo/a.go", Protected: false}},
		DeliverableMissing: false,
		Reason:             "[missing_section] heading absent",
	}
	var verdict retryladder.Verdict = retryladder.Classify(facts)
	var class retryladder.Class = verdict.Class
	if class != retryladder.Quota {
		t.Fatalf("85 with a bench = %q, want %q", class, retryladder.Quota)
	}
	if verdict.Fingerprint != retryladder.Fingerprint(facts.Reason) {
		t.Fatalf("verdict fingerprint = %q, want %q", verdict.Fingerprint, retryladder.Fingerprint(facts.Reason))
	}
	want := map[retryladder.Class]string{
		retryladder.Infra:      "infra",
		retryladder.Quota:      "quota",
		retryladder.Halt:       "halt",
		retryladder.Spec:       "spec",
		retryladder.Capability: "capability",
		retryladder.Unknown:    "unknown",
	}
	for c, name := range want {
		if string(c) != name {
			t.Fatalf("class %q, want %q", c, name)
		}
	}
}
