//go:build e2e

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// pipelineCycle runs one headless claude-p cycle with the given env overlay and
// returns the ledger entries. It never fails the test on a non-zero cycle exit
// — callers assert on ledger-observable routing via ledgerHasRole.

// strictPolicyMarker is a harness-only sentinel; it never reaches the cycle
// subprocess. Present in extraEnv, it makes pipelineCycle drop a
// .evolve/policy.json with workflow.strict_audit:true into the project root.
const strictPolicyMarker = "TEST_WRITE_STRICT_POLICY=1"

func pipelineCycle(t *testing.T, evolveBin, fakeBin, repoRoot, goalHash string, extraEnv ...string) ([]ledgerEntry, error) {
	t.Helper()
	projRoot := setupTempProject(t, repoRoot)

	env := append(os.Environ(),
		"EVOLVE_CLI=claude-p",
		"EVOLVE_PROMPTS_DIR="+repoRoot,
		"EVOLVE_RESEARCH_HOOK_DISABLED=1",
		"BRIDGE_TESTING=1",
		"BRIDGE_CLAUDE_BINARY="+fakeBin,
		// Explicit opt-out of OS confinement: these tests exercise pipeline
		// semantics with fake CLIs, not confinement, and CI runners cannot
		// promise a working sandbox wrap (e.g. no bwrap on Ubuntu images). The
		// gate's own classification is pinned by TestSandboxGate_* in internal/bridge.
		"EVOLVE_SANDBOX=off",
	)
	env = append(env, isolatedHome(t)...)
	for _, e := range extraEnv {
		if e == strictPolicyMarker {
			policyPath := filepath.Join(projRoot, ".evolve", "policy.json")
			if err := os.WriteFile(policyPath, []byte(`{"workflow":{"strict_audit":true}}`), 0o644); err != nil {
				t.Fatalf("write strict policy: %v", err)
			}
			continue // marker is harness-only; never forward it to the subprocess
		}
		env = append(env, e)
	}

	cmd := exec.Command(evolveBin, "cycle", "run",
		"--project-root", projRoot,
		"--goal-hash", goalHash,
		"--evolve-dir", filepath.Join(projRoot, ".evolve"),
	)
	cmd.Env = env
	cmd.Dir = projRoot
	// 300s: a probe cycle runs contract activation, the build handoff floor's
	// git diffs, the audit review gate, and up to two correction re-dispatches
	// (a shorter bound kills cycles mid-flight and loses the subprocess output).
	out, err := runWithTimeout(cmd, 300*time.Second)
	t.Logf("cycle run (%s) err=%v\n%s", goalHash, err, lastN(out, 1200))

	return readLedger(t, projRoot), err
}

func isolatedHome(t *testing.T) []string {
	t.Helper()
	keys := []string{"GOCACHE", "GOMODCACHE", "GOPATH", "GOENV"}
	out, err := exec.Command("go", append([]string{"env"}, keys...)...).Output()
	if err != nil {
		t.Fatalf("go env: %v", err)
	}
	values := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(values) != len(keys) {
		t.Fatalf("go env %v printed %d line(s): %q", keys, len(values), out)
	}
	env := []string{"HOME=" + t.TempDir()}
	for i, key := range keys {
		env = append(env, key+"="+values[i])
	}
	return env
}

func TestIsolatedHome_AnEmptyHomeThatKeepsTheParentsGoCaches(t *testing.T) {
	env := append(os.Environ(), isolatedHome(t)...)
	run := func(name string, args ...string) string {
		cmd := exec.Command(name, args...)
		cmd.Env = env
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("%s %v: %v", name, args, err)
		}
		return strings.TrimSpace(string(out))
	}
	home := run("sh", "-c", "printf %s \"$HOME\"")
	if home == "" || home == os.Getenv("HOME") {
		t.Fatalf("a cycle's HOME must be its own, not the host's (%q); got %q", os.Getenv("HOME"), home)
	}
	if entries, err := os.ReadDir(home); err != nil || len(entries) != 0 {
		t.Fatalf("the cycle's HOME starts empty, like CI's; entries=%v err=%v", entries, err)
	}
	for _, key := range []string{"GOCACHE", "GOMODCACHE", "GOPATH"} {
		parent, _ := exec.Command("go", "env", key).Output()
		if got := run("go", "env", key); got != strings.TrimSpace(string(parent)) {
			t.Errorf("%s under the isolated HOME = %q, want the parent's %q, or every go test in the cycle starts cold", key, got, strings.TrimSpace(string(parent)))
		}
	}
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

func TestE2EPipeline_AuditFail_RunsRetro_NoShip(t *testing.T) {
	evolveBin, fakeBin, repoRoot := mustBuildPipelineBins(t)
	entries, _ := pipelineCycle(t, evolveBin, fakeBin, repoRoot, "e2efail",
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

func TestE2EPipeline_AuditWarn_FluentShips_StrictBlocks(t *testing.T) {
	evolveBin, fakeBin, repoRoot := mustBuildPipelineBins(t)

	t.Run("fluent_ships", func(t *testing.T) {
		entries, runErr := pipelineCycle(t, evolveBin, fakeBin, repoRoot, "e2ewarnfluent",
			"FAKE_CLI_AUDIT_VERDICT=WARN")
		if !ledgerHasRole(entries, "ship") {
			t.Errorf("audit WARN with no strict policy should proceed to the ship phase (fluent); ledger roles=%v", ledgerRoles(entries))
		}
		if runErr != nil {
			t.Errorf("a fluent WARN cycle ships, so the cycle run succeeds; reaching the ship phase and failing there is not a ship: %v", runErr)
		}
	})

	t.Run("strict_blocks", func(t *testing.T) {
		entries, _ := pipelineCycle(t, evolveBin, fakeBin, repoRoot, "e2ewarnstrict",
			"FAKE_CLI_AUDIT_VERDICT=WARN", strictPolicyMarker)
		if ledgerHasRole(entries, "ship") {
			t.Errorf("audit WARN with workflow.strict_audit must be promoted to FAIL and NOT reach the ship phase; ledger roles=%v", ledgerRoles(entries))
		}
		if !ledgerHasRole(entries, "retro") {
			t.Errorf("strict WARN→FAIL must route to retro; ledger roles=%v", ledgerRoles(entries))
		}
	})
}

func TestE2EPipeline_IntentPhase_RunsAndShips(t *testing.T) {
	evolveBin, fakeBin, repoRoot := mustBuildPipelineBins(t)
	entries, runErr := pipelineCycle(t, evolveBin, fakeBin, repoRoot, "e2eintent",
		"EVOLVE_REQUIRE_INTENT=1")

	if !ledgerHasRole(entries, "intent") {
		t.Errorf("EVOLVE_REQUIRE_INTENT=1 should run the intent phase; ledger roles=%v", ledgerRoles(entries))
	}
	if !ledgerHasRole(entries, "ship") {
		t.Errorf("intent-gated happy-path cycle should reach the ship phase; ledger roles=%v", ledgerRoles(entries))
	}
	if runErr != nil {
		t.Errorf("a happy-path cycle ships, so the cycle run succeeds; reaching the ship phase and failing there is not a ship: %v", runErr)
	}
}
