//go:build integration

package ship

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestManualShip_ClosesInboxItemInShipCommit reproduces the stale-inbox path:
// a valid manual ship lands the declared fix but leaves its tracked inbox item
// available for every later cycle to rediscover.
func TestManualShip_ClosesInboxItemInShipCommit(t *testing.T) {
	repo := makeRepo(t)
	excludeCommitGate(t, repo)
	addRemote(t, repo)

	mustWrite(t, filepath.Join(repo, ".gitignore"),
		".evolve/\n!.evolve/\n.evolve/*\n!.evolve/inbox/\n.evolve/inbox/processed/\n.evolve/inbox/processing/\n.evolve/inbox/rejected/\n.commit-gate/\n")
	itemRel := ".evolve/inbox/2026-08-23T00-00-00Z-manual-widget.json"
	mustWrite(t, filepath.Join(repo, filepath.FromSlash(itemRel)),
		`{"id":"manual-widget","title":"Fix manual widget","weight":0.5}`)
	runGit(t, repo, "add", ".gitignore", itemRel)
	runGit(t, repo, "commit", "-m", "seed tracked inbox item")

	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixed by a reviewed console ship\n")
	workspace := t.TempDir()
	mustWrite(t, filepath.Join(workspace, "triage-decision.json"),
		`{"schema_version":1,"top_n":[{"id":"manual-widget"}],"deferred":[],"dropped":[]}`)
	mustWrite(t, filepath.Join(workspace, "acs-verdict.json"), `{"verdict":"PASS","red_count":0}`)

	// The manual path must satisfy its normal review gate; this test then proves
	// inbox retirement is part of the production ship, not a direct helper call.
	writeAttestation(t, repo, treeStateSHA(t, repo))
	res, err := runShip(t, repo, Options{
		Class: ClassManual, CommitMessage: "fix: manual widget",
		WorkspacePath: workspace, Stdout: io.Discard, Stderr: io.Discard,
		Env: map[string]string{"EVOLVE_SHIP_AUTO_CONFIRM": "1"},
	})
	if err != nil || res.ExitCode != ExitOK {
		t.Fatalf("manual ship must land: exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}

	files := commitFileList(t, repo, "main")
	if !strings.Contains(files, "consumed/2026-08-23T00-00-00Z-manual-widget.json") {
		t.Fatalf("manual ship must retire its closed inbox item in the landing commit; files=%q", files)
	}
	if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(itemRel))); !os.IsNotExist(err) {
		t.Fatalf("manual ship left the tracked inbox item pickable: stat err=%v", err)
	}
}
