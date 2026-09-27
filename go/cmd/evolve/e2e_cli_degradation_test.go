//go:build e2e

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestE2ECLIFallbackChain(t *testing.T) {
	if testing.Short() {
		t.Skip("E2E test; skipped in -short mode")
	}
	for _, bin := range []string{"git", "bash"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("required tool %q not on PATH; skipping fallback E2E", bin)
		}
	}
	repoRoot := mustRepoRoot(t)
	binDir := t.TempDir()
	evolveBin := buildBinary(t, binDir, "evolve", "./cmd/evolve", repoRoot)
	fakeBin := buildBinary(t, binDir, "evolve-fake-cli", "./cmd/evolve-fake-cli", repoRoot)

	// ONE representative trigger walks the whole spine end to end: 81
	// (artifact-timeout) is the code that actually fires in production.
	t.Run("trigger_81_falls_back_to_codex", func(t *testing.T) {
		t.Parallel()
		runFallbackCycle(t, fallbackCfg{
			EvolveBin: evolveBin, FakeBin: fakeBin, RepoRoot: repoRoot,
			PrimaryExitCode: 81, ExpectShip: true,
		})
	})

	// 99 (ExitRequireFullUnmet) is NOT a fallback trigger — the primary fails,
	// the runner does NOT try codex, and the cycle must fail.
	t.Run("nontrigger_99_does_not_fall_back", func(t *testing.T) {
		t.Parallel()
		runFallbackCycle(t, fallbackCfg{
			EvolveBin: evolveBin, FakeBin: fakeBin, RepoRoot: repoRoot,
			PrimaryExitCode: 99, ExpectShip: false,
		})
	})
}

type fallbackCfg struct {
	EvolveBin       string
	FakeBin         string
	RepoRoot        string
	PrimaryExitCode int
	ExpectShip      bool
}

func runFallbackCycle(t *testing.T, cfg fallbackCfg) {
	t.Helper()
	projRoot := setupTempProject(t, cfg.RepoRoot)
	writeFallbackProfiles(t, projRoot, "claude-p", []string{"codex"})

	env := append(os.Environ(),
		"EVOLVE_SANDBOX=off", // host opt-out: pipeline-semantics tests, runners cannot promise a wrap (see e2e_pipeline_paths_test.go)
		"EVOLVE_PROMPTS_DIR="+cfg.RepoRoot,
		"EVOLVE_RESEARCH_HOOK_DISABLED=1",
		// No EVOLVE_CLI: the per-agent profile (cli + cli_fallback) drives the
		// chain. claude-p is primary; codex is the fallback.
		"BRIDGE_TESTING=1",
		"BRIDGE_CLAUDE_BINARY="+cfg.FakeBin,
		"BRIDGE_CODEX_BINARY="+cfg.FakeBin,
		// Make every claude-p invocation fail with the chosen code; codex
		// (the fallback) is uninjected and succeeds.
		fmt.Sprintf("FAKE_CLI_CLAUDE_EXIT=%d", cfg.PrimaryExitCode),
	)

	args := []string{"cycle", "run",
		"--project-root", projRoot,
		"--goal-hash", fmt.Sprintf("e2efb%d", cfg.PrimaryExitCode),
		"--evolve-dir", filepath.Join(projRoot, ".evolve"),
	}
	cmd := exec.Command(cfg.EvolveBin, args...)
	cmd.Env = env
	cmd.Dir = projRoot
	// CEILING, not a target: the harness polls the ledger and stops as soon as
	// the target role appears, so a fast host pays nothing extra and a slow one
	// still gets the full ceiling — the result depends on the invariant, not
	// the host's speed.
	out, err := runUntilLedgerRole(cmd, projRoot, "ship", cfg.ExpectShip, 1500*time.Second)

	// Whether the cycle reaches ship is read from the ledger role, not a landed
	// commit (native ship cannot ff-merge in this fixture). Reaching ship at
	// all proves every phase fell back to codex, since the primary fails on
	// every invocation.
	entries := readLedger(t, projRoot)
	reachedShip := ledgerHasRole(entries, "ship")

	if cfg.ExpectShip {
		if !reachedShip {
			t.Logf("--- combined output ---\n%s", out)
			dumpWorkspaceLogs(t, projRoot)
			t.Errorf("trigger exit=%d should fall back to codex and reach the ship phase; ledger roles=%v", cfg.PrimaryExitCode, ledgerRoles(entries))
		}
	} else {
		if err == nil {
			t.Errorf("cycle should FAIL (exit %d is not a fallback trigger), but it succeeded\noutput:\n%s", cfg.PrimaryExitCode, out)
		}
		if reachedShip {
			t.Errorf("non-trigger exit %d must NOT fall back, so the cycle must not reach ship; ledger roles=%v", cfg.PrimaryExitCode, ledgerRoles(entries))
		}
	}
}

