package paths

import (
	"path/filepath"
	"testing"
)

func TestFailureDigestRecordPath_IsTheHarnessRecordOfOneCycle(t *testing.T) {
	t.Parallel()
	got := FailureDigestRecordPath(filepath.Join("root", ".evolve"), 2055)
	if want := filepath.Join("root", ".evolve", FailureDigestsDir, "cycle-2055.json"); got != want || FailureDigestsDir != "failure-digests" {
		t.Fatalf("FailureDigestRecordPath = %q (dir %q); want %q", got, FailureDigestsDir, want)
	}
}
