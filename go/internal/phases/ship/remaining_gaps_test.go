//go:build integration

// remaining_gaps_test.go — targeted coverage for the achievable gaps
// remaining after 91.5%:
//
//   - atomicShip: currentBranch runner error (gitops.go:33)
//   - shipDirect: buildDiffFooter --name-status error (gitops.go:94)
//   - shipDirect: runCommitPrefixGate error (gitops.go:103)
//   - advanceLastCycleNumber: state.json readStateMap error (postship.go:63)
//   - promoteInbox: readStateMap error (postship.go:83)
//   - verifyManualConfirm: git add -A runner error (verify.go:161)
//   - verifyTrivial: diff --name-only runner error (verify.go:267)
//   - verifyTrivial: ls-files runner error (verify.go:271)
//   - native.go:213 postShip WARN log (postShip returns error, ship continues)
package ship

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- atomicShip: currentBranch runner error (gitops.go:33) -----------------

func TestAtomicShip_CurrentBranchRunnerError_Errors(t *testing.T) {
	repo := makeRepo(t)

	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: branch fail",
		ProjectRoot:   repo,
		Runner:        faultRunner("git symbolic-ref", -1, errors.New("symbolic-ref exploded")),
		Stdout:        io.Discard,
		Stderr:        io.Discard,
	}
	err := atomicShip(context.Background(), opts, &RunResult{})
	if err == nil {
		t.Fatal("want currentBranch error, got nil")
	}
}

// --- shipDirect: buildDiffFooter --name-status error (gitops.go:94) --------

func TestShipDirect_BuildDiffFooterNameStatusError_Errors(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "change.txt"), "staged change\n")

	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: footer fail",
		ProjectRoot:   repo,
		Runner: func(ctx context.Context, name, cwd string, args, env []string,
			stdin io.Reader, stdout, stderr io.Writer) (int, error) {
			if name == "git" && argsContain(args, "--name-status") {
				return -1, errors.New("name-status exploded")
			}
			return execRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
		},
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	err := shipDirect(context.Background(), opts, &RunResult{}, "main")
	if err == nil || !strings.Contains(err.Error(), "name-status") {
		t.Fatalf("want name-status error, got %v", err)
	}
}

// --- promoteInbox: readStateMap error (postship.go:83) ----------------------

func TestPromoteInbox_CycleStateReadError_ReturnsError(t *testing.T) {
	repo := makeRepo(t)

	// Replace cycle-state.json with a directory so readStateMap errors.
	csPath := filepath.Join(repo, ".evolve", "cycle-state.json")
	if err := os.MkdirAll(csPath, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(csPath) })

	opts := &Options{ProjectRoot: repo}
	err := promoteInbox(context.Background(), opts, &RunResult{})
	if err == nil {
		t.Fatal("want readStateMap error, got nil")
	}
}

// --- verifyManualConfirm: git add -A runner error (verify.go:157) -----------

func TestVerifyManualConfirm_GitAddAFails_Errors(t *testing.T) {
	opts := &Options{
		ProjectRoot: t.TempDir(),
		Runner: func(ctx context.Context, name, cwd string, args, env []string,
			stdin io.Reader, stdout, stderr io.Writer) (int, error) {
			// Fail git add -A specifically (args[0]=="add").
			if name == "git" && len(args) > 0 && args[0] == "add" {
				return -1, errors.New("add -A exploded")
			}
			return 0, nil
		},
		Stderr: io.Discard,
		Stdin:  strings.NewReader(""),
	}
	err := verifyManualConfirm(context.Background(), opts, &RunResult{})
	if err == nil || !strings.Contains(err.Error(), "git add -A failed") {
		t.Fatalf("want 'git add -A failed' error, got %v", err)
	}
}

// --- verifyTrivial: diff --name-only runner error (verify.go:267) -----------

func TestVerifyTrivial_DiffNameOnlyRunnerError_Errors(t *testing.T) {
	repo := makeRepo(t)

	// Write cycle-state.json with cycle_size_estimate=trivial.
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":1,"cycle_size_estimate":"trivial"}`)

	opts := &Options{
		ProjectRoot: repo,
		Runner: func(ctx context.Context, name, cwd string, args, env []string,
			stdin io.Reader, stdout, stderr io.Writer) (int, error) {
			// verifyTrivial calls: git diff --cached --name-only
			// then git diff --name-only, then git ls-files ...
			// Fail the second diff call: --name-only without --cached.
			if name == "git" && argsContain(args, "--name-only") && !argsContain(args, "--cached") {
				return -1, errors.New("diff name-only exploded")
			}
			return execRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
		},
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	err := verifyTrivial(context.Background(), opts, &RunResult{})
	if err == nil {
		t.Fatal("want runner error from diff --name-only, got nil")
	}
}

// --- verifyTrivial: ls-files runner error (verify.go:271) ------------------

func TestVerifyTrivial_LSFilesRunnerError_Errors(t *testing.T) {
	repo := makeRepo(t)

	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":1,"cycle_size_estimate":"trivial"}`)

	opts := &Options{
		ProjectRoot: repo,
		Runner: func(ctx context.Context, name, cwd string, args, env []string,
			stdin io.Reader, stdout, stderr io.Writer) (int, error) {
			if name == "git" && argsContain(args, "ls-files") {
				return -1, errors.New("ls-files exploded")
			}
			return execRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
		},
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	err := verifyTrivial(context.Background(), opts, &RunResult{})
	if err == nil {
		t.Fatal("want runner error from ls-files, got nil")
	}
}

// --- native.go:213: postShip WARN log (postShip returns error) --------------
// postShip errors log a WARN but do not fail the ship (native.go:213-216).
// Trigger by making cycle-state.json a directory → promoteInbox errors →
// postShip returns that error → Run logs WARN and continues to ExitOK.

func TestRun_PostShipError_LogsWarnAndContinues(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	mustWrite(t, filepath.Join(repo, "warn.txt"), "change\n")
	seedAudit(t, repo, "PASS")

	// Replace cycle-state.json with a directory so postShip fails on
	// advanceLastCycleNumber (readStateMap error → postShip returns error →
	// native.go:213 logs WARN and continues).
	// cycle-state.json doesn't exist in makeRepo, so just MkdirAll it.
	csPath := filepath.Join(repo, ".evolve", "cycle-state.json")
	_ = os.Remove(csPath) // harmless if absent
	if err := os.MkdirAll(csPath, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(csPath) })

	// We also need advanceLastCycleNumber to fail first — it reads cycle-state.json
	// which is now a dir. It will return error, postShip propagates → native logs WARN.
	var stderrBuf strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := Run(ctx, Options{
		Class:          ClassCycle,
		CommitMessage:  "feat: post-ship warn",
		ProjectRoot:    repo,
		PluginRoot:     repo,
		ShipBinaryPath: filepath.Join(repo, "ship-binary-fixture"),
		Runner:         execRunner,
		Stdin:          strings.NewReader(""),
		Stdout:         io.Discard,
		Stderr:         &stderrBuf,
	})
	// Ship should succeed (postShip errors are WARN-only).
	if err != nil {
		t.Fatalf("postShip WARN must not propagate as error, got: %v", err)
	}
	if res.ExitCode != ExitOK {
		t.Fatalf("want ExitOK despite postShip error, got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	// The WARN must appear in logs.
	found := false
	for _, l := range res.Logs {
		if strings.Contains(l, "post-ship hook error") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("want 'post-ship hook error' WARN in logs; got %v", res.Logs)
	}
}
