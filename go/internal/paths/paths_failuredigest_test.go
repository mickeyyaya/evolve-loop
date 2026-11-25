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

func TestBlockerRecordPath_SitsBesideTheDigest(t *testing.T) {
	t.Parallel()
	evolveDir := filepath.Join("root", ".evolve")
	got := BlockerRecordPath(evolveDir, 2049)
	if filepath.Dir(got) != filepath.Dir(FailureDigestRecordPath(evolveDir, 2049)) || filepath.Base(got) != "cycle-2049.blocker.json" {
		t.Fatalf("BlockerRecordPath = %q; want cycle-2049.blocker.json in the failure-digest home", got)
	}
}
