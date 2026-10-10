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
	realScripts := filepath.Join(repoRoot, "legacy", "scripts")
	if _, err := os.Stat(filepath.Join(realScripts, "dispatch", "subagent-run.sh")); err != nil {
		t.Skipf("bash subagent-run.sh not under %s: %v", realScripts, err)
	}
	pluginRoot := t.TempDir()
	scripts := filepath.Join(pluginRoot, "legacy", "scripts")
	if err := os.CopyFS(scripts, os.DirFS(realScripts)); err != nil {
		t.Fatalf("mirror %s: %v", realScripts, err)
	}
	bashScript := filepath.Join(scripts, "dispatch", "subagent-run.sh")

	goBin := filepath.Join(t.TempDir(), "evolve")
	build := exec.Command("go", "build", "-o", goBin, "./cmd/evolve")
	build.Dir = filepath.Join(repoRoot, "go")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build evolve: %v\n%s", err, out)
	}

	fixtureRoot := t.TempDir()
	profilesDir := filepath.Join(fixtureRoot, "profiles")
	adaptersDir := filepath.Join(fixtureRoot, "adapters")
	for _, d := range []string{profilesDir, adaptersDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	scriptAdaptersDir := filepath.Join(scripts, "cli_adapters")

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
	manifestBody := `{"adapter":"parity-cli","supports":{"budget_cap_native":false,"permission_scoping":true}}`
	if err := os.WriteFile(filepath.Join(scriptAdaptersDir, "parity-cli.capabilities.json"), []byte(manifestBody), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	commonEnv := []string{
		"EVOLVE_PROFILES_DIR_OVERRIDE=" + profilesDir,
		"EVOLVE_ADAPTERS_DIR_OVERRIDE=" + adaptersDir,
		"EVOLVE_PROJECT_ROOT=" + fixtureRoot,
		"EVOLVE_PLUGIN_ROOT=" + pluginRoot,
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
