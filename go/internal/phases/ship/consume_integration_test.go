//go:build integration

// consume_integration_test.go — transactional inbox consumption (the re-pick
// killer, consumption-rides-landing-ship 0.92; three live burns: cycle-1448,
// cycle-1464, cycle-1471). The defect: PASS promotion moves items to the
// GITIGNORED processed/ on the runtime plane AFTER the commit, so main keeps
// the tracked item and every fresh lane worktree re-picks it. The contract:
// the PASS ship commit ITSELF carries the consumption — tracked root deletion
// plus a tracked consumed/ record — so main stops offering the item the
// moment the work lands.
package ship

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func consumeScenario(t *testing.T, acsVerdict string) (repo, wt, ws, itemRel string) {
	t.Helper()
	repo, wt = makeWorktreeScenario(t)
	runGit(t, wt, "reset", "HEAD", "wt-change.txt")

	// Mirror the REAL repo's inbox tracking shape: .evolve/ ignored wholesale
	// by the base scenario, with the inbox API re-included and its runtime
	// subdirs ignored again (the exact negation ladder from the live
	// .gitignore — also the wall the layer-4 stager fix covers).
	mustWrite(t, filepath.Join(wt, ".gitignore"),
		".evolve/\n!.evolve/\n.evolve/*\n!.evolve/inbox/\n.evolve/inbox/processed/\n.evolve/inbox/processing/\n.evolve/inbox/rejected/\n")
	runGit(t, wt, "add", ".gitignore")
	itemRel = ".evolve/inbox/2026-08-15T03-00-00Z-fix-the-widget.json"
	mustWrite(t, filepath.Join(wt, filepath.FromSlash(itemRel)),
		`{"id":"fix-the-widget","title":"Fix the widget","weight":0.5}`)
	// Staged, not committed — the harness pattern the eval-drop test proved
	// (the ship commit carries the seed; production's pre-existing-item shape
	// differs only in WHERE the addition lives in history, and the contract
	// under test is the tree/consumed-record state after the ship).
	runGit(t, wt, "add", itemRel)

	ws = t.TempDir()
	mustWrite(t, filepath.Join(ws, "build-report.md"),
		"# Build Report\n\n## Files Changed\n\n- `wt-change.txt`\n")
	mustWrite(t, filepath.Join(ws, "triage-decision.json"),
		`{"schema_version":1,"top_n":[{"id":"fix-the-widget"}],"deferred":[],"dropped":[]}`)
	mustWrite(t, filepath.Join(ws, "acs-verdict.json"),
		`{"verdict":"`+acsVerdict+`","red_count":0}`)
	return repo, wt, ws, itemRel
}

func TestShipFromWorktree_ConsumesCommittedItemInTheShipCommit(t *testing.T) {
	repo, wt, ws, itemRel := consumeScenario(t, "PASS")
	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: widget fixed",
		ProjectRoot:   repo, PluginRoot: repo,
		WorkspacePath: ws, Stdout: io.Discard, Stderr: io.Discard,
	}
	res := &RunResult{}
	if err := shipFromWorktree(context.Background(), opts, res, "main", wt); err != nil {
		t.Fatalf("shipFromWorktree: %v", err)
	}
	files := commitFileList(t, wt, "cycle-1")
	if !strings.Contains(files, "consumed/2026-08-15T03-00-00Z-fix-the-widget.json") {
		t.Fatalf("the ship commit must carry the tracked consumed/ record; files=%q", files)
	}
	if _, err := os.Stat(filepath.Join(wt, filepath.FromSlash(itemRel))); !os.IsNotExist(err) {
		t.Error("root item must be gone from the tree — main stops offering it")
	}
	raw, err := os.ReadFile(filepath.Join(wt, ".evolve/inbox/consumed/2026-08-15T03-00-00Z-fix-the-widget.json"))
	if err != nil {
		t.Fatalf("consumed record must exist in the tree: %v", err)
	}
	var doc map[string]any
	if json.Unmarshal(raw, &doc) != nil || doc["consumed"] == nil {
		t.Errorf("consumed record must carry the consumption annotation: %s", raw)
	}
	if !strings.Contains(strings.Join(res.Logs, "\n"), "consumed") {
		t.Errorf("consumption must be LOUD in ship logs: %v", res.Logs)
	}
}

