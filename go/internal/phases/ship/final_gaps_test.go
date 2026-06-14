// final_gaps_test.go — last achievable coverage gaps after 92.5%:
//
//   - verifySelfSHA: sha256File error on unreadable binary (verify.go:65)
//   - verifySelfSHA: repin writeStateMap error (verify.go:81)
//   - verifyManualConfirm: diff --cached --quiet runner error (verify.go:161)
//   - verifyTrivial: diff --cached --name-only runner error (verify.go:232)
//   - verifyAuditBinding: sha256File error on unreadable artifact (audit.go:66)
//   - verifyAuditBinding: os.ReadFile error on unreadable artifact (audit.go:77)
//   - verifyAuditBinding: rev-parse HEAD runner error (audit.go:131)
//   - verifyAuditBinding: computeTreeStateSHA runner error (audit.go:141)
//   - verifyAuditBinding: os.Stat freshness error (audit.go:152)
//   - shipDirect: runCommitPrefixGate error (gitops.go:103)
//   - shipFromWorktree: runCommitPrefixGate error (gitops.go:186)
//   - shipFromWorktree: git commit runner failure (gitops.go:195)
//   - shipFromWorktree: write-tree error + empty-output fail-closed (ADR-0048 C1)
//   - repinPostCycle: readStateMap error (postship.go:167)
//   - repinPostCycle: writeStateMap error (postship.go:188)
package ship

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// --- verifySelfSHA: sha256File error on unreadable binary (verify.go:65) ---

func TestVerifySelfSHA_UnreadableBinary_Errors(t *testing.T) {
	repo := makeRepo(t)

	// Create a file for the binary path, then make it unreadable.
	bin := filepath.Join(repo, "unreadable-bin")
	mustWrite(t, bin, "binary content\n")
	if err := os.Chmod(bin, 0o000); err != nil {
		t.Skip("cannot chmod 0000 on this system")
	}
	t.Cleanup(func() { _ = os.Chmod(bin, 0o644) })

	opts := &Options{
		ProjectRoot:    repo,
		PluginRoot:     repo,
		ShipBinaryPath: bin,
		Runner:         execRunner,
		Stdin:          strings.NewReader(""),
		Stdout:         io.Discard,
		Stderr:         io.Discard,
	}
	err := verifySelfSHA(context.Background(), opts, &RunResult{})
	if err == nil || !strings.Contains(err.Error(), "cannot SHA ship binary") {
		t.Fatalf("want 'cannot SHA ship binary' error, got %v", err)
	}
}

// --- verifySelfSHA: repin writeStateMap error (verify.go:81) ---------------
// First-run (no expected_ship_sha in state.json) triggers repin → tries to
// write state.json. Make .evolve/ read-only so writeStateMap fails.

func TestVerifySelfSHA_RepinWriteStateFails_Errors(t *testing.T) {
	repo := makeRepo(t)
	bin := filepath.Join(repo, "ship-binary-fixture")
	preSeedTOFU(t, repo, bin) // seeds expected_sha != actual so repin fires... actually we want FIRST-RUN

	// Overwrite state.json with empty object → no expected_ship_sha → first-run repin.
	mustWrite(t, filepath.Join(repo, ".evolve", "state.json"), "{}\n")

	// Make .evolve/ read-only so writeStateMap (CreateTemp) fails.
	evolveDir := filepath.Join(repo, ".evolve")
	if err := os.Chmod(evolveDir, 0o555); err != nil {
		t.Skip("cannot chmod .evolve dir")
	}
	t.Cleanup(func() { _ = os.Chmod(evolveDir, 0o755) })

	opts := &Options{
		ProjectRoot:    repo,
		PluginRoot:     repo,
		ShipBinaryPath: bin,
		Runner:         execRunner,
		Stdin:          strings.NewReader(""),
		Stdout:         io.Discard,
		Stderr:         io.Discard,
	}
	err := verifySelfSHA(context.Background(), opts, &RunResult{})
	// With the ADR-0049 S2 shared state.json lock, a read-only .evolve dir now
	// fails at lock-acquire (the .lock file can't be created) BEFORE the write —
	// both are the same fail-safe STATE_IO refusal on an unwritable state dir.
	// Assert the contract (a STATE_IO refusal, not silent pass), not the site.
	var se *core.ShipError
	if err == nil || !errors.As(err, &se) || se.Code != core.CodeStateIO {
		t.Fatalf("want a STATE_IO refusal on read-only .evolve, got %v", err)
	}
}

