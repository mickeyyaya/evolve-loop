// realgit_testhelpers_test.go — shared test helpers for the ship package.
//
// NO build tag: this file is compiled in both the fast (default) tier and the
// integration tier. Functions that spawn real git live here so integration-tagged
// files can call them, and pure file-system helpers (mustWrite, mustMkdir,
// containsLog, writeAttestation) live here so untagged fast-tier files can call
// them without pulling in the integration tag.
package ship

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- pure file-system helpers (used by fast-tier untagged files) -----------

// mustWrite creates parent dirs and writes content to path, failing the test
// on any error.
func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// mustMkdir creates path (and all parents), failing the test on error.
func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

// containsLog reports whether any entry in res.Logs contains substr.
func containsLog(res RunResult, substr string) bool {
	for _, l := range res.Logs {
		if strings.Contains(l, substr) {
			return true
		}
	}
	return false
}

// writeAttestation writes a commit-gate attestation JSON under
// <repo>/.commit-gate/attestation.json with three canonical reviewers.
// Used by reviewtrailer_test.go (fast tier) and commitgate_test.go
// (integration tier).
func writeAttestation(t *testing.T, repo, treeSHA string) {
	t.Helper()
	mustMkdir(t, filepath.Join(repo, ".commit-gate"))
	body := fmt.Sprintf(`{"tree_state_sha":%q,"ts":"2026-05-27T00:00:00Z","checks_passed":["go:gofmt","go:test"],"reviewers_run":["code-simplifier","code-reviewer","go-reviewer"],"tool":"shasum"}`+"\n", treeSHA)
	mustWrite(t, filepath.Join(repo, ".commit-gate", "attestation.json"), body)
}

// --- real-git helpers (called only from integration-tagged test functions) --

// tempRepoDir returns a fresh temp directory for a git repo whose cleanup is
// BEST-EFFORT — unlike t.TempDir(), a RemoveAll failure does NOT fail the test.
//
// On macOS CI runners, os.RemoveAll of a git work tree intermittently fails
// with EBADF ("bad file descriptor") on .git internals (e.g. a hooks/*.sample
// file) under -race load. With t.TempDir() that cleanup error fails an
// otherwise-passing test and forces a full ~5-minute CI re-run (observed on
// TestShipFromWorktree_GitAddFails_Errors, 2026-06-02). The temp dir is
// ephemeral — CI reclaims it regardless — so best-effort removal is safe and
// keeps a cosmetic cleanup race from gating a green build. The chmod-walk makes
// git's 0444 pack/object files removable.
func tempRepoDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "shiptest-*")
	if err != nil {
		t.Fatalf("mkdir temp repo: %v", err)
	}
	t.Cleanup(func() {
		_ = filepath.WalkDir(dir, func(p string, _ fs.DirEntry, walkErr error) error {
			if walkErr == nil {
				_ = os.Chmod(p, 0o755)
			}
			return nil
		})
		_ = os.RemoveAll(dir)
	})
	return dir
}

// makeRepo creates a fresh git repo with:
//   - fixture.txt tracked
//   - .gitignore (.evolve/)
//   - empty .evolve/{ledger.jsonl,state.json}
//   - a stub ship-binary-fixture file (TOFU pins its SHA)
//   - initial commit "initial test repo"
//
// Returns the absolute repo path. Cleanup is best-effort via tempRepoDir.
func makeRepo(t *testing.T) string {
	t.Helper()
	repo := tempRepoDir(t)
	mustWrite(t, filepath.Join(repo, ".gitignore"), ".evolve/\n")
	mustMkdir(t, filepath.Join(repo, ".evolve", "runs", "cycle-1"))
	mustWrite(t, filepath.Join(repo, ".evolve", "ledger.jsonl"), "")
	mustWrite(t, filepath.Join(repo, ".evolve", "state.json"), "{}\n")
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\n")
	// Ship binary fixture: stable initial content. Tests that need to
	// "tamper" with the ship binary modify this file directly.
	mustWrite(t, filepath.Join(repo, "ship-binary-fixture"), "ship-binary-v1\n")

	runGit(t, repo, "init", "-q")
	runGit(t, repo, "config", "user.email", "test@evolve-loop.test")
	runGit(t, repo, "config", "user.name", "Test User")
	runGit(t, repo, "config", "core.hooksPath", "/dev/null")
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "initial test repo")
	return repo
}

