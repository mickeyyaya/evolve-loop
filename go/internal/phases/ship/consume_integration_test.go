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