// WARN cycles ship under the fluent posture but the work may be partial —
// consumption stays gated on a PASS verdict; the item survives for re-pick.
func TestShipFromWorktree_WarnVerdictDoesNotConsume(t *testing.T) {
	repo, wt, ws, itemRel := consumeScenario(t, "WARN")
	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: widget attempt",
		ProjectRoot:   repo, PluginRoot: repo,
		WorkspacePath: ws, Stdout: io.Discard, Stderr: io.Discard,
	}
	if err := shipFromWorktree(context.Background(), opts, &RunResult{}, "main", wt); err != nil {
		t.Fatalf("shipFromWorktree: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wt, filepath.FromSlash(itemRel))); err != nil {
		t.Error("a WARN ship must NOT consume the item — the work may be partial")
	}
}

// An id the tree does not hold (already consumed, foreign, or never tracked)
// is a loud no-op — never an error that blocks the ship.
func TestShipFromWorktree_MissingItemIsANoOp(t *testing.T) {
	repo, wt, ws, _ := consumeScenario(t, "PASS")
	mustWrite(t, filepath.Join(ws, "triage-decision.json"),
		`{"schema_version":1,"top_n":[{"id":"fix-the-widget"},{"id":"never-existed"}],"deferred":[],"dropped":[]}`)
	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: widget fixed",
		ProjectRoot:   repo, PluginRoot: repo,
		WorkspacePath: ws, Stdout: io.Discard, Stderr: io.Discard,
	}
	if err := shipFromWorktree(context.Background(), opts, &RunResult{}, "main", wt); err != nil {
		t.Fatalf("a missing committed id must never block the ship: %v", err)
	}
}

// shipDirect wiring + the PRODUCTION staged-D shape (review survivors): the
// item lives in HEAD (committed), the direct cycle-class ship consumes it, and
// the resulting commit carries BOTH the root deletion and the consumed record.
func TestShipDirect_ConsumesCommittedItemFromHead(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	seedAudit(t, repo, "PASS")
	mustWrite(t, filepath.Join(repo, ".gitignore"),
		".evolve/\n!.evolve/\n.evolve/*\n!.evolve/inbox/\n.evolve/inbox/processed/\n.evolve/inbox/processing/\n.evolve/inbox/rejected/\n")
	itemRel := ".evolve/inbox/2026-08-15T04-00-00Z-direct-widget.json"
	mustWrite(t, filepath.Join(repo, filepath.FromSlash(itemRel)),
		`{"id":"direct-widget","title":"Direct widget","weight":0.4}`)
	runGit(t, repo, "add", ".gitignore", itemRel)
	runGit(t, repo, "commit", "-m", "seed tracked inbox item")
	mustWrite(t, filepath.Join(repo, "direct-change.txt"), "work\n")

	ws := t.TempDir()
	mustWrite(t, filepath.Join(ws, "build-report.md"),
		"# Build Report\n\n## Files Changed\n\n- `direct-change.txt`\n")
	mustWrite(t, filepath.Join(ws, "triage-decision.json"),
		`{"schema_version":1,"top_n":[{"id":"direct-widget"}],"deferred":[],"dropped":[]}`)
	mustWrite(t, filepath.Join(ws, "acs-verdict.json"), `{"verdict":"PASS","red_count":0}`)

	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: direct widget",
		ProjectRoot:   repo, PluginRoot: repo,
		WorkspacePath: ws, Stdout: io.Discard, Stderr: io.Discard,
	}
	res := &RunResult{}
	if err := shipDirect(context.Background(), opts, res, "main"); err != nil {
		t.Fatalf("shipDirect: %v", err)
	}
	files := commitFileList(t, repo, "main")
	if !strings.Contains(files, "consumed/2026-08-15T04-00-00Z-direct-widget.json") {
		t.Fatalf("direct ship commit must carry the consumed record; files=%q", files)
	}
	if !strings.Contains(files, "inbox/2026-08-15T04-00-00Z-direct-widget.json") {
		t.Fatalf("direct ship commit must carry the tracked ROOT DELETION (the staged-D production shape); files=%q", files)
	}
	if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(itemRel))); !os.IsNotExist(err) {
		t.Error("root item must be gone — main stops offering it")
	}
}

