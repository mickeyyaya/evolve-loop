//go:build integration

package shiptest_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/shiptest"
)

func TestShipDirect_LegitimateBoundShipIsNotBlocked(t *testing.T) {
	t.Parallel()
	repo := shiptest.MakeRepo(t)
	shiptest.AddRemote(t, repo)
	shiptest.RunGit(t, repo, "push", "-q", "origin", "main")

	shiptest.MustWrite(t, filepath.Join(repo, "direct-change.txt"), "the cycle's change\n")
	shiptest.SeedAudit(t, repo, "PASS")

	res, err := runShip(t, repo, ship.Options{
		Class:         ship.ClassCycle,
		CommitMessage: "feat: direct-path change",
	})
	if err != nil {
		t.Fatalf("a legitimate bound direct ship must not be blocked, but it was rejected: %v\nlogs: %v", err, res.Logs)
	}
	var verified bool
	for _, log := range res.Logs {
		if strings.Contains(log, "tree-SHA binding verified") {
			verified = true
		}
	}
	if !verified {
		t.Errorf("the ship succeeded but never logged a binding verification — the guard was inert rather than satisfied; logs: %v", res.Logs)
	}
}
