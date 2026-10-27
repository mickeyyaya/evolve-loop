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
	t.Parallel()
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

	t.Run("trigger_81_falls_back_to_codex", func(t *testing.T) {
		t.Parallel()
		runFallbackCycle(t, fallbackCfg{
			EvolveBin: evolveBin, FakeBin: fakeBin, RepoRoot: repoRoot,
			PrimaryExitCode: 81, ExpectShip: true,
		})
	})

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
		"EVOLVE_RESEARCH_HOOK_DISABLED=1",
		fakeCLIsFirstOnPath(t, cfg.FakeBin, "claude", "codex"),
		fmt.Sprintf("FAKE_CLI_CLAUDE_EXIT=%d", cfg.PrimaryExitCode),
	)
	env = append(env, isolatedHome(t)...)

	args := []string{"cycle", "run",
		"--project-root", projRoot,
		"--goal-hash", fmt.Sprintf("e2efb%d", cfg.PrimaryExitCode),
		"--evolve-dir", filepath.Join(projRoot, ".evolve"),
		"--sandbox-off=e2e fake CLIs need no confinement",
	}
	cmd := exec.Command(cfg.EvolveBin, args...)
	cmd.Env = env
	cmd.Dir = projRoot
	out, err := runUntilLedgerRole(cmd, projRoot, "ship", cfg.ExpectShip, 1500*time.Second)

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

func gitLogContains(t *testing.T, projRoot, sub string) bool {
	t.Helper()
	logOut, err := exec.Command("git", "-C", projRoot, "log", "--format=%s", "-1").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(logOut), sub)
}

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