// writeFallbackProfiles rewrites every phase profile with a primary cli + an
// ordered cli_fallback list. cli_fallback_on_exit is left unset so the runner
// uses the documented default trigger set {80,81,124,127}.
func writeFallbackProfiles(t *testing.T, projRoot, primary string, fallback []string) {
	t.Helper()
	profilesDir := filepath.Join(projRoot, ".evolve", "profiles")
	quoted := make([]string, len(fallback))
	for i, f := range fallback {
		quoted[i] = fmt.Sprintf("%q", f)
	}
	fbJSON := "[" + strings.Join(quoted, ",") + "]"
	for _, name := range []string{"intent", "scout", "triage", "tdd-engineer", "builder", "auditor", "retrospective"} {
		body := fmt.Sprintf(
			`{"name":%q,"role":%q,"cli":%q,"cli_fallback":%s,"model_tier_default":"sonnet","allowed_tools":["Read","Write","Bash"]}`,
			name, name, primary, fbJSON)
		path := filepath.Join(profilesDir, name+".json")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write fallback profile %s: %v", name, err)
		}
	}
}

// gitLogContains reports whether the latest commit subject contains sub.
func gitLogContains(t *testing.T, projRoot, sub string) bool {
	t.Helper()
	logOut, err := exec.Command("git", "-C", projRoot, "log", "--format=%s", "-1").Output()
	if err != nil {
		// No commits beyond init is possible on the fail path; treat as "not found".
		return false
	}
	return strings.Contains(string(logOut), sub)
}

// runUntilLedgerRole starts cmd and returns as soon as the ledger records role
// (when wantRole is true), else waits for the process to exit on its own. The
// poll interval is coarse on purpose: the ledger is appended once per phase, so
// sub-second polling buys nothing. Returns the child's combined output captured
// so far and its exit error (nil when we stopped it after observing the role).
func runUntilLedgerRole(cmd *exec.Cmd, projRoot, role string, wantRole bool, ceiling time.Duration) (string, error) {
	var buf syncBuffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Start(); err != nil {
		return buf.String(), err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	if !wantRole {
		// Non-trigger case: the cycle MUST fail on its own and fail fast. Waiting
		// for the real exit is the assertion.
		select {
		case err := <-done:
			return buf.String(), err
		case <-time.After(ceiling):
			_ = cmd.Process.Kill()
			<-done
			return buf.String(), fmt.Errorf("cycle did not exit within %s", ceiling)
		}
	}

	ledger := filepath.Join(projRoot, ".evolve", "ledger.jsonl")
	deadline := time.Now().Add(ceiling)
	for {
		select {
		case err := <-done:
			// Exited on its own — the caller reads the ledger either way.
			return buf.String(), err
		default:
		}
		if data, rerr := os.ReadFile(ledger); rerr == nil && strings.Contains(string(data), role) {
			_ = cmd.Process.Kill()
			<-done
			return buf.String(), nil
		}
		if !time.Now().Before(deadline) {
			_ = cmd.Process.Kill()
			<-done
			return buf.String(), fmt.Errorf("role %q not recorded within %s", role, ceiling)
		}
		time.Sleep(2 * time.Second)
	}
}

// syncBuffer is a mutex-guarded buffer: the child's output is written from the
// exec goroutine while the poll loop may read it, which a bare bytes.Buffer
// cannot serve race-free.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