// addRemote creates a bare repo to serve as origin and registers it.
// Also forces branch to main and pushes initial commit so `git push`
// is fast-forward later.
func addRemote(t *testing.T, repo string) string {
	t.Helper()
	bare := filepath.Join(tempRepoDir(t), "remote.git")
	out, err := captureWithEBADFRetry(func() ([]byte, error) {
		return exec.Command("git", "init", "-q", "--bare", bare).CombinedOutput()
	})
	if err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	runGit(t, repo, "remote", "add", "origin", bare)
	runGit(t, repo, "branch", "-M", "main")
	return bare
}

// seedAudit writes audit-report.md + ledger.jsonl entry mirroring the
// bash seed_audit helper. exitCode defaults to 0; pass -1 to use the
// override. verdict text is embedded into the report body.
//
// optOverrides: optional map with keys "head", "tree", "exit_code" to
// override the ledger entry fields. Use to test mismatch cases.
func seedAudit(t *testing.T, repo, verdict string, optOverrides ...map[string]string) {
	t.Helper()
	auditPath := filepath.Join(repo, ".evolve", "runs", "cycle-1", "audit-report.md")
	body := fmt.Sprintf("<!-- challenge-token: testtoken123 -->\n# Audit Report — Cycle 1\n\nVerdict: %s\n\nAll criteria met (test fixture).\n", verdict)
	mustWrite(t, auditPath, body)
	sha := mustHashFile(t, auditPath)

	overrides := map[string]string{}
	if len(optOverrides) > 0 {
		overrides = optOverrides[0]
	}

	headSHA := overrides["head"]
	if headSHA == "" {
		headSHA = strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))
	}
	treeSHA := overrides["tree"]
	if treeSHA == "" {
		treeSHA = treeStateSHA(t, repo)
	}
	exitCode := 0
	if v := overrides["exit_code"]; v != "" {
		fmt.Sscanf(v, "%d", &exitCode)
	}

	entry := map[string]any{
		"ts":              "2026-04-27T00:00:00Z",
		"cycle":           1,
		"role":            "auditor",
		"kind":            "agent_subprocess",
		"model":           "sonnet",
		"exit_code":       exitCode,
		"duration_s":      "30",
		"artifact_path":   auditPath,
		"artifact_sha256": sha,
		"challenge_token": "testtoken123",
		"git_head":        headSHA,
		"tree_state_sha":  treeSHA,
	}
	line, _ := json.Marshal(entry)
	mustWrite(t, filepath.Join(repo, ".evolve", "ledger.jsonl"), string(line)+"\n")
}

// treeStateSHA computes sha256(git diff HEAD) — the same fingerprint
// the audit-binding model uses. Wraps the git invocation for test setup.
func treeStateSHA(t *testing.T, repo string) string {
	t.Helper()
	out := runGitOut(t, repo, "diff", "HEAD")
	h := sha256.New()
	_, _ = h.Write([]byte(out))
	return hex.EncodeToString(h.Sum(nil))
}

func mustHashFile(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		t.Fatalf("hash %s: %v", path, err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func runGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	out, err := captureWithEBADFRetry(func() ([]byte, error) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = filteredEnv()
		return cmd.CombinedOutput()
	})
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func runGitOut(t *testing.T, repo string, args ...string) string {
	t.Helper()
	out, err := captureWithEBADFRetry(func() ([]byte, error) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = filteredEnv()
		return cmd.Output()
	})
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}

