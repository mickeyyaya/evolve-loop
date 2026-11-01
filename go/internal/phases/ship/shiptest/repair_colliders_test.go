//go:build integration

package shiptest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/shiptest"
)

func colliderFixture(t *testing.T, cycleBranch, path, wtContent, mainContent string) (repo, wt string) {
	t.Helper()
	repo = shiptest.MakeRepo(t)
	shiptest.AddRemote(t, repo)
	wt = shiptest.MakeWorktree(t, repo, cycleBranch)
	shiptest.MustWrite(t, filepath.Join(wt, "feature.txt"), "real cycle work\n")
	shiptest.MustWrite(t, filepath.Join(wt, path), wtContent)
	shiptest.MustWrite(t, filepath.Join(repo, path), mainContent)
	shiptest.MustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":1,"phase":"ship","active_worktree":"`+wt+`"}`)
	shiptest.SeedAudit(t, repo, "PASS")
	return repo, wt
}

func TestRepair_Colliders_IdenticalHealed_ShipProceeds(t *testing.T) {
	t.Parallel()
	repo, _ := colliderFixture(t, "cycle-rc-ident",
		"out/artifact.txt", "same bytes\n", "same bytes\n")

	res, err := runShip(t, repo, ship.Options{Class: ship.ClassCycle, CommitMessage: "feat: identical collider"})
	if res.ExitCode != ship.ExitOK {
		t.Fatalf("identical collider must self-heal; got exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	if res.RepairAttempted != string(core.CodeGitFFMergeDiverged) {
		t.Errorf("RepairAttempted = %q, want %s", res.RepairAttempted, core.CodeGitFFMergeDiverged)
	}
	if got := shiptest.RunGitOut(t, repo, "show", "HEAD:out/artifact.txt"); got != "same bytes\n" {
		t.Errorf("HEAD:out/artifact.txt = %q, want merged worktree content", got)
	}
	if _, statErr := os.Stat(filepath.Join(repo, ".evolve", "quarantine", "cycle-1", "out", "artifact.txt")); !os.IsNotExist(statErr) {
		t.Errorf("identical collider must not be quarantined (stat err=%v)", statErr)
	}
	if got, head := shiptest.RemoteHeadSHA(t, repo), shiptest.HeadSHA(t, repo); got != head {
		t.Errorf("remote main = %s, want pushed HEAD %s", got, head)
	}
}

func TestRepair_Colliders_DifferingQuarantined_ShipProceeds(t *testing.T) {
	t.Parallel()
	repo, _ := colliderFixture(t, "cycle-rc-diff",
		"notes/topology.md", "audited worktree copy\n", "stale main copy\n")

	res, err := runShip(t, repo, ship.Options{Class: ship.ClassCycle, CommitMessage: "feat: differing collider"})
	if res.ExitCode != ship.ExitOK {
		t.Fatalf("differing collider must quarantine + self-heal; got exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}

	qPath := filepath.Join(repo, ".evolve", "quarantine", "cycle-1", "notes", "topology.md")
	got, rerr := os.ReadFile(qPath)
	if rerr != nil {
		t.Fatalf("quarantined copy missing (content must NEVER be lost): %v", rerr)
	}
	if string(got) != "stale main copy\n" {
		t.Errorf("quarantined content = %q, want the original main-side bytes", got)
	}

	manifest, merr := os.ReadFile(filepath.Join(repo, ".evolve", "quarantine", "cycle-1", "manifest.json"))
	if merr != nil {
		t.Fatalf("quarantine manifest missing: %v", merr)
	}
	if !strings.Contains(string(manifest), "notes/topology.md") {
		t.Errorf("manifest must name the quarantined path; got %s", manifest)
	}

	if gotHead := shiptest.RunGitOut(t, repo, "show", "HEAD:notes/topology.md"); gotHead != "audited worktree copy\n" {
		t.Errorf("HEAD:notes/topology.md = %q, want audited worktree content", gotHead)
	}
	if gotRemote, head := shiptest.RemoteHeadSHA(t, repo), shiptest.HeadSHA(t, repo); gotRemote != head {
		t.Errorf("remote main = %s, want pushed HEAD %s", gotRemote, head)
	}
}
