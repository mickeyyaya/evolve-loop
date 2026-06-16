package releasepipeline

import (
	"errors"
	"testing"
)

// TestResult_RunBadTargetReturnsSeededResult names the releasepipeline.Result
// type (Run returns it but the bare type is never named in a test) and pins that
// Run threads opts.Target into the returned Result even on the earliest
// (semver-validation) failure path, before any pipeline step runs.
func TestResult_RunBadTargetReturnsSeededResult(t *testing.T) {
	var got Result
	got, err := Run(Options{Target: "not-semver"})
	if !errors.Is(err, ErrPrePublishFailed) {
		t.Fatalf("err=%v, want ErrPrePublishFailed", err)
	}
	if got.Target != "not-semver" {
		t.Errorf("Result.Target=%q, want %q", got.Target, "not-semver")
	}
	if len(got.StepsCompleted) != 0 {
		t.Errorf("StepsCompleted=%v, want none (failed before any step)", got.StepsCompleted)
	}
	if got.RollbackTriggered {
		t.Error("RollbackTriggered=true, want false on pre-publish validation failure")
	}
}
