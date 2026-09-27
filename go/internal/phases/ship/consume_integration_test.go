//go:build integration

package ship

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/acsrunner"
)

func consumeScenario(t *testing.T, acsVerdict string) (repo, wt, ws, itemRel string) {
	t.Helper()
	repo, wt = makeWorktreeScenario(t)
	runGit(t, wt, "reset", "HEAD", "wt-change.txt")

	// Mirrors the real repo's .gitignore negation ladder for .evolve/inbox.
	mustWrite(t, filepath.Join(wt, ".gitignore"),
		".evolve/\n!.evolve/\n.evolve/*\n!.evolve/inbox/\n.evolve/inbox/processed/\n.evolve/inbox/processing/\n.evolve/inbox/rejected/\n")
	runGit(t, wt, "add", ".gitignore")
	itemRel = ".evolve/inbox/2026-08-15T03-00-00Z-fix-the-widget.json"
	mustWrite(t, filepath.Join(wt, filepath.FromSlash(itemRel)),
		`{"id":"fix-the-widget","title":"Fix the widget","weight":0.5}`)
	// Staged, not committed: the ship commit itself carries the seed; only
	// the tree/consumed-record state after ship is under test.
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

func TestShipFromWorktree_WarnWithRedsDoesNotConsume(t *testing.T) {
	repo, wt, ws, itemRel := consumeScenarioWith(t, func(ws string) {
		mustWrite(t, filepath.Join(ws, "acs-verdict.json"), `{"verdict":"WARN","red_count":2}`)
	})
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
		t.Error("a WARN ship with red_count>0 must NOT consume the item — the work is genuinely partial")
	}
}

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

func consumeScenarioWith(t *testing.T, mutate func(ws string)) (repo, wt, ws, itemRel string) {
	t.Helper()
	repo, wt, ws, itemRel = consumeScenario(t, "PASS")
	mutate(ws)
	return repo, wt, ws, itemRel
}

