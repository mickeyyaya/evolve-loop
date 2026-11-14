package inboxcurator

import (
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestCommit_AMergedBranchBodyReadsClosesInboxByTheTrailerGrammarOfClosesinbox(t *testing.T) {
	for name, tc := range map[string]struct {
		bodies []string
		want   []string
	}{
		"a fenced example is quoted text":              {[]string{"```\nCloses-Inbox: fenced-item\n```"}, nil},
		"a tilde fenced example is quoted text":        {[]string{"~~~text\nCloses-Inbox: tilde-item\n~~~"}, nil},
		"a claim after a closed fence still counts":    {[]string{"```\nCloses-Inbox: fenced-item\n```\nCloses-Inbox: real-item"}, []string{"real-item"}},
		"a backticked bullet marker is a claim":        {[]string{"- Closes-Inbox: `ticked-item`"}, []string{"ticked-item"}},
		"a star bullet marker is a claim":              {[]string{"* Closes-Inbox: star-item"}, []string{"star-item"}},
		"an indented marker is quoted text":            {[]string{"  Closes-Inbox: indented-item"}, nil},
		"a blockquoted marker is quoted text":          {[]string{"> Closes-Inbox: quoted-item"}, nil},
		"prose after the marker is not an id":          {[]string{"Closes-Inbox: the salvage layer item"}, nil},
		"an upper case none is no id":                  {[]string{"Closes-Inbox: NONE"}, nil},
		"a valid id survives an invalid sibling":       {[]string{"Closes-Inbox: good-item, not an id"}, []string{"good-item"}},
		"an unclosed fence hides nothing of a sibling": {[]string{"```\nan example that never closes", "Closes-Inbox: sibling-item"}, []string{"sibling-item"}},
	} {
		repo, _, _ := mergeFixture(t, map[string][]string{"feature": tc.bodies}, "feature")
		repo.Git("merge", "--no-ff", "-q", "-m", "Merge feature", "feature")

		change := commitOf(t, repo, repo.Git("rev-parse", "HEAD"))

		if !slices.Equal(change.Closes, tc.want) {
			t.Errorf("merged branch, %s: Closes = %v, want %v", name, change.Closes, tc.want)
		}
	}
}

func TestCommit_AnOrdinaryCommitBodyReadsClosesInboxByTheTrailerGrammarOfClosesinbox(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		want []string
	}{
		"a fenced example is quoted text":     {"```\nCloses-Inbox: fenced-item\n```", nil},
		"a backticked bullet marker is claim": {"- Closes-Inbox: `ticked-item`", []string{"ticked-item"}},
		"an upper case none is no id":         {"Closes-Inbox: NONE", nil},
		"prose after the marker is not an id": {"Closes-Inbox: the salvage layer item", nil},
	} {
		repo := gittest.Fixture(t)
		repo.Write("a.txt", "x\n")
		repo.Git("add", ".")
		repo.Git("commit", "-q", "-m", "work", "-m", tc.body)

		change := commitOf(t, repo, repo.Git("rev-parse", "HEAD"))

		if !slices.Equal(change.Closes, tc.want) {
			t.Errorf("ordinary commit, %s: Closes = %v, want %v", name, change.Closes, tc.want)
		}
	}
}