// filteredEnv strips evolve-loop env vars from the parent test process
// so we don't pick up the operator's actual state.
func filteredEnv() []string {
	var out []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "EVOLVE_") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// runShip invokes the native Run() with sensible defaults for testing.
// Returns the result; tests assert on .ExitCode and .Logs.
func runShip(t *testing.T, repo string, opts Options) (RunResult, error) {
	t.Helper()
	if opts.ProjectRoot == "" {
		opts.ProjectRoot = repo
	}
	if opts.PluginRoot == "" {
		opts.PluginRoot = repo
	}
	if opts.ShipBinaryPath == "" {
		opts.ShipBinaryPath = filepath.Join(repo, "ship-binary-fixture")
	}
	// Use real exec runner for tests — they assert real git semantics.
	if opts.Runner == nil {
		opts.Runner = execRunner
	}
	// Default to bytes.Buffer for stderr capture so tests can assert log lines.
	var stderr bytes.Buffer
	if opts.Stderr == nil {
		opts.Stderr = &stderr
	}
	if opts.Stdin == nil {
		opts.Stdin = bytes.NewReader(nil)
	}
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return Run(ctx, opts)
}

// makeWorktree adds a git worktree of repo on a fresh branch at main and
// returns its absolute path. The worktree shares repo's object store, so a
// commit there is ff-mergeable into main.
func makeWorktree(t *testing.T, repo, branch string) string {
	t.Helper()
	wt := filepath.Join(t.TempDir(), "wt")
	runGit(t, repo, "worktree", "add", "-b", branch, wt, "main")
	return wt
}

// seedAuditWithBoundTree is seedAudit plus an `audit_bound_tree_sha:` line
// in the report body, so verifyAuditBinding stashes it into
// opts.internalAuditBoundTreeSHA and gitops enforces the pre-merge check.
func seedAuditWithBoundTree(t *testing.T, repo, verdict, boundTreeSHA string) {
	t.Helper()
	auditPath := filepath.Join(repo, ".evolve", "runs", "cycle-1", "audit-report.md")
	body := fmt.Sprintf("<!-- challenge-token: testtoken123 -->\n# Audit Report — Cycle 1\n\nVerdict: %s\naudit_bound_tree_sha: %s\n\nAll criteria met (test fixture).\n", verdict, boundTreeSHA)
	mustWrite(t, auditPath, body)
	sha := mustHashFile(t, auditPath)
	headSHA := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))
	treeSHA := treeStateSHA(t, repo)
	entry := map[string]any{
		"ts": "2026-04-27T00:00:00Z", "cycle": 1, "role": "auditor",
		"kind": "agent_subprocess", "model": "sonnet", "exit_code": 0,
		"duration_s": "30", "artifact_path": auditPath, "artifact_sha256": sha,
		"challenge_token": "testtoken123", "git_head": headSHA, "tree_state_sha": treeSHA,
	}
	line, _ := json.Marshal(entry)
	mustWrite(t, filepath.Join(repo, ".evolve", "ledger.jsonl"), string(line)+"\n")
}

// makeWorktreeScenario returns (mainRepo, worktreePath) where:
//   - mainRepo has a remote, a committed file, and a seeded audit
//   - worktreePath is a linked worktree on branch "cycle-1"
//     with one staged change so there are uncommitted changes to ship
func makeWorktreeScenario(t *testing.T) (string, string) {
	t.Helper()
	repo := makeRepo(t)
	addRemote(t, repo)
	seedAudit(t, repo, "PASS")

	// Create a linked worktree on a new branch.
	wt := tempRepoDir(t)
	runGit(t, repo, "worktree", "add", "-b", "cycle-1", wt)

	// Stage a file in the worktree so worktreeCleanNoCommit == false.
	mustWrite(t, filepath.Join(wt, "wt-change.txt"), "worktree change\n")
	runGit(t, wt, "add", "wt-change.txt")

	return repo, wt
}