func TestConsume_ResolvesIDsLikePostShip(t *testing.T) {
	consumedRel := ".evolve/inbox/consumed/2026-08-15T03-00-00Z-fix-the-widget.json"

	t.Run("no triage decision: the lane-scope pin is the committed set", func(t *testing.T) {
		repo, wt, ws, itemRel := consumeScenarioWith(t, func(ws string) {
			mustRemove(t, filepath.Join(ws, "triage-decision.json"))
			mustWrite(t, filepath.Join(ws, "lane-scope.json"),
				`{"todo_ids":["fix-the-widget"],"goal_hash":"abc"}`)
		})
		shipConsumeAndAssert(t, repo, wt, ws, itemRel, consumedRel,
			"a lane cycle with no triage decision must consume its lane-scope item in-commit")
	})

	t.Run("triage decided nothing: a lane-scope pin must NOT retire the declined menu", func(t *testing.T) {
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
			// Triage renamed the work, so top_n never matches the inbox file's own id.
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

func TestManualShip_NonShippableVerdictKeepsItemPickable(t *testing.T) {
	t.Parallel() // self-contained temp repo; see TestConsumeGate_OnlyVerdictPASSConsumes
	repo := makeRepo(t)
	excludeCommitGate(t, repo)
	addRemote(t, repo)

	mustWrite(t, filepath.Join(repo, ".gitignore"),
		".evolve/\n!.evolve/\n.evolve/*\n!.evolve/inbox/\n.evolve/inbox/processed/\n.evolve/inbox/processing/\n.evolve/inbox/rejected/\n.commit-gate/\n")
	itemRel := ".evolve/inbox/2026-09-26T00-00-00Z-unfinished-widget.json"
	mustWrite(t, filepath.Join(repo, filepath.FromSlash(itemRel)),
		`{"id":"unfinished-widget","title":"Fix the unfinished widget","weight":0.5}`)
	runGit(t, repo, "add", ".gitignore", itemRel)
	runGit(t, repo, "commit", "-m", "seed tracked inbox item")
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "landed by a reviewed console ship\n")

	// A killed/timed-out predicate run: the test started and never reported.
	stream := strings.NewReader(`{"Action":"run","Test":"TestC1691_001_Widget"}` + "\n")
	v, err := acsrunner.ParseTestJSON(stream, 1691)
	if err != nil {
		t.Fatalf("acsrunner.ParseTestJSON: %v", err)
	}
	verdictPath, err := acsrunner.WriteVerdict(filepath.Join(t.TempDir(), ".evolve"), v)
	if err != nil {
		t.Fatalf("acsrunner.WriteVerdict: %v", err)
	}
	workspace := filepath.Dir(verdictPath)
	requireNonShippableZeroRed(t, verdictPath)
	mustWrite(t, filepath.Join(workspace, "triage-decision.json"),
		`{"schema_version":1,"top_n":[{"id":"unfinished-widget"}],"deferred":[],"dropped":[]}`)

	writeAttestation(t, repo, treeStateSHA(t, repo))
	res, err := runShip(t, repo, Options{
		Class: ClassManual, CommitMessage: "fix: unfinished widget",
		WorkspacePath: workspace, Stdout: io.Discard, Stderr: io.Discard,
		Env: map[string]string{"EVOLVE_SHIP_AUTO_CONFIRM": "1"},
	})
	if err != nil || res.ExitCode != ExitOK {
		t.Fatalf("manual ship must still land (the code was reviewed): exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}

	files := commitFileList(t, repo, "main")
	if !strings.Contains(files, "fixture.txt") {
		t.Fatalf("the landing commit must carry the shipped change; files=%q", files)
	}
	if strings.Contains(files, "inbox/consumed/") {
		t.Errorf("an unfinished suite (verdict FAIL, ship_eligible false, red_count 0) must not retire its inbox item; files=%q", files)
	}
	if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(itemRel))); err != nil {
		t.Errorf("the inbox item must stay pickable in the tree: %v", err)
	}
	if strings.TrimSpace(runGitOut(t, repo, "ls-files", "--", itemRel)) != itemRel {
		t.Errorf("the inbox item must stay tracked after the landing commit")
	}
	if !strings.Contains(strings.Join(res.Logs, "\n"), "inbox consumption skipped") {
		t.Errorf("a refused consumption must be LOUD in ship logs: %v", res.Logs)
	}
}

// Guards the fixture: if the producer ever stops writing verdict FAIL,
// ship_eligible false and red_count 0 together, this must fail loudly
// instead of passing vacuously.
func requireNonShippableZeroRed(t *testing.T, path string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read produced verdict: %v", err)
	}
	var doc struct {
		Verdict         string `json:"verdict"`
		ShipEligible    *bool  `json:"ship_eligible"`
		RedCount        *int   `json:"red_count"`
		IncompleteCount int    `json:"incomplete_count"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse produced verdict: %v", err)
	}
	if doc.Verdict != "FAIL" || doc.ShipEligible == nil || *doc.ShipEligible ||
		doc.RedCount == nil || *doc.RedCount != 0 || doc.IncompleteCount == 0 {
		t.Fatalf("fixture precondition: acsrunner must write verdict FAIL, ship_eligible false, red_count 0, incomplete_count>0; got %s", raw)
	}
}

func TestConsumeGate_OnlyVerdictPASSConsumes(t *testing.T) {
	t.Parallel()
	acssuitePass, err := json.Marshal(predicateVerdictFixture(1691, 3, 0, 1))
	if err != nil {
		t.Fatal(err)
	}
	acssuiteFail, err := json.Marshal(predicateVerdictFixture(1691, 2, 1, 0))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name        string
		body        string // "" = no acs-verdict.json at all
		wantConsume bool
	}{
		{"acssuite PASS", string(acssuitePass), true},
		{"PASS red_count 0", `{"verdict":"PASS","red_count":0,"ship_eligible":true}`, true},
		{"PASS without red_count", `{"verdict":"PASS"}`, true},
		{"acsrunner unfinished: FAIL red_count 0", `{"verdict":"FAIL","red_count":0,"incomplete_count":2,"ship_eligible":false}`, false},
		{"FAIL red_count 0 ship_eligible false", `{"verdict":"FAIL","red_count":0,"ship_eligible":false}`, false},
		{"acssuite FAIL", string(acssuiteFail), false},
		{"WARN red_count 0", `{"verdict":"WARN","red_count":0}`, false},
		{"WARN red_count 2", `{"verdict":"WARN","red_count":2}`, false},
		{"missing verdict file", "", false},
		{"unparseable verdict file", `{broken`, false},
	}
	for _, class := range []Class{ClassCycle, ClassManual} {
		for _, tc := range cases {
			t.Run(string(class)+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				repo, wt, ws, itemRel := consumeScenarioWith(t, func(ws string) {
					path := filepath.Join(ws, "acs-verdict.json")
					if tc.body == "" {
						mustRemove(t, path)
						return
					}
					mustWrite(t, path, tc.body)
				})
				opts := &Options{
					Class:         class,
					CommitMessage: "feat: widget attempt",
					ProjectRoot:   repo, PluginRoot: repo,
					WorkspacePath: ws, Stdout: io.Discard, Stderr: io.Discard,
				}
				res := &RunResult{}
				if err := shipFromWorktree(context.Background(), opts, res, "main", wt); err != nil {
					t.Fatalf("shipFromWorktree: %v", err)
				}
				_, statErr := os.Stat(filepath.Join(wt, filepath.FromSlash(itemRel)))
				consumed := os.IsNotExist(statErr)
				if consumed != tc.wantConsume {
					t.Errorf("verdict %q: consumed=%v, want %v; logs=%v", tc.body, consumed, tc.wantConsume, res.Logs)
				}
				if landedPASS := workspaceACSVerdict(ws) == "PASS"; consumed != landedPASS {
					t.Errorf("verdict %q: in-commit consumption (%v) drifted from postship's landedPASS gate (%v)", tc.body, consumed, landedPASS)
				}
			})
		}
	}
}
