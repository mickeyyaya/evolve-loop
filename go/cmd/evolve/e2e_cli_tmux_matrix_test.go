//go:build e2e

// End-to-end matrix that drives a FULL evolve cycle (Scout → Triage → TDD →
// Build → Audit → Ship) through the INTERACTIVE tmux drivers — claude-tmux,
// codex-tmux, agy-tmux — against a REAL tmux server, with evolve-fake-cli
// serving a persistent REPL in place of the real CLI binary.
//
// This is the tmux companion to e2e_cycle_cli_matrix_test.go (which covers
// the 3 HEADLESS drivers). Together they assert the "any CLI × any phase"
// invariant end-to-end for every shipped driver on the happy path.
//
// How the offline tmux path is made deterministic:
//   - BRIDGE_TESTING=1 + BRIDGE_<CLI>_BINARY point the driver at the fake
//     (resolveBinary honors this for tmux drivers too — verified at
//     driver_claudetmux.go).
//   - The fake auto-detects REPL mode (no -p / no `exec`) and prints a boot
//     line containing EVERY driver's hardcoded marker (❯ / › / "? for
//     shortcuts" / ">>> "). The drivers hardcode these and ignore the
//     manifest prompt_marker, so no manifest override is needed — one fake
//     satisfies any driver's boot-ready capture-pane check.
//   - HOME + EVOLVE_CODEX_CONFIG_PATH are redirected to temp dirs so the
//     codex/agy tmux preflights never touch the operator's real ~/.codex,
//     and so a fancy user shell prompt (starship/pure use ❯) can't trip a
//     false boot-ready from the bare tmux shell.
//
// Live opt-in (EVOLVE_E2E_LIVE=1) runs one real-binary tmux cycle per CLI,
// auto-skipped when the binary is absent — same gating as the headless matrix.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// envDurationSeconds reads an integer-seconds env override, falling back to
// def when unset or unparseable.
func envDurationSeconds(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return def
}

// tmuxCLIs is the interactive-driver matrix domain.
var tmuxCLIs = []string{"claude-tmux", "codex-tmux", "agy-tmux"}

func TestE2ECycleTmuxMatrix(t *testing.T) {
	// Stage 5.1 (Go-only consolidation): like its headless twin
	// TestE2ECycleCLIMatrix, this happy-path-through-ship matrix fake-shipped
	// via the now-removed EVOLVE_NATIVE_SHIP=0 + EVOLVE_SHIP_SCRIPT legacy
	// hatch. With native-only ship the shipper can no longer be stubbed here:
	// the tmux drivers DO complete all six phases (verified), but the native
	// ff-merge into the fixture's main can't succeed without a seeded bare
	// remote + audit-binding + tracked-state.json handling. A proper native
	// tmux-matrix e2e belongs in go/test/e2e/ alongside its headless twin —
	// tracked in go/test/trustkernel/PORTING-LEDGER.md.
	t.Skip("legacy ship-script hatch removed (Stage 5.1); native tmux-matrix e2e pending port to go/test/e2e/ — see PORTING-LEDGER.md")
	if testing.Short() {
		t.Skip("E2E test; skipped in -short mode")
	}
	for _, bin := range []string{"git", "bash", "tmux"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("required tool %q not on PATH; skipping tmux E2E", bin)
		}
	}

	repoRoot := mustRepoRoot(t)
	binDir := t.TempDir()
	evolveBin := buildBinary(t, binDir, "evolve", "./cmd/evolve", repoRoot)
	fakeBin := buildBinary(t, binDir, "evolve-fake-cli", "./cmd/evolve-fake-cli", repoRoot)

	for _, cli := range tmuxCLIs {
		cli := cli
		// NOT parallel: real-tmux boot timing gets flaky under concurrent
		// server load (documented in tmux_repl_integration_test.go).
		t.Run("offline_"+cli, func(t *testing.T) {
			runOneTmuxCycle(t, tmuxCycleConfig{
				CLI: cli, EvolveBin: evolveBin, FakeBin: fakeBin, RepoRoot: repoRoot, Live: false,
			})
		})
	}

	if os.Getenv("EVOLVE_E2E_LIVE") != "1" {
		t.Logf("EVOLVE_E2E_LIVE not set; skipping live tmux sub-tests. Set EVOLVE_E2E_LIVE=1 to run.")
		return
	}
	for _, cli := range tmuxCLIs {
		cli := cli
		t.Run("live_"+cli, func(t *testing.T) {
			runOneTmuxCycle(t, tmuxCycleConfig{
				CLI: cli, EvolveBin: evolveBin, FakeBin: fakeBin, RepoRoot: repoRoot, Live: true,
			})
		})
	}
}