// consumeScenarioWith builds the consume harness and lets the caller replace the
// workspace's id-source files, so a test can express "no triage decision" or
// "triage named something else" without duplicating the fixture.
func consumeScenarioWith(t *testing.T, mutate func(ws string)) (repo, wt, ws, itemRel string) {
	t.Helper()
	repo, wt, ws, itemRel = consumeScenario(t, "PASS")
	mutate(ws)
	return repo, wt, ws, itemRel
}

// TestConsume_ResolvesIDsLikePostShip pins that IN-COMMIT consumption resolves the
// committed-id set from the same sources the POST-ship promotion already does.
//
// consume.go read triage top_n alone, while postship.go (:190-250) resolves from
// three and even documents the gap ("extractIDs only walks top_n/skip_shipped, so
// these orphans were never retired"). The asymmetry is why consumption never fired
// in 8 cycles: a carryover-driven lane carries no triage id matching its inbox
// file, so the item stayed pickable even on a PASS ship that closed it.
func TestConsume_ResolvesIDsLikePostShip(t *testing.T) {
	consumedRel := ".evolve/inbox/consumed/2026-08-15T03-00-00Z-fix-the-widget.json"

	t.Run("no triage decision: the lane-scope pin is the committed set", func(t *testing.T) {
		repo, wt, ws, itemRel := consumeScenarioWith(t, func(ws string) {
			// A continuation/lane cycle carries NO triage decision at all.
			mustRemove(t, filepath.Join(ws, "triage-decision.json"))
			mustWrite(t, filepath.Join(ws, "lane-scope.json"),
				`{"todo_ids":["fix-the-widget"],"goal_hash":"abc"}`)
		})
		shipConsumeAndAssert(t, repo, wt, ws, itemRel, consumedRel,
			"a lane cycle with no triage decision must consume its lane-scope item in-commit")
	})

	t.Run("triage decided nothing: a lane-scope pin must NOT retire the declined menu", func(t *testing.T) {
		// Precedence guard at the CONSUME site. postship has its own pin
		// (TestPromoteInbox_EmptyCommittedDeclinedMenuStaysOpen) but the blast
		// radius here is worse: a wrong consume lands a tracked deletion on main,
		// where postship would only have made a recoverable processed/ move.
		repo, wt, ws, itemRel := consumeScenarioWith(t, func(ws string) {
			mustWrite(t, filepath.Join(ws, "triage-decision.json"),
				`{"schema_version":1,"top_n":[],"deferred":[],"dropped":[]}`)
			mustWrite(t, filepath.Join(ws, "lane-scope.json"),
				`{"todo_ids":["fix-the-widget"],"goal_hash":"abc"}`)
		})
		opts := &Options{
			Class: ClassCycle, CommitMessage: "feat: nothing committed",
			ProjectRoot: repo, PluginRoot: repo,
			WorkspacePath: ws, Stdout: io.Discard, Stderr: io.Discard,
		}
		if err := shipFromWorktree(context.Background(), opts, &RunResult{}, "main", wt); err != nil {
			t.Fatalf("shipFromWorktree: %v", err)
		}
		if _, err := os.Stat(filepath.Join(wt, filepath.FromSlash(itemRel))); err != nil {
			t.Error("a PRESENT triage decision that committed zero ids must keep the declined menu open — lane-scope must not override it")
		}
	})

	t.Run("triage dropped the assigned id as already-shipped: the PASS ship still consumes it (cycle-1552)", func(t *testing.T) {
		// soak-20260824a wave-2 burn: 1552's triage put the fleet-scope id in
		// dropped[] with top_n:[], build shipped the item's implementation
		// anyway (df322f6c), consumption resolved zero ids, and the stale item
		// cost the next wave a full lane re-proving finished work. A dropped
		// ASSIGNED id is an affirmative close and must retire in-commit.
		repo, wt, ws, itemRel := consumeScenarioWith(t, func(ws string) {
			mustWrite(t, filepath.Join(ws, "triage-decision.json"),
				`{"schema_version":1,"top_n":[],"deferred":[],"dropped":[{"id":"fix-the-widget","reason":"already-shipped"}]}`)
			mustWrite(t, filepath.Join(ws, "lane-scope.json"),
				`{"todo_ids":["fix-the-widget"],"goal_hash":"abc"}`)
		})
		shipConsumeAndAssert(t, repo, wt, ws, itemRel,
			".evolve/inbox/consumed/2026-08-15T03-00-00Z-fix-the-widget.json",
			"a triage-dropped assigned scope id must be consumed by the PASS landing (cycle-1552)")
	})

	t.Run("triage named a different id: the Closes-Inbox marker still closes it", func(t *testing.T) {
		repo, wt, ws, itemRel := consumeScenarioWith(t, func(ws string) {
			// The decomposition shape: triage renames the work, so top_n never
			// matches the inbox file's own id.
			mustWrite(t, filepath.Join(ws, "triage-decision.json"),
				`{"schema_version":1,"top_n":[{"id":"some-decomposed-subtask"}],"deferred":[],"dropped":[]}`)
			mustWrite(t, filepath.Join(ws, "build-report.md"),
				"# Build Report\n\n## Files Changed\n\n- `wt-change.txt`\n\nCloses-Inbox: fix-the-widget\n")
		})
		shipConsumeAndAssert(t, repo, wt, ws, itemRel, consumedRel,
			"a builder that declares Closes-Inbox must have the item consumed in-commit even when triage renamed the work")
	})
}

