package inboxstamps

import (
	"context"
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

const (
	liveItem   = Pathspec + "live.json"
	liveBody   = `{"id":"live","weight":0.5,"failure_count":1}`
	stampedRaw = `{"id":"live","weight":0.5,"route":"console-manual","routed_reason":"protected","failure_count":2}`
)

func stampRepo(t *testing.T) *gittest.Repo {
	t.Helper()
	repo := gittest.Fixture(t)
	write(t, repo, liveItem, liveBody)
	write(t, repo, sourceFile, "package x\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "seed")
	return repo
}

func commitAll(repo *gittest.Repo, subject string) string {
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", subject)
	return repo.Git("rev-parse", "HEAD")
}

func TestIsStampCommit_FollowsTheWriterContract(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		subject string
		mutate  func(t *testing.T, r *gittest.Repo)
		want    bool
	}{
		{"kept stamp of mover keys", keptMessage, func(t *testing.T, r *gittest.Repo) { write(t, r, liveItem, stampedRaw) }, true},
		{"replay stamp of mover keys", replayMessage, func(t *testing.T, r *gittest.Repo) { write(t, r, liveItem, stampedRaw) }, true},
		{"quarantine move with mover keys", moveMessage, func(t *testing.T, r *gittest.Repo) {
			r.Git("rm", "-q", liveItem)
			write(t, r, quarantineDir+"live.json", stampedRaw)
		}, true},
		{"consumed move with mover keys", moveMessage, func(t *testing.T, r *gittest.Repo) {
			r.Git("rm", "-q", liveItem)
			write(t, r, consumedDir+"live.json", stampedRaw)
		}, true},
		{"lone root deletion", moveMessage, func(t *testing.T, r *gittest.Repo) { r.Git("rm", "-q", liveItem) }, true},
		{"injected task under kept", keptMessage, func(t *testing.T, r *gittest.Repo) { write(t, r, Pathspec+"injected-task.json", `{"id":"x"}`) }, false},
		{"injected task under move", moveMessage, func(t *testing.T, r *gittest.Repo) { write(t, r, Pathspec+"injected-task.json", `{"id":"x"}`) }, false},
		{"author-owned key under kept", keptMessage, func(t *testing.T, r *gittest.Repo) {
			write(t, r, liveItem, `{"id":"live","weight":0.5,"notes":"x","failure_count":1}`)
		}, false},
		{"add under quarantine without a deletion", moveMessage, func(t *testing.T, r *gittest.Repo) { write(t, r, quarantineDir+"live.json", liveBody) }, false},
		{"move differing by an author-owned key", moveMessage, func(t *testing.T, r *gittest.Repo) {
			r.Git("rm", "-q", liveItem)
			write(t, r, quarantineDir+"live.json", `{"id":"live","weight":0.5,"notes":"x","failure_count":1}`)
		}, false},
		{"move with a non-moving extra add", moveMessage, func(t *testing.T, r *gittest.Repo) {
			r.Git("rm", "-q", liveItem)
			write(t, r, quarantineDir+"live.json", liveBody)
			write(t, r, quarantineDir+"other.json", liveBody)
		}, false},
		{"entry under processed", keptMessage, func(t *testing.T, r *gittest.Repo) {
			write(t, r, liveItem, stampedRaw)
			write(t, r, processedDir+"live.json", liveBody)
		}, false},
		{"modification under move subject", moveMessage, func(t *testing.T, r *gittest.Repo) { write(t, r, liveItem, stampedRaw) }, false},
		{"code path under kept", keptMessage, func(t *testing.T, r *gittest.Repo) {
			write(t, r, liveItem, stampedRaw)
			write(t, r, sourceFile, "package x\n\nvar A = 1\n")
		}, false},
		{"code deletion under move", moveMessage, func(t *testing.T, r *gittest.Repo) { r.Git("rm", "-q", sourceFile) }, false},
		{"root deletion plus code deletion under move", moveMessage, func(t *testing.T, r *gittest.Repo) {
			r.Git("rm", "-q", liveItem)
			r.Git("rm", "-q", sourceFile)
		}, false},
		{"move rewriting a curated key", moveMessage, func(t *testing.T, r *gittest.Repo) {
			r.Git("rm", "-q", liveItem)
			write(t, r, quarantineDir+"live.json", `{"id":"live","weight":0.5,"failure_count":1,"fix":"evil"}`)
		}, false},
		{"one deletion feeding two adds", moveMessage, func(t *testing.T, r *gittest.Repo) {
			r.Git("rm", "-q", liveItem)
			write(t, r, quarantineDir+"live.json", stampedRaw)
			write(t, r, consumedDir+"live.json", stampedRaw)
		}, false},
		{"stamp of a tracked processed item", keptMessage, func(t *testing.T, r *gittest.Repo) {
			write(t, r, processedDir+"old.json", liveBody)
			r.Git("add", "-f", processedDir+"old.json")
			r.Git("commit", "-q", "-m", "seed processed")
			write(t, r, processedDir+"old.json", stampedRaw)
		}, false},
		{"unknown subject", "chore: other", func(t *testing.T, r *gittest.Repo) { write(t, r, liveItem, stampedRaw) }, false},
		{"empty commit", keptMessage, func(t *testing.T, r *gittest.Repo) {}, false},
		{"unchanged item", keptMessage, func(t *testing.T, r *gittest.Repo) {
			write(t, r, liveItem, `{"weight":0.5,"id":"live","failure_count":1}`)
		}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			repo := stampRepo(t)
			c.mutate(t, repo)
			sha := commitAllAllowEmpty(repo, c.subject)
			got, err := IsStampCommit(context.Background(), git(repo), sha, c.subject)
			if err != nil || got != c.want {
				t.Errorf("IsStampCommit = (%v, %v), want %v", got, err, c.want)
			}
		})
	}
}

