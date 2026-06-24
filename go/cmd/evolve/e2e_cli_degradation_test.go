//go:build e2e

// End-to-end coverage of the CLI fallback chain (ADR-0029 / Workstream G)
// driven through `evolve cycle run`: when the primary CLI fails with a
// trigger exit code, the runner must retry the phase on the next allowed CLI
// and the cycle must still complete.
//
// Scope note — what this asserts vs. what lives elsewhere:
//   - Fallback ORCHESTRATION (the runner looping candidates on a trigger code)
//     is exercised here, end-to-end, because nothing else covers it.
//   - The require-full → exit-99 GATE (BRIDGE_REQUIRE_FULL with a sub-full
//     tier) is already covered at the bridge level — see
//     internal/bridge/launch_modes_test.go:TestLaunchArgs_RequireFull_Unmet
//     and coverage_batch2_test.go:TestRequireFull_ManifestMissing — so it is
//     NOT re-tested here. Exit 99 appears below only as a NON-trigger code
//     that must NOT fall back.
//   - The bash adapters' graceful-degradation stub (missing binary → stub
//     artifact → exit 0) does NOT exist in the v11+ Go path; the Go path
//     returns a fallback-trigger exit code (e.g. 127 ExitMissingBinary) and
//     relies on the fallback chain instead. Asserting the bash stub here would
//     test behavior the Go path does not have. See docs/TEST_PLAN.md.
//
// Deterministic + host-independent: the fake's per-CLI exit injection
// (FAKE_CLI_CLAUDE_EXIT) makes the primary claude-p fail with a chosen code
// while codex (same fake binary, codex invocation style) succeeds.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	// Default fallback triggers (cli_chain.go:defaultFallbackOnExit):
	// 80 boot-timeout, 81 artifact-timeout, 124 timeout(1), 127 missing-binary.
	// Each must make the primary claude-p fall back to codex and still ship.
	for _, code := range []int{80, 81, 124, 127} {
		code := code
		t.Run(fmt.Sprintf("trigger_%d_falls_back_to_codex", code), func(t *testing.T) {
			t.Parallel()
			runFallbackCycle(t, fallbackCfg{
				EvolveBin: evolveBin, FakeBin: fakeBin, RepoRoot: repoRoot,
				PrimaryExitCode: code, ExpectShip: true,
			})
		})
	}

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
	out, err := runWithTimeout(cmd, 120*time.Second)

	// The primary (claude-p) ALWAYS fails with the chosen code. Whether the
	// cycle REACHES the ship phase is read from the ledger role — the
	// orchestrator records role="ship" the moment ship is attempted
	// (orchestrator.recordShipError) — rather than a landed commit: native-only
	// ship cannot complete a real ff-merge in this fixture (no seeded remote +
	// audit binding; that successful-native-ship e2e is deferred to go/test/e2e/
	// — see PORTING-LEDGER.md). Reaching ship at all PROVES every phase fell
	// back to codex, since the primary fails on every invocation. The legacy
	// EVOLVE_NATIVE_SHIP=0 + EVOLVE_SHIP_SCRIPT fake-ship hatch this test was
	// first authored against was removed in the Go-only consolidation.
	entries := readLedger(t, projRoot)
	reachedShip := ledgerHasRole(entries, "ship")

	if cfg.ExpectShip {
		if !reachedShip {
			t.Logf("--- combined output ---\n%s", out)
			dumpWorkspaceLogs(t, projRoot)
			t.Errorf("trigger exit=%d should fall back to codex and reach the ship phase; ledger roles=%v", cfg.PrimaryExitCode, ledgerRoles(entries))
		}
	} else {
		// 99 is not a trigger → no fallback → the cycle must fail at the first
		// phase and never reach ship.
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