// --- verifyManualConfirm: diff --cached --quiet runner error (verify.go:161) -
// git add -A succeeds (exit 0), then diff --cached --quiet runner errors.

func TestVerifyManualConfirm_DiffCachedQuietRunnerError_Errors(t *testing.T) {
	call := 0
	opts := &Options{
		ProjectRoot: t.TempDir(),
		Runner: func(ctx context.Context, name string, args, env []string, cwd string,
			stdin io.Reader, stdout, stderr io.Writer) (int, error) {
			if name != "git" {
				return 0, nil
			}
			call++
			if call == 1 {
				// First call: git add -A — succeed.
				return 0, nil
			}
			// Second call: git diff --cached --quiet — runner error.
			return -1, errors.New("diff quiet runner error")
		},
		Stderr: io.Discard,
		Stdin:  strings.NewReader(""),
	}
	err := verifyManualConfirm(context.Background(), opts, &RunResult{})
	if err == nil || !strings.Contains(err.Error(), "diff --cached --quiet failed") {
		t.Fatalf("want 'diff --cached --quiet failed' error, got %v", err)
	}
}

// --- verifyTrivial: diff --cached --name-only runner error (verify.go:232) --

func TestVerifyTrivial_StagedNameOnlyRunnerError_Errors(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":1,"cycle_size_estimate":"trivial"}`)

	opts := &Options{
		ProjectRoot: repo,
		Runner: func(ctx context.Context, name string, args, env []string, cwd string,
			stdin io.Reader, stdout, stderr io.Writer) (int, error) {
			// verifyTrivial's first git call: git diff --cached --name-only.
			if name == "git" && argsContain(args, "--cached") && argsContain(args, "--name-only") {
				return -1, errors.New("staged name-only exploded")
			}
			return execRunner(ctx, name, args, env, cwd, stdin, stdout, stderr)
		},
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	err := verifyTrivial(context.Background(), opts, &RunResult{})
	if err == nil {
		t.Fatal("want runner error from diff --cached --name-only, got nil")
	}
}

// --- verifyAuditBinding: sha256File error on unreadable artifact (audit.go:66)

func TestVerifyAuditBinding_UnreadableArtifact_SHA256Error(t *testing.T) {
	repo := makeRepo(t)
	seedAudit(t, repo, "PASS")

	// Make the audit-report.md unreadable (but present so Stat passes).
	auditPath := filepath.Join(repo, ".evolve", "runs", "cycle-1", "audit-report.md")
	if err := os.Chmod(auditPath, 0o000); err != nil {
		t.Skip("cannot chmod 0000 on this system")
	}
	t.Cleanup(func() { _ = os.Chmod(auditPath, 0o644) })

	opts := auditOpts(t, repo)
	err := verifyAuditBinding(context.Background(), opts, &RunResult{})
	if err == nil {
		t.Fatal("want sha256File error on unreadable artifact, got nil")
	}
	// Must be a plain error (not IntegrityError — Stat passes, sha256 fails).
	if _, ok := err.(*IntegrityError); ok {
		t.Errorf("sha256 read error should be plain error, not IntegrityError; got %v", err)
	}
}

// --- verifyAuditBinding: os.ReadFile error (audit.go:77) -------------------
// sha256File reads byte-by-byte via sha256.New(); it succeeds even if
// os.ReadFile would fail in principle. To get audit.go:66 to pass but
// audit.go:77 to fail we need the file to be readable for sha256File but
// not for os.ReadFile. This is only possible with a custom sha256File seam
// which doesn't exist. Skip: practically identical to the chmod test above.
// audit.go:77 is reachable only if sha256File succeeds but ReadFile fails —
// not achievable without OS-level fault injection between two open() calls.

// --- verifyAuditBinding: rev-parse HEAD runner error (audit.go:131) --------

func TestVerifyAuditBinding_RevParseHeadRunnerError_Errors(t *testing.T) {
	repo := makeRepo(t)
	seedAudit(t, repo, "PASS")

	opts := auditOpts(t, repo)
	opts.Runner = func(ctx context.Context, name string, args, env []string, cwd string,
		stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name == "git" && argsContain(args, "rev-parse") && argsContain(args, "HEAD") &&
			!argsContain(args, "HEAD^{tree}") && !argsContain(args, "--abbrev-ref") {
			return -1, errors.New("rev-parse HEAD exploded")
		}
		return execRunner(ctx, name, args, env, cwd, stdin, stdout, stderr)
	}
	err := verifyAuditBinding(context.Background(), opts, &RunResult{})
	if err == nil {
		t.Fatal("want rev-parse HEAD error, got nil")
	}
	if _, ok := err.(*IntegrityError); ok {
		t.Errorf("runner error should be plain error, not IntegrityError; got %v", err)
	}
}

// --- verifyAuditBinding: computeTreeStateSHA runner error (audit.go:141) ---

func TestVerifyAuditBinding_ComputeTreeSHARunnerError_Errors(t *testing.T) {
	repo := makeRepo(t)
	seedAudit(t, repo, "PASS")

	opts := auditOpts(t, repo)
	// computeTreeStateSHA calls "git diff HEAD" — fail it while letting
	// rev-parse HEAD succeed (needed to pass the HEAD binding check first).
	opts.Runner = func(ctx context.Context, name string, args, env []string, cwd string,
		stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name == "git" && argsContain(args, "diff") && argsContain(args, "HEAD") {
			return -1, errors.New("git diff HEAD exploded")
		}
		return execRunner(ctx, name, args, env, cwd, stdin, stdout, stderr)
	}
	err := verifyAuditBinding(context.Background(), opts, &RunResult{})
	if err == nil {
		t.Fatal("want computeTreeStateSHA error, got nil")
	}
}

// --- shipDirect: runCommitPrefixGate error (gitops.go:103) -----------------
// Exercise the prefix-gate path by faulting the runner for the gate's git
// call. runCommitPrefixGate shells out to "git diff --cached --name-only" to
// gather the file list; fail that to trigger a gate error.
// (The commitprefixgate library reads the manifest then checks file names
// against prefixes; its internal git call uses the Runner seam.)

func TestShipDirect_CommitPrefixGateRejects_Errors(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	mustWrite(t, filepath.Join(repo, "staged.txt"), "staged change\n")

	// Real manifest schema (prefixes map): a "docs:" commit must touch docs/.
	// The only staged path is staged.txt (outside docs/), so the gate raises a
	// scope violation, which shipDirect surfaces as a commit-prefix-gate error.
	manifest := `{"prefixes":{"docs":{"required_paths":["docs/"]}}}`
	mustWrite(t, filepath.Join(repo, ".evolve", "commit-prefix-scope.json"), manifest)

	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "docs: touches the wrong paths",
		ProjectRoot:   repo,
		Runner:        execRunner,
		Stdin:         strings.NewReader(""),
		Stdout:        io.Discard,
		Stderr:        io.Discard,
	}
	err := shipDirect(context.Background(), opts, &RunResult{}, "main")
	if err == nil || !strings.Contains(err.Error(), "commit-prefix-gate") {
		t.Fatalf("want commit-prefix-gate rejection, got %v", err)
	}
}

