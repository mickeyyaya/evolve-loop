//go:build integration

package ship

// consume_unified_integration_test.go — the real-git reachability half of the
// unified all-or-nothing consume: driven through shipFromWorktree (the cycle
// ship path), the LANDING COMMIT carries every member's consumed/ record or
// none of them. A rollback that restores the files but leaves the index staged
// still ships a split closeout, which only a real commit can show.

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// unifiedShipScenario is consumeScenario with a validated two-member unified
// commitment: both member items are staged in the cycle worktree, and every id
// in corrupt is present but unparseable.
func unifiedShipScenario(t *testing.T, corrupt ...string) (repo, wt, ws string, rel, orig map[string]string) {
	t.Helper()
	repo, wt = makeWorktreeScenario(t)
	runGit(t, wt, "reset", "HEAD", "wt-change.txt")
	mustWrite(t, filepath.Join(wt, ".gitignore"),
		".evolve/\n!.evolve/\n.evolve/*\n!.evolve/inbox/\n.evolve/inbox/processed/\n.evolve/inbox/processing/\n.evolve/inbox/rejected/\n")
	runGit(t, wt, "add", ".gitignore")

	members := []string{unifiedMemberA, unifiedMemberB}
	bad := map[string]bool{}
	for _, id := range corrupt {
		bad[id] = true
	}
	rel, orig = map[string]string{}, map[string]string{}
	for i, id := range members {
		rel[id] = fmt.Sprintf(".evolve/inbox/2026-09-27T00-00-%02dZ-%s.json", i, id)
		orig[id] = fmt.Sprintf(`{"id":%q,"title":"fixture %s","weight":0.5}`, id, id)
		if bad[id] {
			orig[id] = corruptItemBytes
		}
		mustWrite(t, filepath.Join(wt, filepath.FromSlash(rel[id])), orig[id])
		runGit(t, wt, "add", rel[id])
	}

	ws = t.TempDir()
	mustWrite(t, filepath.Join(ws, "build-report.md"),
		"# Build Report\n\n## Files Changed\n\n- `wt-change.txt`\n")
	mustWrite(t, filepath.Join(ws, "triage-decision.json"), unifiedDecision(t, members, members, true))
	mustWrite(t, filepath.Join(ws, "acs-verdict.json"), `{"verdict":"PASS","red_count":0}`)
	return repo, wt, ws, rel, orig
}

func shipUnifiedScenario(t *testing.T, repo, wt, ws string) *RunResult {
	t.Helper()
	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: unified commitment members",
		ProjectRoot:   repo, PluginRoot: repo,
		WorkspacePath: ws, Stdout: io.Discard, Stderr: io.Discard,
	}
	res := &RunResult{}
	if err := shipFromWorktree(context.Background(), opts, res, "main", wt); err != nil {
		t.Fatalf("a consumption problem must never block the ship: %v", err)
	}
	return res
}

func TestShipFromWorktree_UnifiedCommitmentClosesAllOrNothing(t *testing.T) {
	t.Run("a member that cannot close keeps every member out of the landing commit", func(t *testing.T) {
		repo, wt, ws, rel, orig := unifiedShipScenario(t, unifiedMemberB)
		res := shipUnifiedScenario(t, repo, wt, ws)
		files := commitFileList(t, wt, "cycle-1")
		for _, id := range []string{unifiedMemberA, unifiedMemberB} {
			if strings.Contains(files, "consumed/"+filepath.Base(rel[id])) {
				t.Errorf("the landing commit closed %s while its sibling could not close — a split unified closeout; files=%q\nlogs:\n%s",
					id, files, strings.Join(res.Logs, "\n"))
			}
			raw, err := os.ReadFile(filepath.Join(wt, filepath.FromSlash(rel[id])))
			if err != nil || string(raw) != orig[id] {
				t.Errorf("%s must stay pickable in the shipped tree (err=%v got=%q)", id, err, raw)
			}
		}
		if !unifiedWarn(res, unifiedMemberB) {
			t.Errorf("expected a WARN naming unified_commitment and %q; logs:\n%s", unifiedMemberB, strings.Join(res.Logs, "\n"))
		}
	})

	t.Run("members that all close ride the landing commit together", func(t *testing.T) {
		repo, wt, ws, rel, _ := unifiedShipScenario(t)
		res := shipUnifiedScenario(t, repo, wt, ws)
		files := commitFileList(t, wt, "cycle-1")
		for _, id := range []string{unifiedMemberA, unifiedMemberB} {
			if !strings.Contains(files, "consumed/"+filepath.Base(rel[id])) {
				t.Errorf("the landing commit must carry %s's consumed/ record; files=%q\nlogs:\n%s",
					id, files, strings.Join(res.Logs, "\n"))
			}
			if _, err := os.Stat(filepath.Join(wt, filepath.FromSlash(rel[id]))); !os.IsNotExist(err) {
				t.Errorf("%s must leave the inbox root of the shipped tree (stat err=%v)", id, err)
			}
		}
	})
}
