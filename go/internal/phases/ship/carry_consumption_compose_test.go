package ship

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const (
	laneInboxItem    = ".evolve/inbox/lane-item.json"
	laneConsumedItem = ".evolve/inbox/consumed/lane-item.json"
	cycle1825Shape   = "cycle 1825's shape: the carry explains audited->carried (the peer's paths) and the re-ship's consumption explains carried->staged; neither alone explains audited->staged"
)

func laneWithAnInboxItem(t *testing.T) carriedLane {
	t.Helper()
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, laneInboxItem), "{\"id\":\"lane-item\"}\n")
	runGit(t, repo, "add", "-f", laneInboxItem)
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "the lane's inbox item")
	return rebasedLaneOn(t, repo, 1825)
}

func reShipConsumes(t *testing.T, l carriedLane, opts *Options) string {
	t.Helper()
	runGit(t, l.repo, "rm", "-q", "--cached", laneInboxItem)
	mustWrite(t, filepath.Join(l.repo, laneConsumedItem), "{\"id\":\"lane-item\",\"consumed\":{\"via\":\"ship\"}}\n")
	runGit(t, l.repo, "add", "-f", laneConsumedItem)
	opts.internalConsumedPaths = append(opts.internalConsumedPaths, laneInboxItem, laneConsumedItem)
	return strings.TrimSpace(runGitOut(t, l.repo, "write-tree"))
}

func smuggle(t *testing.T, l carriedLane) string {
	t.Helper()
	mustWrite(t, filepath.Join(l.repo, "smuggled.txt"), "never audited\n")
	runGit(t, l.repo, "add", "smuggled.txt")
	return strings.TrimSpace(runGitOut(t, l.repo, "write-tree"))
}

func land(t *testing.T, l carriedLane) {
	t.Helper()
	runGit(t, l.repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "ship")
}

func carriedLaneBound(t *testing.T) (carriedLane, *Options) {
	t.Helper()
	l := laneWithAnInboxItem(t)
	writeCarry(t, l, "audit-ref", l.tree0)
	return l, boundTo(t, l, l.tree0, "audit-ref")
}

func TestVerifyStagedTree_ACarriedRebaseShipsWithTheReShipsInboxConsumption(t *testing.T) {
	l, opts := carriedLaneBound(t)
	if err := verifyExecutionTree(context.Background(), opts, &RunResult{}, l.repo); err != nil {
		t.Fatalf("precondition: the carry re-proves on the tree before consumption, as it did live for 1825: %v", err)
	}
	reShipConsumes(t, l, opts)
	res := &RunResult{}

	err := newWorktreeShip(context.Background(), opts, res, "main", l.repo).verifyStagedTree()

	if err != nil {
		t.Fatalf("verifyStagedTree = %v; %s", err, cycle1825Shape)
	}
	if logs := strings.Join(res.Logs, "\n"); !strings.Contains(logs, "pre-commit tree drift") || !strings.Contains(logs, "carry of cycle 1825, re-proven") {
		t.Errorf("logs = %q, want the accepted pre-commit drift to name the carry that explains it", logs)
	}
}

func TestVerifyStagedTree_ACarryPlusConsumptionStillRefusesAnUnsanctionedExtraPath(t *testing.T) {
	l, opts := carriedLaneBound(t)
	reShipConsumes(t, l, opts)
	smuggle(t, l)

	err := newWorktreeShip(context.Background(), opts, &RunResult{}, "main", l.repo).verifyStagedTree()

	se := wantShipErr(t, err, core.CodeIntegrityTreeDrift, core.ShipClassIntegrity, "INTEGRITY BREACH (pre-commit)")
	if !strings.Contains(se.Message, "smuggled.txt") {
		t.Errorf("message = %q, want the refusal to name the path neither the carry nor the consumption explains", se.Message)
	}
}