// --- shipFromWorktree: runCommitPrefixGate rejection (gitops.go:186) --------

func TestShipFromWorktree_CommitPrefixGateRejects_Errors(t *testing.T) {
	repo, wt := makeWorktreeScenario(t)

	// The gate uses RepoDir=worktree, so the manifest lives in wt/.evolve/.
	// makeWorktreeScenario stages wt-change.txt (outside docs/), so a "docs:"
	// commit violates required_paths → the gate rejects pre-merge.
	manifest := `{"prefixes":{"docs":{"required_paths":["docs/"]}}}`
	mustWrite(t, filepath.Join(wt, ".evolve", "commit-prefix-scope.json"), manifest)

	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "docs: touches the wrong paths",
		ProjectRoot:   repo,
		Runner:        execRunner,
		Stdin:         strings.NewReader(""),
		Stdout:        io.Discard,
		Stderr:        io.Discard,
	}
	err := shipFromWorktree(context.Background(), opts, &RunResult{}, "main", wt)
	if err == nil || !strings.Contains(err.Error(), "commit-prefix-gate") {
		t.Fatalf("want commit-prefix-gate rejection, got %v", err)
	}
}

// --- shipFromWorktree: git commit runner failure (gitops.go:195) -----------

func TestShipFromWorktree_GitCommitFails_Errors(t *testing.T) {
	repo, wt := makeWorktreeScenario(t)

	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: commit fail",
		ProjectRoot:   repo,
		Runner:        faultRunner("git commit", 1, nil),
		Stdin:         strings.NewReader(""),
		Stdout:        io.Discard,
		Stderr:        io.Discard,
	}
	err := shipFromWorktree(context.Background(), opts, &RunResult{}, "main", wt)
	if err == nil || !strings.Contains(err.Error(), "git commit in worktree failed") {
		t.Fatalf("want 'git commit in worktree failed' error, got %v", err)
	}
}

