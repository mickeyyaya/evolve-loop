//go:build integration

package ship

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestRun_AForgedIntentNeverShipsAnAuditFAIL(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	seedAudit(t, l.Repo, "FAIL")
	runGit(t, l.WT, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "not a ship commit")
	forged := l.Tip(t)
	tree := strings.TrimSpace(runGitOut(t, l.WT, "rev-parse", "HEAD^{tree}"))
	view, err := explanationViewSHA(l.Repo, landingCycle, landingRunID)
	if err != nil {
		t.Fatal(err)
	}
	l.WriteIntentMap(t, map[string]any{"cycle": landingCycle, "run_id": landingRunID, "audit_artifact_sha256": mustHashFile(t, filepath.Join(l.WS, "audit-report.md")),
		"audited_tree": tree, "lane_tree": tree, "worktree_base_sha": l.Base, "commit_sha": forged, "commit_tree": tree,
		"explanation_view_sha256": view, "pre_main": l.Base, "branch": "main", "status": "prepared"})

	res, err := runShip(t, l.Repo, l.options(execRunner, &sleepLog{}))

	if err == nil || res.ExitCode == ExitOK || remoteHeadSHA(t, l.Repo) == forged {
		t.Errorf("exit=%d origin=%s: an intent that no ship journaled, over an audit FAIL, never ships %s", res.ExitCode, remoteHeadSHA(t, l.Repo), forged)
	}
}

func (l landingLane) carryOntoAPeer(t *testing.T) (string, string) {
	t.Helper()
	base1 := pushOnTopOf(t, l.Bare, l.Base)
	runGit(t, l.Repo, "fetch", "-q", "origin", "main")
	runGit(t, l.Repo, "merge", "--ff-only", "-q", "origin/main")
	runGit(t, l.WT, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "the audited lane change")
	runGit(t, l.WT, "-c", "commit.gpgsign=false", "rebase", "-q", base1)
	runGit(t, l.WT, "reset", "-q", "--soft", base1)
	tree1 := strings.TrimSpace(runGitOut(t, l.WT, "write-tree"))
	writeCarry(t, carriedLane{repo: l.Repo, worktree: l.WT, base0: l.Base, tree0: l.Audited, base1: base1, tree1: tree1, cycle: landingCycle},
		mustHashFile(t, filepath.Join(l.WS, "audit-report.md")), l.Audited)
	return base1, tree1
}

func (l landingLane) prepareACarriedLanding(t *testing.T, base1, tree1 string) string {
	t.Helper()
	consumed := ".evolve/inbox/consumed/" + filepath.Base(landingItem)
	mustWrite(t, filepath.Join(l.WT, filepath.FromSlash(consumed)), `{"id":"`+landingItemID+`","consumed":{"via":"ship"}}`+"\n")
	runGit(t, l.WT, "rm", "-q", landingItem)
	runGit(t, l.WT, "add", consumed)
	runGit(t, l.WT, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "evolve-cycle 1830")
	commit := l.Tip(t)
	journal, err := json.Marshal(map[string]any{"sha": commit, "class": "cycle", "ts": "2026-10-08T00:00:00Z", "cycle": landingCycle})
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(l.Repo, ".evolve", "ship-journal.jsonl"), string(journal)+"\n")
	view, err := explanationViewSHA(l.Repo, landingCycle, landingRunID)
	if err != nil {
		t.Fatal(err)
	}
	l.WriteIntentMap(t, map[string]any{"cycle": landingCycle, "run_id": landingRunID,
		"audit_artifact_sha256": mustHashFile(t, filepath.Join(l.WS, "audit-report.md")), "audited_tree": l.Audited, "lane_tree": tree1,
		"worktree_base_sha": base1, "commit_sha": commit, "commit_tree": strings.TrimSpace(runGitOut(t, l.WT, "rev-parse", "HEAD^{tree}")),
		"consumed_paths": []string{landingItem, consumed}, "explanation_view_sha256": view, "pre_main": base1,
		"lane_branch": landingBranch, "branch": "main", "status": "prepared"})
	return commit
}

func TestRun_APolicyRefusalOfACarriedLaneUnwindsItToItsCarriedTree(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	base1, tree1 := l.carryOntoAPeer(t)
	l.prepareACarriedLanding(t, base1, tree1)
	outage := &pushOutage{Failures: 1 << 20, Stderr: githubPolicyError}

	_, err := runShip(t, l.Repo, l.options(outage.Runner(), &sleepLog{}))

	wantShipErr(t, err, "GIT_PUSH_POLICY_REFUSED", core.ShipClassPrecondition, "")
	head := strings.TrimSpace(runGitOut(t, l.WT, "rev-parse", "HEAD"))
	staged := strings.TrimSpace(runGitOut(t, l.WT, "write-tree"))
	if head != base1 || staged != tree1 {
		t.Errorf("lane HEAD=%s staged=%s, want the carried change %s pending on the peer's base %s: the unwind restores the lane's own tree, not the tree Audit saw on the old base", head, staged, tree1, base1)
	}
	if status := l.Intent(t)["status"]; status != "unwound" {
		t.Errorf("intent status %v, want unwound", status)
	}
}
