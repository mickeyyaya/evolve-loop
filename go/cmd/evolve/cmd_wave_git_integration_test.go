//go:build integration

package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestGitWaveAdapters_ReadTheHeadAndTheMergedPRs(t *testing.T) {
	r := gittest.Fixture(t)
	r.Git("commit", "-q", "--allow-empty", "-m", "base")
	base := r.Git("rev-parse", "HEAD")
	r.Git("checkout", "-q", "-b", "feature")
	r.Git("commit", "-q", "--allow-empty", "-m", "feat: x")
	r.Git("checkout", "-q", "main")
	r.Git("merge", "-q", "--no-ff", "-m", "Merge pull request #5 from o/feature", "feature")

	head, headErr := gitMainHead(r.Dir)
	prs, prsErr := gitMergedPRs(r.Dir, base)
	_, badErr := gitMergedPRs(r.Dir, "no-such-ref")

	if headErr != nil || head != r.Git("rev-parse", "HEAD") {
		t.Errorf("gitMainHead = %q, %v; want HEAD", head, headErr)
	}
	if prsErr != nil || !slices.Equal(prs, []string{"5"}) {
		t.Errorf("gitMergedPRs = %q, %v; want [5]", prs, prsErr)
	}
	if badErr == nil || !strings.Contains(badErr.Error(), "list the merges since no-such-ref") {
		t.Errorf("gitMergedPRs(bad ref) error = %v, want one that names the ref", badErr)
	}
}
