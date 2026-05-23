package subagent

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestValidateProfile_BashParity invokes the bash subagent-run.sh with
// --validate-profile against a fixture agent AND the Go ValidateProfile
// against the same agent, then diffs the resulting EVOLVE_DISPATCH_PLAN_LOG
// JSON files byte-for-byte. Gated by EVOLVE_BASH_PARITY=1 because it
// requires bash + jq + an installed claude.sh adapter; CI without those
// dependencies skips it.
//
// To run locally:
//   EVOLVE_BASH_PARITY=1 go test -run BashParity ./internal/subagent/
//
// The test seeds a fixture profile + LLM config that pin every variable
// to a known value, so the only thing that can differ between bash + Go
// is the actual rendering logic.
func TestValidateProfile_BashParity(t *testing.T) {
	if os.Getenv("EVOLVE_BASH_PARITY") != "1" {
		t.Skip("EVOLVE_BASH_PARITY!=1; skipping bash-vs-Go parity check")
	}
	if runtime.GOOS == "windows" {
		t.Skip("bash parity test requires unix tools (jq, bash)")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not on PATH")
	}
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq not on PATH")
	}

	repoRoot := findRepoRoot(t)
	bashScript := filepath.Join(repoRoot, "legacy", "scripts", "dispatch", "subagent-run.sh")
	if _, err := os.Stat(bashScript); err != nil {
		t.Skipf("bash subagent-run.sh not at %s: %v", bashScript, err)
	}

	// Build the Go binary on demand so the parity comparison runs against
	// HEAD code, not a stale ./bin/evolve.
	goBin := filepath.Join(t.TempDir(), "evolve")
	build := exec.Command("go", "build", "-o", goBin, "./cmd/evolve")
	build.Dir = filepath.Join(repoRoot, "go")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build evolve: %v\n%s", err, out)
	}

	// Seed an isolated fixture so bash + Go see identical files.
	//
	// Capability manifests are read from REAL_ADAPTERS_DIR (script-relative)
	// in bash and from CapabilityDir (plugin install path) in Go — both
	// ignore EVOLVE_ADAPTERS_DIR_OVERRIDE. So we must place the test
	// manifest inside the real legacy/scripts/cli_adapters/ tree to be
	// visible. We add + clean up a single parity-cli.capabilities.json
	// file there for the test's lifetime.
	fixtureRoot := t.TempDir()
	profilesDir := filepath.Join(fixtureRoot, "profiles")
	adaptersDir := filepath.Join(fixtureRoot, "adapters")
	for _, d := range []string{profilesDir, adaptersDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	realAdaptersDir := filepath.Join(repoRoot, "legacy", "scripts", "cli_adapters")

	// Profile: minimal valid agent that routes to our fake-claude adapter.
	profileBody := `{
  "name": "parity",
  "role": "scout",
  "cli": "parity-cli",
  "model_tier_default": "sonnet",
  "output_artifact": ".evolve/runs/cycle-{cycle}/parity.md"
}
`
	if err := os.WriteFile(filepath.Join(profilesDir, "parity.json"), []byte(profileBody), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	// Adapter: trivial stub that exits 0 immediately on VALIDATE_ONLY.
	adapterBody := `#!/usr/bin/env bash
if [ "${VALIDATE_ONLY:-0}" = "1" ]; then
  echo "[parity-adapter] VALIDATE_ONLY=1 — ok" >&2
  exit 0
fi
exit 1
`
	if err := os.WriteFile(filepath.Join(adaptersDir, "parity-cli.sh"), []byte(adapterBody), 0o755); err != nil {
		t.Fatalf("write adapter: %v", err)
	}
	// Capability manifest: degraded budget_cap_native + full permission_scoping
	// gives us a non-trivial WARN + plan-log shape to compare. Placed in the
	// REAL adapters dir because both bash + Go read manifests from there
	// (override-immune by design — protects against test-seam capability lies).
	manifestBody := `{"adapter":"parity-cli","supports":{"budget_cap_native":false,"permission_scoping":true}}`
	realManifest := filepath.Join(realAdaptersDir, "parity-cli.capabilities.json")
	if err := os.WriteFile(realManifest, []byte(manifestBody), 0o644); err != nil {
		t.Fatalf("write real manifest: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(realManifest) })

	// Empty llm_config so both implementations fall back to profile.cli.
	llmConfigPath := filepath.Join(fixtureRoot, "llm_config.json")
	if err := os.WriteFile(llmConfigPath, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write llm_config: %v", err)
	}

	commonEnv := []string{
		"EVOLVE_PROFILES_DIR_OVERRIDE=" + profilesDir,
		"EVOLVE_ADAPTERS_DIR_OVERRIDE=" + adaptersDir,
		"EVOLVE_LLM_CONFIG_PATH=" + llmConfigPath,
		"EVOLVE_PROJECT_ROOT=" + fixtureRoot,
		"EVOLVE_PLUGIN_ROOT=" + repoRoot, // bash uses this to find resolve-llm.sh sibling
	}

	bashLog := filepath.Join(fixtureRoot, "bash-plan.json")
	bashCmd := exec.Command("bash", bashScript, "--validate-profile", "parity")
	bashCmd.Env = append(append([]string{}, os.Environ()...), commonEnv...)
	bashCmd.Env = append(bashCmd.Env, "EVOLVE_DISPATCH_PLAN_LOG="+bashLog)
	bashOut, bashErr := bashCmd.CombinedOutput()
	if bashErr != nil {
		t.Fatalf("bash subagent-run.sh failed: %v\n%s", bashErr, bashOut)
	}

	goLog := filepath.Join(fixtureRoot, "go-plan.json")
	goCmd := exec.Command(goBin, "subagent", "validate-profile", "parity")
	goCmd.Env = append(append([]string{}, os.Environ()...), commonEnv...)
	goCmd.Env = append(goCmd.Env, "EVOLVE_DISPATCH_PLAN_LOG="+goLog)
	goOut, goErr := goCmd.CombinedOutput()
	if goErr != nil {
		t.Fatalf("Go evolve subagent validate-profile failed: %v\n%s", goErr, goOut)
	}

	bashPlan := readPlanJSON(t, bashLog, "bash")
	goPlan := readPlanJSON(t, goLog, "go")

	if !mapsEqual(bashPlan, goPlan) {
		bashJSON, _ := json.MarshalIndent(bashPlan, "", "  ")
		goJSON, _ := json.MarshalIndent(goPlan, "", "  ")
		t.Fatalf("dispatch plan JSON differs:\nbash:\n%s\n\ngo:\n%s", bashJSON, goJSON)
	}

	// Also assert at least one WARN line appears in both stderr captures.
	if !strings.Contains(string(bashOut), "missing=budget_cap_native") {
		t.Errorf("bash stderr missing WARN line:\n%s", bashOut)
	}
	if !strings.Contains(string(goOut), "missing=budget_cap_native") {
		t.Errorf("go stderr missing WARN line:\n%s", goOut)
	}
}

func readPlanJSON(t *testing.T, path, label string) map[string]interface{} {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s plan log at %s: %v", label, path, err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("parse %s plan JSON: %v\nbody: %s", label, err, body)
	}
	return out
}

func mapsEqual(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		bv, ok := b[k]
		if !ok {
			return false
		}
		if !valuesEqual(av, bv) {
			return false
		}
	}
	return true
}

func valuesEqual(a, b interface{}) bool {
	// Slices need element-wise comparison; primitives use ==.
	switch av := a.(type) {
	case []interface{}:
		bv, ok := b.([]interface{})
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !valuesEqual(av[i], bv[i]) {
				return false
			}
		}
		return true
	case map[string]interface{}:
		bv, ok := b.(map[string]interface{})
		if !ok {
			return false
		}
		return mapsEqual(av, bv)
	default:
		return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	// Walk up from the package dir until we find a sibling legacy/ dir.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "legacy", "scripts", "dispatch", "subagent-run.sh")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not locate repo root from %s", dir)
	return ""
}
