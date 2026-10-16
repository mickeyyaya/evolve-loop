package releasepreflight

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestWIPCommitLines_MatchesOnlyAnchoredWIPPrefixes(t *testing.T) {
	gitLog := strings.Join([]string{
		"a1 WIP refactor gate",
		"a2 fixup! feat: x",
		"a3 squash! feat: x",
		"a4 amend! feat: x",
		"a5 feat: clean",
		"a6 docs: explain fixup! and WIP prefixes",
		"a7 wip lower case",
		"",
	}, "\n")

	got := wipCommitLines(gitLog)

	want := []string{"a1 WIP refactor gate", "a2 fixup! feat: x", "a3 squash! feat: x", "a4 amend! feat: x"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("wipCommitLines = %q, want %q", got, want)
	}
}

func TestRun_RefusesWIPCommitsAndNamesEachOne(t *testing.T) {
	opts := stubOpts(makeRepo(t, "1.0.0"), "1.0.1")
	opts.WIPCommits = func(string) ([]string, error) {
		return []string{"abc1234 fixup! feat: x", "def5678 WIP gate"}, nil
	}

	_, err := Run(opts)

	if !errors.Is(err, ErrCheckFailed) {
		t.Fatalf("err = %v, want ErrCheckFailed", err)
	}
	for _, want := range []string{"2 WIP commit(s)", "abc1234 fixup! feat: x", "def5678 WIP gate"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %v, want it to contain %q", err, want)
		}
	}
}

func TestRun_WIPScanErrorFailsClosed(t *testing.T) {
	opts := stubOpts(makeRepo(t, "1.0.0"), "1.0.1")
	opts.WIPCommits = func(string) ([]string, error) { return nil, errors.New("git log HEAD: exit status 128") }

	_, err := Run(opts)

	if !errors.Is(err, ErrCheckFailed) || !strings.Contains(err.Error(), "WIP-commit scan error: git log HEAD") {
		t.Fatalf("err = %v, want ErrCheckFailed naming the scan error", err)
	}
}

func TestRun_DryRunDoesNotScanForWIPCommits(t *testing.T) {
	scanned := false
	opts := Options{Target: "1.0.1", RepoRoot: makeRepo(t, "1.0.0"), DryRun: true}
	opts.WIPCommits = func(string) ([]string, error) {
		scanned = true
		return []string{"abc1234 WIP"}, nil
	}

	if _, err := Run(opts); err != nil || scanned {
		t.Fatalf("dry-run: err = %v, scanned = %v; want nil and no scan", err, scanned)
	}
}

func TestRun_RequireCIFailsClosedOnlyOnAnUnavailableVerdict(t *testing.T) {
	cases := []struct {
		name       string
		requireCI  bool
		conclusion string
		wantErr    bool
	}{
		{"unavailable verdict with --require-ci", true, ciConclusionUnavailable, true},
		{"unavailable verdict stays advisory", false, ciConclusionUnavailable, false},
		{"green verdict with --require-ci", true, "success", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			opts := stubOpts(makeRepo(t, "1.0.0"), "1.0.1")
			opts.Stderr = &buf
			opts.RequireCI = tc.requireCI
			opts.CIConclusion = func(string) (CIRunStatus, error) { return CIRunStatus{Conclusion: tc.conclusion}, nil }

			_, err := Run(opts)

			if tc.wantErr != (err != nil) {
				t.Fatalf("err = %v, wantErr %v\nlog=%s", err, tc.wantErr, buf.String())
			}
			if tc.wantErr && (!errors.Is(err, ErrCheckFailed) || !strings.Contains(err.Error(), "--require-ci")) {
				t.Errorf("err = %v, want ErrCheckFailed naming --require-ci", err)
			}
		})
	}
}

func TestDefaultWIPCommits_ScansSinceTheLastTagOrTheWholeHistory(t *testing.T) {
	repo := gittest.Fixture(t)
	repo.Git("commit", "-q", "--allow-empty", "-m", "fixup! before the tag")
	got, err := defaultWIPCommits(repo.Dir)
	if err != nil || len(got) != 1 || !strings.HasSuffix(got[0], " fixup! before the tag") {
		t.Fatalf("untagged repo: defaultWIPCommits = %q, %v; want the one fixup! commit", got, err)
	}

	repo.Git("tag", "v1.0.0")
	repo.Git("commit", "-q", "--allow-empty", "-m", "feat: clean")
	repo.Git("commit", "-q", "--allow-empty", "-m", "squash! feat: clean")
	got, err = defaultWIPCommits(repo.Dir)
	short := repo.Git("rev-parse", "--short", "HEAD")
	if err != nil || !reflect.DeepEqual(got, []string{short + " squash! feat: clean"}) {
		t.Fatalf("tagged repo: defaultWIPCommits = %q, %v; want only the squash! commit after v1.0.0", got, err)
	}
}

func TestDefaultWIPCommits_UnbornHeadIsAnError(t *testing.T) {
	repo := gittest.Fixture(t)

	got, err := defaultWIPCommits(repo.Dir)

	if got != nil || err == nil || !strings.HasPrefix(err.Error(), "git log HEAD: ") {
		t.Fatalf("defaultWIPCommits = %q, %v; want nil and the git log failure", got, err)
	}
}
