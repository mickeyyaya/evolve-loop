// End-to-end coverage of the ADVERSARIAL pipeline paths that the happy-path
// matrices never exercise: audit FAIL → retro → no-ship, audit WARN under
// fluent vs strict, and the optional intent phase. All run through the
// headless claude-p driver, where env (FAKE_CLI_AUDIT_VERDICT, EVOLVE_*)
// propagates directly to the fake subprocess.
//
// Assertions key off OBSERVABLE ledger routing — which phases the pipeline
// reached (audit, retro, ship) — not the process exit code or a landed ship
// commit. The orchestrator records a role="ship" ledger entry the moment ship
// is attempted (orchestrator.recordShipError), so "reached ship" is provable
// even though native-only ship cannot complete a real ff-merge in this unit
// fixture (no seeded remote + audit binding; that successful-native-ship e2e is
// deferred to go/test/e2e/ — see go/test/trustkernel/PORTING-LEDGER.md). The
// legacy EVOLVE_NATIVE_SHIP=0 + EVOLVE_SHIP_SCRIPT fake-ship hatch these tests
// were first authored against was removed in the Go-only consolidation.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// pipelineCycle runs one headless claude-p cycle with the given env overlay and
// returns the ledger entries. It never fails the test on a non-zero cycle exit —
// a blocked cycle (and, post Stage-5.1, a native ship that cannot ff-merge in
// the fixture) legitimately returns non-zero — so callers assert on the
// ledger-observable routing (which phases the pipeline reached) via ledgerHasRole.
func pipelineCycle(t *testing.T, evolveBin, fakeBin, repoRoot, goalHash string, extraEnv ...string) []ledgerEntry {
	t.Helper()
	projRoot := setupTempProject(t, repoRoot)

	env := append(os.Environ(),
		"EVOLVE_CLI=claude-p",
		"EVOLVE_PROMPTS_DIR="+repoRoot,
		"EVOLVE_RESEARCH_HOOK_DISABLED=1",
		"BRIDGE_TESTING=1",
		"BRIDGE_CLAUDE_BINARY="+fakeBin,
	)
	env = append(env, extraEnv...)

	cmd := exec.Command(evolveBin, "cycle", "run",
		"--project-root", projRoot,
		"--goal-hash", goalHash,
		"--evolve-dir", filepath.Join(projRoot, ".evolve"),
	)
	cmd.Env = env
	cmd.Dir = projRoot
	out, err := runWithTimeout(cmd, 120*time.Second)
	t.Logf("cycle run (%s) err=%v\n%s", goalHash, err, lastN(out, 1200))

	return readLedger(t, projRoot)
}

func lastN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}

func mustBuildPipelineBins(t *testing.T) (evolveBin, fakeBin, repoRoot string) {
	t.Helper()
	if testing.Short() {
		t.Skip("E2E test; skipped in -short mode")
	}
	for _, bin := range []string{"git", "bash"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("required tool %q not on PATH; skipping pipeline E2E", bin)
		}
	}
	repoRoot = mustRepoRoot(t)
	binDir := t.TempDir()
	evolveBin = buildBinary(t, binDir, "evolve", "./cmd/evolve", repoRoot)
	fakeBin = buildBinary(t, binDir, "evolve-fake-cli", "./cmd/evolve-fake-cli", repoRoot)
	return
}

// Audit FAIL (red_count=1) must block the ship and route to the retro phase.
func TestE2EPipeline_AuditFail_RunsRetro_NoShip(t *testing.T) {
	evolveBin, fakeBin, repoRoot := mustBuildPipelineBins(t)
	entries := pipelineCycle(t, evolveBin, fakeBin, repoRoot, "e2efail",
		"FAKE_CLI_AUDIT_VERDICT=FAIL")

	if !ledgerHasRole(entries, "audit") {
		t.Errorf("audit role missing from ledger; roles=%v", ledgerRoles(entries))
	}
	if !ledgerHasRole(entries, "retro") {
		t.Errorf("audit FAIL must route to the retro phase; ledger roles=%v", ledgerRoles(entries))
	}
	if ledgerHasRole(entries, "ship") {
		t.Errorf("audit FAIL must NOT reach the ship phase; ledger roles=%v", ledgerRoles(entries))
	}
}

// Audit WARN ships by default (fluent), but EVOLVE_STRICT_AUDIT=1 promotes it
// to FAIL → block + retro.
func TestE2EPipeline_AuditWarn_FluentShips_StrictBlocks(t *testing.T) {
	evolveBin, fakeBin, repoRoot := mustBuildPipelineBins(t)

	t.Run("fluent_ships", func(t *testing.T) {
		entries := pipelineCycle(t, evolveBin, fakeBin, repoRoot, "e2ewarnfluent",
			"FAKE_CLI_AUDIT_VERDICT=WARN", "EVOLVE_STRICT_AUDIT=0")
		if !ledgerHasRole(entries, "ship") {
			t.Errorf("audit WARN with EVOLVE_STRICT_AUDIT=0 should proceed to the ship phase (fluent); ledger roles=%v", ledgerRoles(entries))
		}
	})

	t.Run("strict_blocks", func(t *testing.T) {
		entries := pipelineCycle(t, evolveBin, fakeBin, repoRoot, "e2ewarnstrict",
			"FAKE_CLI_AUDIT_VERDICT=WARN", "EVOLVE_STRICT_AUDIT=1")
		if ledgerHasRole(entries, "ship") {
			t.Errorf("audit WARN with EVOLVE_STRICT_AUDIT=1 must be promoted to FAIL and NOT reach the ship phase; ledger roles=%v", ledgerRoles(entries))
		}
		if !ledgerHasRole(entries, "retro") {
			t.Errorf("strict WARN→FAIL must route to retro; ledger roles=%v", ledgerRoles(entries))
		}
	})
}

// EVOLVE_REQUIRE_INTENT=1 inserts the intent phase ahead of scout; the cycle
// still reaches ship on the happy path.
func TestE2EPipeline_IntentPhase_RunsAndShips(t *testing.T) {
	evolveBin, fakeBin, repoRoot := mustBuildPipelineBins(t)
	entries := pipelineCycle(t, evolveBin, fakeBin, repoRoot, "e2eintent",
		"EVOLVE_REQUIRE_INTENT=1")

	if !ledgerHasRole(entries, "intent") {
		t.Errorf("EVOLVE_REQUIRE_INTENT=1 should run the intent phase; ledger roles=%v", ledgerRoles(entries))
	}
	if !ledgerHasRole(entries, "ship") {
		t.Errorf("intent-gated happy-path cycle should reach the ship phase; ledger roles=%v", ledgerRoles(entries))
	}
}