func TestVerifyCommittedTree_ACarriedRebaseShipsWithTheReShipsInboxConsumption(t *testing.T) {
	l, opts := carriedLaneBound(t)
	reShipConsumes(t, l, opts)
	land(t, l)
	committed := strings.TrimSpace(runGitOut(t, l.repo, "rev-parse", "HEAD^{tree}"))
	res := &RunResult{}

	tree, err := newWorktreeShip(context.Background(), opts, res, "main", l.repo).verifyCommittedTree()

	if err != nil || tree != committed {
		t.Fatalf("verifyCommittedTree = (%q, %v), want (%q, nil); %s, after the push as before the commit", tree, err, committed, cycle1825Shape)
	}
	if logs := strings.Join(res.Logs, "\n"); !strings.Contains(logs, "post-push tree drift") || !strings.Contains(logs, "carry of cycle 1825, re-proven") {
		t.Errorf("logs = %q, want the accepted post-push drift to name the carry that explains it", logs)
	}
}

func TestVerifyCommittedTree_ACarryPlusConsumptionStillRefusesAnUnsanctionedExtraPath(t *testing.T) {
	l, opts := carriedLaneBound(t)
	reShipConsumes(t, l, opts)
	smuggle(t, l)
	land(t, l)

	tree, err := newWorktreeShip(context.Background(), opts, &RunResult{}, "main", l.repo).verifyCommittedTree()

	se := wantShipErr(t, err, core.CodeIntegrityTreeDrift, core.ShipClassIntegrity, "INTEGRITY BREACH: audit-bound tree")
	if tree != "" || !strings.Contains(se.Message, "smuggled.txt") {
		t.Errorf("verifyCommittedTree = (%q, %q), want no tree and the refusal naming the path neither the carry nor the consumption explains", tree, se.Message)
	}
}

func TestCarrySatisfied_DeclinesWhatTheReShipsConsumptionCannotCompose(t *testing.T) {
	for name, tc := range map[string]struct {
		arrange func(t *testing.T, l carriedLane) (*Options, string)
		want    func(l carriedLane, actual string) string
	}{
		"a path the consumption does not sanction": {
			func(t *testing.T, l carriedLane) (*Options, string) {
				writeCarry(t, l, "audit-ref", l.tree0)
				opts := boundTo(t, l, l.tree0, "audit-ref")
				reShipConsumes(t, l, opts)
				return opts, smuggle(t, l)
			},
			func(l carriedLane, actual string) string {
				return "the carry of cycle 1825 names the tree " + l.tree1 + ", not " + actual + " (unsanctioned drift path(s): smuggled.txt)"
			},
		},
		"a record whose patch-id is another change's": {
			func(t *testing.T, l carriedLane) (*Options, string) {
				other := []byte("diff --git a/other.txt b/other.txt\nnew file mode 100644\nindex 0000000000000000000000000000000000000000..a1b2c3d4e5f60718293a4b5c6d7e8f9012345678\n--- /dev/null\n+++ b/other.txt\n@@ -0,0 +1 @@\n+another change\n")
				writeCarryOf(t, l, "audit-ref", l.tree0, l.tree1, other, other)
				opts := boundTo(t, l, l.tree0, "audit-ref")
				return opts, reShipConsumes(t, l, opts)
			},
			func(carriedLane, string) string { return "is not the record's" },
		},
		"an audited tree that holds no change": {
			func(t *testing.T, l carriedLane) (*Options, string) {
				emptyAudit := strings.TrimSpace(runGitOut(t, l.repo, "rev-parse", l.base0+"^{tree}"))
				writeCarryStating(t, l, "audit-ref", emptyAudit, l.tree1)
				opts := boundTo(t, l, emptyAudit, "audit-ref")
				return opts, reShipConsumes(t, l, opts)
			},
			func(l carriedLane, _ string) string {
				return "the change on " + l.base1 + " is not byte for byte the audited change"
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			l := laneWithAnInboxItem(t)
			opts, actual := tc.arrange(t, l)

			ok, reason := carrySatisfied(context.Background(), opts, l.repo, actual)

			if ok || !strings.Contains(reason, tc.want(l, actual)) {
				t.Fatalf("carrySatisfied = (%v, %q), want a decline naming %q: the consumption composes with a carry only when the carry re-proves on its own tree and the consumption is all that remains", ok, reason, tc.want(l, actual))
			}
		})
	}
}
