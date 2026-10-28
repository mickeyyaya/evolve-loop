//go:build integration

package ship

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"
)

func trackEvolveBinary(t *testing.T, repo string) {
	t.Helper()
	mustWrite(t, filepath.Join(repo, "go", "evolve"), "binary-v1\n")
	runGit(t, repo, "add", "go/evolve")
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "track evolve binary")
}

func TestShip_BinaryChurnNormalizedBeforeAudit(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	trackEvolveBinary(t, repo)
	addRemote(t, repo)
	wt := makeWorktree(t, repo, "cycle-1c-branch")
	mustWrite(t, filepath.Join(wt, "feature.txt"), "real cycle work\n")
	mustWrite(t, filepath.Join(wt, "go", "evolve"), "binary-v2-unaudited-churn\n")
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":1,"phase":"ship","active_worktree":"`+wt+`"}`)
	if err := discardBinaryChurn(context.Background(), &Options{ProjectRoot: repo, Runner: execRunner}, wt); err != nil {
		t.Fatal(err)
	}
	seedAudit(t, repo, "PASS")

	res, err := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "evolve-cycle 1: goal=test"})
	if err != nil || res.ExitCode != ExitOK {
		t.Fatalf("ship failed: exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}

	committed := runGitOut(t, repo, "show", "HEAD:go/evolve")
	if committed != "binary-v1\n" {
		t.Errorf("HEAD:go/evolve = %q — unaudited binary churn reached the cycle commit (want the audited \"binary-v1\\n\")", committed)
	}
	mainFiles := runGitOut(t, repo, "log", "-1", "--name-only", "--format=")
	if !strings.Contains(mainFiles, "feature.txt") {
		t.Errorf("feature.txt missing from the ship commit; files: %q", mainFiles)
	}
	if strings.Contains(mainFiles, "go/evolve") {
		t.Errorf("go/evolve appears in the cycle commit's file list — churn was committed: %q", mainFiles)
	}
}

func TestShip_SourceChangesPreserved(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	trackEvolveBinary(t, repo)
	mustWrite(t, filepath.Join(repo, "go", "internal", "widget.go"), "package widget // v1\n")
	runGit(t, repo, "add", "go/internal/widget.go")
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "track widget source")
	addRemote(t, repo)
	wt := makeWorktree(t, repo, "cycle-1d-branch")
	mustWrite(t, filepath.Join(wt, "go", "internal", "widget.go"), "package widget // v2 cycle work\n")
	mustWrite(t, filepath.Join(wt, "go", "evolve"), "binary-v2-unaudited-churn\n")
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":1,"phase":"ship","active_worktree":"`+wt+`"}`)
	if err := discardBinaryChurn(context.Background(), &Options{ProjectRoot: repo, Runner: execRunner}, wt); err != nil {
		t.Fatal(err)
	}
	seedAudit(t, repo, "PASS")

	res, err := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "evolve-cycle 1: goal=test"})
	if err != nil || res.ExitCode != ExitOK {
		t.Fatalf("ship failed: exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	if got := runGitOut(t, repo, "show", "HEAD:go/internal/widget.go"); got != "package widget // v2 cycle work\n" {
		t.Errorf("HEAD:go/internal/widget.go = %q — the discard swept real source work (must be scoped to go/evolve)", got)
	}
	if got := runGitOut(t, repo, "show", "HEAD:go/evolve"); got != "binary-v1\n" {
		t.Errorf("HEAD:go/evolve = %q, want audited \"binary-v1\\n\"", got)
	}
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestShip_PinPostCommitSha(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	binRel := filepath.Join("go", "evolve")
	binAbs := filepath.Join(repo, binRel)

	mustWrite(t, binAbs, "binary-v1\n")
	runGit(t, repo, "add", binRel)
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "binary v1")
	mustWrite(t, binAbs, "binary-v2-committed\n")
	runGit(t, repo, "add", binRel)
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "binary v2")
	mustWrite(t, binAbs, "binary-v3-stale-disk\n")

	opts := &Options{
		Class:          ClassCycle,
		ProjectRoot:    repo,
		PluginRoot:     repo,
		ShipBinaryPath: binAbs,
		Runner:         execRunner,
	}
	res := &RunResult{CommitSHA: headSHA(t, repo)}
	if err := repinPostCycle(opts, res); err != nil {
		t.Fatalf("repinPostCycle: %v", err)
	}

	stMap, err := readStateMap(filepath.Join(repo, ".evolve", "state.json"))
	if err != nil {
		t.Fatalf("read state.json: %v", err)
	}
	got := stateString(stMap, "expected_ship_sha")
	wantCommitted := sha256Hex([]byte("binary-v2-committed\n"))
	staleDisk := sha256Hex([]byte("binary-v3-stale-disk\n"))
	if got == staleDisk {
		t.Fatalf("expected_ship_sha pinned from the STALE ON-DISK blob (%s) — must pin the post-commit content at HEAD", got)
	}
	if got != wantCommitted {
		t.Errorf("expected_ship_sha = %q, want sha256 of HEAD:go/evolve content %q", got, wantCommitted)
	}
}