func mustRemove(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove %s: %v", path, err)
	}
}

// shipConsumeAndAssert runs a cycle ship over wt and asserts the item was
// consumed INTO the commit: root deletion + tracked consumed/ record + a loud log.
func shipConsumeAndAssert(t *testing.T, repo, wt, ws, itemRel, consumedRel, why string) {
	t.Helper()
	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: widget fixed",
		ProjectRoot:   repo, PluginRoot: repo,
		WorkspacePath: ws, Stdout: io.Discard, Stderr: io.Discard,
	}
	res := &RunResult{}
	if err := shipFromWorktree(context.Background(), opts, res, "main", wt); err != nil {
		t.Fatalf("shipFromWorktree: %v", err)
	}
	// Full path, not the basename: the commit also carries the tracked ROOT
	// DELETION under the identical basename, so a basename match is satisfied by
	// an implementation that deletes the item and never writes the record.
	if files := commitFileList(t, wt, "cycle-1"); !strings.Contains(files, "consumed/"+filepath.Base(consumedRel)) {
		t.Fatalf("%s\nthe ship commit must carry the tracked consumed/ record; files=%q", why, files)
	}
	if _, err := os.Stat(filepath.Join(wt, filepath.FromSlash(itemRel))); !os.IsNotExist(err) {
		t.Errorf("%s\nroot item must be gone from the tree", why)
	}
	if !strings.Contains(strings.Join(res.Logs, "\n"), "consumed") {
		t.Errorf("%s\nconsumption must be LOUD in ship logs: %v", why, res.Logs)
	}
}
