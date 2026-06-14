//go:build integration

// closure_idempotency_test.go — cycle-234 task `ship-closure-idempotency` (RED).
//
// Three defects from the cycle-233 landing saga (inbox
// 2026-06-06T03-27-08Z-ship-closure.json) made a fully-successful push
// batch-fatal:
//
//	D1: a correction re-dispatch AFTER the push re-ran the FULL ship →
//	    AUDIT_BINDING_HEAD_MOVED dead-end (HEAD is the ship's own commit).
//	D2: `git -C <worktree> add -A` swept unaudited go/evolve binary churn
//	    into the cycle commit (build's recoverBuildLeak pattern not applied
//	    in ship) → audit AC5 silent-pass broken.
//	D3: expected_ship_sha pinned from a PRE-commit blob, not the binary as
//	    committed at HEAD → SELF_SHA_TAMPERED next cycle, hand-corrected
//	    twice via state.json delete.
//
// All tests use the real-git fixtures from native_test.go / worktree_test.go
// (makeRepo, addRemote, makeWorktree, seedAudit, runShip). They are
// BEHAVIORAL: they assert on git history, remote refs and state.json — not
// on log strings alone.
package ship

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"
)

// shipWorktreeFixture provisions the standard worktree-cycle ship scenario:
// repo + bare remote + active worktree on cycleBranch with feature.txt
// edited, cycle-state.json pointing at the worktree, PASS audit seeded.
func shipWorktreeFixture(t *testing.T, cycleBranch string) (repo, wt string) {
	t.Helper()
	repo = makeRepo(t)
	addRemote(t, repo)
	wt = makeWorktree(t, repo, cycleBranch)
	mustWrite(t, filepath.Join(wt, "feature.txt"), "real cycle work\n")
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":1,"phase":"ship","active_worktree":"`+wt+`"}`)
	seedAudit(t, repo, "PASS")
	return repo, wt
}

func headSHA(t *testing.T, repo string) string {
	t.Helper()
	return strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))
}

func remoteHeadSHA(t *testing.T, repo string) string {
	t.Helper()
	out := runGitOut(t, repo, "ls-remote", "origin", "refs/heads/main")
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// --- D1: post-push correction re-dispatch must be report-only ------------

// TestShip_PostPush_Idempotent_CorrectReportOnly: after a successful
// commit+push, a re-dispatch of ship for the SAME cycle (the orchestrator's
// deliverable-contract correction path) must recognize that HEAD already IS
// its own ship commit (ship-binding.json) and succeed report-only: no new
// commit, no push, no AUDIT_BINDING_HEAD_MOVED dead-end.
func TestShip_PostPush_Idempotent_CorrectReportOnly(t *testing.T) {
	repo, _ := shipWorktreeFixture(t, "cycle-1-branch")

	// First ship: the real push. Precondition, not the behavior under test.
	res1, err := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "evolve-cycle 1: goal=test"})
	if err != nil || res1.ExitCode != ExitOK {
		t.Fatalf("fixture ship failed: exit=%d err=%v logs=%v", res1.ExitCode, err, res1.Logs)
	}
	shippedHEAD := headSHA(t, repo)
	shippedRemote := remoteHeadSHA(t, repo)

	// Correction re-dispatch: identical invocation, post-push state.
	res2, err2 := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "evolve-cycle 1: goal=test"})

	if res2.ExitCode != ExitOK {
		t.Fatalf("post-push re-dispatch exit=%d (err=%v) — must be ExitOK report-only, not the cycle-233 AUDIT_BINDING_HEAD_MOVED dead-end; logs=%v",
			res2.ExitCode, err2, res2.Logs)
	}
	if err2 != nil {
		t.Fatalf("post-push re-dispatch returned error: %v", err2)
	}
	// Report-only: the re-dispatch must still report the existing ship
	// commit so the orchestrator's records stay coherent.
	if res2.CommitSHA != shippedHEAD {
		t.Errorf("re-dispatch CommitSHA=%q, want the existing ship commit %q", res2.CommitSHA, shippedHEAD)
	}
	// No new commit and no second push.
	if got := headSHA(t, repo); got != shippedHEAD {
		t.Errorf("HEAD moved on re-dispatch (%s → %s) — post-push correction must not commit", shippedHEAD, got)
	}
	if got := remoteHeadSHA(t, repo); got != shippedRemote {
		t.Errorf("remote main moved on re-dispatch (%s → %s) — post-push correction must not push", shippedRemote, got)
	}
}

