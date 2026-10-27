package ship

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func auditedLaneThenMainMoved(t *testing.T) laneCarriedOntoAPeer {
	t.Helper()
	repo := makeRepo(t)
	rev := func(dir string, args ...string) string { return strings.TrimSpace(runGitOut(t, dir, args...)) }
	base0 := rev(repo, "rev-parse", "HEAD")
	wt := tempRepoDir(t)
	runGit(t, repo, "worktree", "add", "-b", "cycle-1", wt)
	mustWrite(t, filepath.Join(wt, "lane.txt"), "the lane's audited change\n")
	if err := writeStateMap(filepath.Join(repo, ".evolve", "cycle-state.json"), map[string]any{"active_worktree": wt}); err != nil {
		t.Fatal(err)
	}
	seedAudit(t, repo, "PASS")
	tree0 := rev(wt, "write-tree")
	mustWrite(t, filepath.Join(repo, "peer.txt"), "a peer landing after the audit\n")
	runGit(t, repo, "add", "peer.txt")
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "peer")
	base1 := rev(repo, "rev-parse", "HEAD")
	runGit(t, wt, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "lane")
	runGit(t, wt, "-c", "commit.gpgsign=false", "rebase", "-q", base1)
	runGit(t, wt, "reset", "-q", "--soft", base1)
	return laneCarriedOntoAPeer{
		lane: carriedLane{repo: repo, worktree: wt, base0: base0, tree0: tree0, base1: base1, tree1: rev(wt, "write-tree"), cycle: 1715},
		ref:  mustHashFile(t, filepath.Join(repo, ".evolve", "runs", "cycle-1", "audit-report.md")),
	}
}

func (c laneCarriedOntoAPeer) shipRecording(t *testing.T) (error, []signalcenter.Event) {
	t.Helper()
	opts := auditOpts(t, c.lane.repo)
	opts.ActiveWorktree = c.lane.worktree
	center, got := recordingCenter()
	opts.Signals = center
	return verifyAuditBinding(context.Background(), opts, &RunResult{}), *got
}

func carryNotReProvenFrom(events []signalcenter.Event, origin string) []signalcenter.Event {
	var found []signalcenter.Event
	for _, e := range events {
		if e.Code == codeCarryNotReProven && e.Origin == origin {
			found = append(found, e)
		}
	}
	return found
}

func TestVerifyAuditBinding_AMainThatMovedAfterTheAuditShipsOnTheCarryThatNamesIt(t *testing.T) {
	t.Parallel()
	c := auditedLaneThenMainMoved(t)
	writeCarry(t, c.lane, c.ref, c.lane.tree0)

	res, err := c.ship(t)

	if err != nil {
		t.Fatalf("verifyAuditBinding = %v; main moved after the audit, the lane rebased byte for byte and its carry names the new main, so the audit re-binds with no audit round (cycles 2058 to 2063)", err)
	}
	if logs := strings.Join(res.Logs, "\n"); !strings.Contains(logs, "re-bound to "+c.lane.base1) || !strings.Contains(logs, "carry of cycle 1715") {
		t.Errorf("logs = %q, want the binding to name the new HEAD and the carry that re-bound it", logs)
	}
}

func TestVerifyAuditBinding_AMovedMainIsHeadMovedWithoutACarryForThatHead(t *testing.T) {
	t.Parallel()
	t.Run("no carry", func(t *testing.T) {
		c := auditedLaneThenMainMoved(t)

		err, events := c.shipRecording(t)

		wantShipErr(t, err, core.CodeAuditBindingHeadMoved, core.ShipClassPrecondition, "carry not re-bound: no identical-rebase carry names audit "+c.ref)
		if signals := carryNotReProvenFrom(events, "carryReboundHead"); len(signals) != 0 {
			t.Fatalf("signals = %+v, want none: no carry was found to re-prove", signals)
		}
	})
	t.Run("main moved again after the carry", func(t *testing.T) {
		c := auditedLaneThenMainMoved(t)
		writeCarry(t, c.lane, c.ref, c.lane.tree0)
		mustWrite(t, filepath.Join(c.lane.repo, "later.txt"), "a later peer\n")
		runGit(t, c.lane.repo, "add", "later.txt")
		runGit(t, c.lane.repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "later peer")

		_, err := c.ship(t)

		wantShipErr(t, err, core.CodeAuditBindingHeadMoved, core.ShipClassPrecondition, "carry not re-bound: the carry of cycle 1715 names main at "+c.lane.base1)
	})
	t.Run("a carry the ship cannot re-prove", func(t *testing.T) {
		c := auditedLaneThenMainMoved(t)
		writeCarry(t, c.lane, c.ref, c.lane.tree0)
		appendACopyOutsideTheChain(t, c.lane)

		err, events := c.shipRecording(t)

		wantShipErr(t, err, core.CodeAuditBindingHeadMoved, core.ShipClassPrecondition, "carry not re-bound: the ledger does not verify")
		if signals := carryNotReProvenFrom(events, "carryReboundHead"); len(signals) != 1 || signals[0].Fields["carry_cycle"] != "1716" {
			t.Fatalf("signals = %+v, want one SHIP_CARRY_NOT_REPROVEN for the out-of-chain copy, cycle 1716", signals)
		}
	})
	t.Run("an in-place ship", func(t *testing.T) {
		c := auditedLaneThenMainMoved(t)
		writeCarry(t, c.lane, c.ref, c.lane.tree0)
		opts := auditOpts(t, c.lane.repo)

		err := verifyAuditBinding(context.Background(), opts, &RunResult{})

		wantShipErr(t, err, core.CodeAuditBindingHeadMoved, core.ShipClassPrecondition, "git HEAD has moved since audit")
	})
}