type tmuxCycleConfig struct {
	CLI       string
	EvolveBin string
	FakeBin   string
	RepoRoot  string
	Live      bool
}

func runOneTmuxCycle(t *testing.T, cfg tmuxCycleConfig) {
	t.Helper()
	projRoot := setupTempProject(t, cfg.RepoRoot)
	shipScript := writeFakeShipScript(t, projRoot)
	fakeHome := t.TempDir()

	env := append(os.Environ(),
		"EVOLVE_CLI="+cfg.CLI,
		"EVOLVE_PROMPTS_DIR="+cfg.RepoRoot,
		"EVOLVE_SHIP_SCRIPT="+shipScript,
		"EVOLVE_NATIVE_SHIP=0",
		"EVOLVE_STRICT_AUDIT=0",
		"EVOLVE_RESEARCH_HOOK_DISABLED=1",
		// Redirect codex's pre-trust write away from the real ~/.codex.
		"EVOLVE_CODEX_CONFIG_PATH="+filepath.Join(fakeHome, ".codex", "config.toml"),
		"HOME="+fakeHome,
	)
	if !cfg.Live {
		env = append(env,
			"BRIDGE_TESTING=1",
			"BRIDGE_CLAUDE_BINARY="+cfg.FakeBin,
			"BRIDGE_CODEX_BINARY="+cfg.FakeBin,
			"BRIDGE_AGY_BINARY="+cfg.FakeBin,
		)
	} else if _, err := exec.LookPath(liveBinaryName(cfg.CLI)); err != nil {
		t.Skipf("live mode: %s binary not on PATH (%v); skipping", cfg.CLI, err)
	}

	args := []string{"cycle", "run",
		"--project-root", projRoot,
		"--goal-hash", "e2etmux" + cfg.CLI,
		"--evolve-dir", filepath.Join(projRoot, ".evolve"),
	}
	if cfg.Live {
		args = append(args, "--budget-usd", "0.50")
	}

	cmd := exec.Command(cfg.EvolveBin, args...)
	cmd.Env = env
	cmd.Dir = projRoot

	// tmux adds real per-phase boot + poll overhead vs the headless matrix
	// (every phase spawns a session, polls for the boot marker, then polls
	// the artifact at 2s). The whole 6-phase cycle is several minutes of
	// wall-clock; tunable via env for slower CI hosts.
	timeout := envDurationSeconds("EVOLVE_E2E_TMUX_TIMEOUT_S", 7*time.Minute)
	if cfg.Live {
		timeout = envDurationSeconds("EVOLVE_E2E_TMUX_LIVE_TIMEOUT_S", 12*time.Minute)
	}
	out, err := runWithTimeout(cmd, timeout)
	if err != nil {
		t.Logf("--- combined output ---\n%s", out)
		dumpWorkspaceLogs(t, projRoot)
		t.Fatalf("evolve cycle run (%s) failed: %v", cfg.CLI, err)
	}

	entries := readLedger(t, projRoot)
	if len(entries) == 0 {
		dumpWorkspaceLogs(t, projRoot)
		t.Fatalf("ledger is empty; cycle output:\n%s", out)
	}
	for _, want := range expectedPhasesHappyPath {
		if !ledgerHasRole(entries, want) {
			dumpWorkspaceLogs(t, projRoot)
			t.Errorf("ledger missing role=%q\nfull ledger roles: %v\noutput:\n%s", want, ledgerRoles(entries), out)
		}
	}
	if state := readState(t, projRoot); state.LastCycleNumber < 1 {
		t.Errorf("state.json:lastCycleNumber=%d, want >=1", state.LastCycleNumber)
	}
	logOut, err := exec.Command("git", "-C", projRoot, "log", "--format=%s", "-1").Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if !strings.Contains(string(logOut), "e2etmux") {
		t.Errorf("git log missing e2etmux ship commit; got %q", string(logOut))
	}
}