// TestShip_PrePush_CorrectionStillFullShip (adversarial guard): the
// idempotency check must key on "HEAD == the binding's OWN ship commit",
// not on the mere presence of ship-binding.json — a STALE binding from an
// earlier attempt must not short-circuit a real, not-yet-pushed cycle.
func TestShip_PrePush_CorrectionStillFullShip(t *testing.T) {
	repo, _ := shipWorktreeFixture(t, "cycle-1b-branch")

	// Stale sidecar: a binding whose commit_sha is NOT the current HEAD.
	mustWrite(t, filepath.Join(repo, ".evolve", "runs", "cycle-1", "ship-binding.json"),
		`{"audit_bound_tree_sha":"","tree_sha_committed":"","commit_sha":"`+strings.Repeat("d", 40)+`","cycle":1}`)

	preHEAD := headSHA(t, repo)
	res, err := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "evolve-cycle 1: goal=test"})
	if err != nil || res.ExitCode != ExitOK {
		t.Fatalf("pre-push ship must run in full: exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	postHEAD := headSHA(t, repo)
	if postHEAD == preHEAD {
		t.Fatal("HEAD did not advance — the stale binding short-circuited a REAL ship (idempotency fired too eagerly)")
	}
	// The real work must be on main.
	mainFiles := runGitOut(t, repo, "log", "-1", "--name-only", "--format=")
	if !strings.Contains(mainFiles, "feature.txt") {
		t.Errorf("feature.txt not in the ship commit; files: %q", mainFiles)
	}
	if got := remoteHeadSHA(t, repo); got != postHEAD {
		t.Errorf("remote main=%s, want pushed HEAD %s", got, postHEAD)
	}
}

// --- D2: unaudited binary churn must not reach the cycle commit ----------

// trackEvolveBinary commits go/evolve (content v1) into the fixture repo so
// it is a TRACKED file, mirroring the real repo's release-managed binary.
func trackEvolveBinary(t *testing.T, repo string) {
	t.Helper()
	mustWrite(t, filepath.Join(repo, "go", "evolve"), "binary-v1\n")
	runGit(t, repo, "add", "go/evolve")
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "track evolve binary")
}

// TestShip_BinaryChurnDiscarded: a `go build`-regenerated go/evolve in the
// WORKTREE is unaudited churn (the audit bound main's tree). Ship must
// discard it before staging — the parallel of the orchestrator's
// recoverBuildLeak for the build phase — so the merged cycle commit carries
// the audited tree, not binary drift (audit AC5 soundness).
func TestShip_BinaryChurnDiscarded(t *testing.T) {
	repo := makeRepo(t)
	trackEvolveBinary(t, repo)
	addRemote(t, repo)
	wt := makeWorktree(t, repo, "cycle-1c-branch")
	// Real cycle work + unaudited binary churn, both in the worktree.
	mustWrite(t, filepath.Join(wt, "feature.txt"), "real cycle work\n")
	mustWrite(t, filepath.Join(wt, "go", "evolve"), "binary-v2-unaudited-churn\n")
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":1,"phase":"ship","active_worktree":"`+wt+`"}`)
	seedAudit(t, repo, "PASS")

	res, err := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "evolve-cycle 1: goal=test"})
	if err != nil || res.ExitCode != ExitOK {
		t.Fatalf("ship failed: exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}

	// The shipped tree must carry the AUDITED binary (v1), not the churn.
	committed := runGitOut(t, repo, "show", "HEAD:go/evolve")
	if committed != "binary-v1\n" {
		t.Errorf("HEAD:go/evolve = %q — unaudited binary churn reached the cycle commit (want the audited \"binary-v1\\n\")", committed)
	}
	// The real work still shipped.
	mainFiles := runGitOut(t, repo, "log", "-1", "--name-only", "--format=")
	if !strings.Contains(mainFiles, "feature.txt") {
		t.Errorf("feature.txt missing from the ship commit; files: %q", mainFiles)
	}
	if strings.Contains(mainFiles, "go/evolve") {
		t.Errorf("go/evolve appears in the cycle commit's file list — churn was committed: %q", mainFiles)
	}
}

// TestShip_SourceChangesPreserved (adversarial guard): the churn discard
// must be PATH-SCOPED to the release-managed binary. Real Go source edits
// under go/ are the cycle's actual work and must ship untouched.
func TestShip_SourceChangesPreserved(t *testing.T) {
	repo := makeRepo(t)
	trackEvolveBinary(t, repo)
	mustWrite(t, filepath.Join(repo, "go", "internal", "widget.go"), "package widget // v1\n")
	runGit(t, repo, "add", "go/internal/widget.go")
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "track widget source")
	addRemote(t, repo)
	wt := makeWorktree(t, repo, "cycle-1d-branch")
	// The cycle's real work: a SOURCE change next to binary churn.
	mustWrite(t, filepath.Join(wt, "go", "internal", "widget.go"), "package widget // v2 cycle work\n")
	mustWrite(t, filepath.Join(wt, "go", "evolve"), "binary-v2-unaudited-churn\n")
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":1,"phase":"ship","active_worktree":"`+wt+`"}`)
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

// --- D3: expected_ship_sha pinned from the POST-commit tree --------------

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// TestShip_PinPostCommitSha: repinPostCycle must record the SHA of the ship
// binary AS COMMITTED AT HEAD — not whatever blob happens to sit on disk at
// repin time (the pre-commit/stale-build value that twice forced a manual
// state.json delete this campaign).
//
// Divergence fixture: HEAD commits the binary at v2 while the on-disk file
// has drifted to v3. The pin must be sha256(v2-committed), never sha256(v3).
func TestShip_PinPostCommitSha(t *testing.T) {
	repo := makeRepo(t)
	binRel := filepath.Join("go", "evolve")
	binAbs := filepath.Join(repo, binRel)

	mustWrite(t, binAbs, "binary-v1\n")
	runGit(t, repo, "add", binRel)
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "binary v1")
	mustWrite(t, binAbs, "binary-v2-committed\n")
	runGit(t, repo, "add", binRel)
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "binary v2")
	// Disk drifts AFTER the commit (a rebuild) — committed state stays v2.
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
