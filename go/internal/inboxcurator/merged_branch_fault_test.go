package inboxcurator

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestCommit_AFaultReadingTheMergedBranchIsAnError(t *testing.T) {
	repo, _, _ := mergeFixture(t, map[string][]string{"feature": {"Closes-Inbox: a-item"}}, "feature")
	repo.Git("merge", "--no-ff", "-q", "-m", "Merge feature", "feature")
	head := repo.Git("rev-parse", "HEAD")
	gittest.FaultOn(t, "rev-list")

	if _, err := Commit(context.Background(), gitexec.Default(repo.Dir), head); err == nil || !strings.Contains(err.Error(), "read the merged branch of commit") {
		t.Errorf("Commit = %v, want the merged branch fault named", err)
	}
}