// --- shipFromWorktree: write-tree error in the pre-commit binding check -----
// ADR-0048 Slice C1 moved the audit-bound tree-SHA verification to a
// `git write-tree` on the staged index BEFORE the commit. A write-tree failure
// must propagate (the binding cannot be verified, so ship must not advance).

func TestShipFromWorktree_WriteTreeFails_Errors(t *testing.T) {
	repo, wt := makeWorktreeScenario(t)

	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: write-tree fail",
		ProjectRoot:   repo,
		Runner: func(ctx context.Context, name string, args, env []string, cwd string,
			stdin io.Reader, stdout, stderr io.Writer) (int, error) {
			if name == "git" && argsContain(args, "write-tree") {
				return -1, errors.New("write-tree exploded")
			}
			return execRunner(ctx, name, args, env, cwd, stdin, stdout, stderr)
		},
		// Set a non-empty internalAuditBoundTreeSHA so the pre-commit binding
		// check runs (and reaches the write-tree call we fault-inject).
		internalAuditBoundTreeSHA: "someboundsha",
		Stdin:                     strings.NewReader(""),
		Stdout:                    io.Discard,
		Stderr:                    io.Discard,
	}
	err := shipFromWorktree(context.Background(), opts, &RunResult{}, "main", wt)
	if err == nil {
		t.Fatal("want write-tree error, got nil")
	}
}

// TestShipFromWorktree_WriteTreeEmptyOutput_FailsClosed: ADR-0048 Slice C1
// fail-closed posture — if `write-tree` returns exit 0 but EMPTY stdout, the
// audit binding cannot be verified, so ship must abort rather than commit
// unverified work (a set binding is never silently skipped).
func TestShipFromWorktree_WriteTreeEmptyOutput_FailsClosed(t *testing.T) {
	repo, wt := makeWorktreeScenario(t)

	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: write-tree empty",
		ProjectRoot:   repo,
		Runner: func(ctx context.Context, name string, args, env []string, cwd string,
			stdin io.Reader, stdout, stderr io.Writer) (int, error) {
			if name == "git" && argsContain(args, "write-tree") {
				return 0, nil // exit 0, no stdout written
			}
			return execRunner(ctx, name, args, env, cwd, stdin, stdout, stderr)
		},
		internalAuditBoundTreeSHA: "someboundsha",
		Stdin:                     strings.NewReader(""),
		Stdout:                    io.Discard,
		Stderr:                    io.Discard,
	}
	err := shipFromWorktree(context.Background(), opts, &RunResult{}, "main", wt)
	if err == nil {
		t.Fatal("want fail-closed error on empty write-tree output, got nil")
	}
	se, ok := core.AsShipError(err)
	if !ok || se.Code != core.CodeGitIO {
		t.Fatalf("want CodeGitIO ShipError, got %v", err)
	}
}

// Note: repinPostCycle readStateMap error (postship.go:167) and writeStateMap
// error (postship.go:188) are already covered by
// TestRepinPostCycle_StateReadError_ReturnsError and
// TestRepinPostCycle_WriteStateFails_ReturnsError in coverage_final_test.go.