func commitAllAllowEmpty(repo *gittest.Repo, subject string) string {
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "--allow-empty", "-m", subject)
	return repo.Git("rev-parse", "HEAD")
}

func TestIsStampCommit_ANonJSONItemIsAnError(t *testing.T) {
	t.Parallel()
	repo := stampRepo(t)
	write(t, repo, liveItem, "not json")
	sha := commitAll(repo, keptMessage)
	if got, err := IsStampCommit(context.Background(), git(repo), sha, keptMessage); err == nil || got {
		t.Errorf("IsStampCommit = (%v, %v), want an error for a non-JSON item", got, err)
	}
}

func TestIsStampCommit_ANonJSONMovedItemIsAnError(t *testing.T) {
	t.Parallel()
	repo := stampRepo(t)
	repo.Git("rm", "-q", liveItem)
	write(t, repo, quarantineDir+"live.json", "not json")
	sha := commitAll(repo, moveMessage)
	if got, err := IsStampCommit(context.Background(), git(repo), sha, moveMessage); err == nil || got {
		t.Errorf("IsStampCommit = (%v, %v), want an error", got, err)
	}
}

func TestIsStampCommit_AGitFailureIsAnError(t *testing.T) {
	t.Parallel()
	if got, err := IsStampCommit(context.Background(), erroringGit{}, "abc", keptMessage); err == nil || got {
		t.Errorf("IsStampCommit = (%v, %v), want the git error", got, err)
	}
}

type failAfterGit struct {
	real Git
	okay int
}

func (g *failAfterGit) Capture(ctx context.Context, args ...string) (string, string, int, error) {
	if g.okay == 0 {
		return "", "", 0, errors.New("boom")
	}
	g.okay--
	return g.real.Capture(ctx, args...)
}

func TestIsStampCommit_AFailedItemReadIsAnError(t *testing.T) {
	t.Parallel()
	repo := stampRepo(t)
	write(t, repo, liveItem, stampedRaw)
	sha := commitAll(repo, keptMessage)
	for okay := 1; okay <= 3; okay++ {
		g := &failAfterGit{real: git(repo), okay: okay}
		if got, err := IsStampCommit(context.Background(), g, sha, keptMessage); err == nil || got {
			t.Errorf("okay=%d: IsStampCommit = (%v, %v), want an error", okay, got, err)
		}
	}
}

func TestIsStampCommit_ADeletionUnderKeptIsRefused(t *testing.T) {
	t.Parallel()
	repo := stampRepo(t)
	repo.Git("rm", "-q", liveItem)
	sha := commitAll(repo, keptMessage)
	if got, err := IsStampCommit(context.Background(), git(repo), sha, keptMessage); err != nil || got {
		t.Errorf("IsStampCommit = (%v, %v), want false", got, err)
	}
}

func TestIsStampCommit_AFailedMovedItemReadIsAnError(t *testing.T) {
	t.Parallel()
	repo := stampRepo(t)
	repo.Git("rm", "-q", liveItem)
	write(t, repo, quarantineDir+"live.json", stampedRaw)
	sha := commitAll(repo, moveMessage)
	for okay := 1; okay <= 4; okay++ {
		g := &failAfterGit{real: git(repo), okay: okay}
		if got, err := IsStampCommit(context.Background(), g, sha, moveMessage); err == nil || got {
			t.Errorf("okay=%d: IsStampCommit = (%v, %v), want an error", okay, got, err)
		}
	}
}
