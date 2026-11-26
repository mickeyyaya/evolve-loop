package ship

import (
	"context"
	"os"
	"testing"
)

func pushOnlyDryRun(root string) (RunResult, error) {
	return Run(context.Background(), Options{
		PushOnly:    true,
		DryRun:      true,
		ProjectRoot: root,
		Stdout:      os.Stderr,
		Stderr:      os.Stderr,
	})
}

func TestPushOnly_ADryRunLeavesOriginUnchanged(t *testing.T) {
	t.Parallel()
	root, run := initPushOnlyRepo(t)
	commitFile(t, root, run, "knowledge-base/cycles/cycle-9.json", "dossier: cycle-9 closeout")
	before := run(root, "ls-remote", "origin", "main")

	if _, err := pushOnlyDryRun(root); err != nil {
		t.Fatalf("a dry run of a provenanced ahead set must pass, got %v", err)
	}
	if after := run(root, "ls-remote", "origin", "main"); after != before {
		t.Errorf("a dry run moved origin/main: before %q, after %q", before, after)
	}
}

func TestPushOnly_ADryRunStillRefusesAnUnprovenancedCommit(t *testing.T) {
	t.Parallel()
	root, run := initPushOnlyRepo(t)
	sha := commitFile(t, root, run, "a.txt", "not shipped")

	_, err := pushOnlyDryRun(root)
	assertNothingPushed(t, root, run, sha, err)
}
