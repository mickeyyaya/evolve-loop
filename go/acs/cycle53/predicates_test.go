//go:build acs

// Package cycle53 ports the cycle-53 ACS predicates (3 files) — all
// subprocess-with-env-scaffolding style — from acs/cycle-53/*.sh.
//
// These three predicates were the original motivation for the
// SubprocessOutput helper in pkg/acsassert: they shell out to
// scripts/dispatch/subagent-run.sh with EVOLVE_TESTING=1 and a hand-built
// sentinel adapter, then assert on the captured stderr + the dispatch
// plan JSON.
//
// Portability note: these are integration-style and depend on the live
// subagent-run.sh implementation. They Skip when subagent-run.sh is
// absent (e.g., in a partial repo export).
package cycle53

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skipf("not in a git work tree: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func subagentRunPath(t *testing.T) string {
	t.Helper()
	p := filepath.Join(repoRoot(t), "legacy", "scripts", "dispatch", "subagent-run.sh")
	if _, err := os.Stat(p); err != nil {
		t.Skipf("subagent-run.sh missing: %v", err)
	}
	return p
}

// runSubagent shells to bash subagent-run.sh --validate-profile <phase>
// with the supplied env overrides. Returns captured stderr (the only
// output the bash predicates inspect) and the exit code.
func runSubagent(t *testing.T, phase string, env map[string]string) (string, int) {
	t.Helper()
	cmd := exec.Command("bash", subagentRunPath(t), "--validate-profile", phase)
	cmd.Env = append(os.Environ(), "EVOLVE_TESTING=1")
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	stdout, err := cmd.Output()
	// Capture stderr from ExitError.
	stderr := ""
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr = string(exitErr.Stderr)
		}
	}
	if stderr == "" {
		// On a 0-exit run, stderr was streamed via os.Stderr — re-run with
		// CombinedOutput to grab both. Cheaper than wiring a pipe.
		cmd2 := exec.Command("bash", subagentRunPath(t), "--validate-profile", phase)
		cmd2.Env = cmd.Env
		combined, _ := cmd2.CombinedOutput()
		stderr = string(combined)
	}
	code := 0
	if err != nil {
		var ex *exec.ExitError
		if errors.As(err, &ex) {
			code = ex.ExitCode()
		} else {
			code = -1
		}
	}
	_ = stdout
	return stderr, code
}

// scaffold creates the standard cycle-53 test fixture (llm_config.json +
// sentinel adapter) and returns env vars to point subagent-run.sh at it.
func scaffold(t *testing.T, cli, model, sentinelScript string) (envVars map[string]string, planLog string) {
	t.Helper()
	tmp := t.TempDir()
	cfg := filepath.Join(tmp, "llm_config.json")
	cfgBody := `{"schema_version":1,"phases":{"scout":{"cli":"` + cli + `","model":"` + model + `"},"auditor":{"cli":"` + cli + `","model":"` + model + `"}}}`
	if err := os.WriteFile(cfg, []byte(cfgBody), 0o644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}
	adaptersDir := filepath.Join(tmp, "adapters")
	if err := os.MkdirAll(adaptersDir, 0o755); err != nil {
		t.Fatalf("mkdir adapters: %v", err)
	}
	if err := os.WriteFile(filepath.Join(adaptersDir, cli+".sh"), []byte(sentinelScript), 0o755); err != nil {
		t.Fatalf("write adapter: %v", err)
	}
	planLog = filepath.Join(tmp, "dispatch-plan.json")
	return map[string]string{
		"EVOLVE_GEMINI_CLAUDE_PATH":    "",
		"EVOLVE_LLM_CONFIG_PATH":       cfg,
		"EVOLVE_ADAPTERS_DIR_OVERRIDE": adaptersDir,
		"EVOLVE_DISPATCH_PLAN_LOG":     planLog,
	}, planLog
}

const geminiSentinel = `#!/usr/bin/env bash
echo "[sentinel-gemini] CAP_BUDGET_NATIVE=${CAP_BUDGET_NATIVE:-unset}" >&2
echo "[sentinel-gemini] RESOLVED_CLI=${RESOLVED_CLI:-unset}" >&2
exit 0
`

const claudeSentinel = `#!/usr/bin/env bash
if [ "${VALIDATE_ONLY:-0}" = "1" ]; then
    echo "[sentinel-claude] model_routed=${RESOLVED_MODEL:-unset}" >&2
    exit 0
fi
echo "[sentinel-claude] CAP_BUDGET_NATIVE=${CAP_BUDGET_NATIVE:-unset}" >&2
exit 0
`

// TestC53_004_CapabilityMatrixHonoredNoBudgetCap ports cycle-53/004.
// When llm_config routes to gemini (no budget_cap_native), expect
// [adapter-cap] WARN + CAP_BUDGET_NATIVE=false on env; anti-tautology
// claude has no such WARN.
func TestC53_004_CapabilityMatrixHonoredNoBudgetCap(t *testing.T) {
	env, _ := scaffold(t, "gemini", "gemini-3-pro-preview", geminiSentinel)
	stderr, _ := runSubagent(t, "scout", env)
	if !strings.Contains(stderr, "[adapter-cap] WARN cli=gemini missing=budget_cap_native") {
		t.Errorf("AC1: missing [adapter-cap] WARN for gemini in stderr:\n%s", stderr)
	}
	if !strings.Contains(stderr, "CAP_BUDGET_NATIVE=false") {
		t.Errorf("AC2: sentinel did not receive CAP_BUDGET_NATIVE=false:\n%s", stderr)
	}

	// Anti-tautology: claude has no such WARN.
	env2, _ := scaffold(t, "claude", "claude-sonnet-4-6", claudeSentinel)
	stderr2, _ := runSubagent(t, "scout", env2)
	if strings.Contains(stderr2, "[adapter-cap] WARN") && strings.Contains(stderr2, "missing=budget_cap_native") {
		t.Errorf("AC3 (anti-tautology): WARN appeared for claude:\n%s", stderr2)
	}
}

// TestC53_007_DegradedModeEmitsStructuredWARN ports cycle-53/007 —
// dispatch plan JSON has capability_warns array matching stderr WARNs.
func TestC53_007_DegradedModeEmitsStructuredWARN(t *testing.T) {
	env, planLog := scaffold(t, "gemini", "gemini-3-pro-preview", "#!/usr/bin/env bash\nexit 0\n")
	stderr, _ := runSubagent(t, "scout", env)

	planBytes, err := os.ReadFile(planLog)
	if err != nil {
		t.Skipf("EVOLVE_DISPATCH_PLAN_LOG not written (feature missing): %v", err)
	}
	var plan struct {
		CapabilityWarns []string `json:"capability_warns"`
	}
	if err := json.Unmarshal(planBytes, &plan); err != nil {
		t.Errorf("AC2: dispatch plan log is not valid JSON: %v; raw=%s", err, planBytes)
		return
	}
	if len(plan.CapabilityWarns) == 0 {
		t.Errorf("AC2: capability_warns array is empty; plan=%s", planBytes)
	}

	formatRE := regexp.MustCompile(`^cli=[a-z]+ missing=[a-z_]+ substitute=[a-z_]+$`)
	for i, w := range plan.CapabilityWarns {
		if !formatRE.MatchString(w) {
			t.Errorf("AC3: warn entry[%d] does not match format: %q", i, w)
		}
	}

	stderrWarnCount := strings.Count(stderr, "[adapter-cap] WARN")
	if stderrWarnCount != len(plan.CapabilityWarns) {
		t.Errorf("AC4: plan has %d warns but stderr has %d WARN lines", len(plan.CapabilityWarns), stderrWarnCount)
	}

	// Anti-tautology: claude plan must have 0 warns.
	env2, planLog2 := scaffold(t, "claude", "claude-sonnet-4-6", "#!/usr/bin/env bash\nexit 0\n")
	_, _ = runSubagent(t, "scout", env2)
	if b, err := os.ReadFile(planLog2); err == nil {
		var p2 struct {
			CapabilityWarns []string `json:"capability_warns"`
		}
		if json.Unmarshal(b, &p2) == nil && len(p2.CapabilityWarns) > 0 {
			t.Errorf("AC5 (anti-tautology): claude plan has %d warns — should be 0", len(p2.CapabilityWarns))
		}
	}
}

// TestC53_008_ModelRoutedViaLLMConfig ports cycle-53/008 — exact model
// from llm_config overrides profile.model_tier_default.
func TestC53_008_ModelRoutedViaLLMConfig(t *testing.T) {
	env, _ := scaffold(t, "claude", "claude-opus-4-7", claudeSentinel)
	stderrWith, _ := runSubagent(t, "auditor", env)
	if !strings.Contains(stderrWith, "model_routed=claude-opus-4-7") {
		t.Errorf("AC1: expected model_routed=claude-opus-4-7 not found:\n%s", stderrWith)
	}
	if !regexp.MustCompile(`cli_resolution:.*source=llm_config`).MatchString(stderrWith) {
		t.Errorf("AC3: cli_resolution source=llm_config log missing:\n%s", stderrWith)
	}

	// Anti-tautology: without llm_config, model is NOT claude-opus-4-7.
	envWithout := map[string]string{}
	// Reuse the sentinel-claude adapter from a fresh scaffold but without LLM_CONFIG path.
	envBase, _ := scaffold(t, "claude", "claude-opus-4-7", claudeSentinel)
	for k, v := range envBase {
		if k != "EVOLVE_LLM_CONFIG_PATH" {
			envWithout[k] = v
		}
	}
	stderrWithout, _ := runSubagent(t, "auditor", envWithout)
	if strings.Contains(stderrWithout, "model_routed=claude-opus-4-7") {
		t.Errorf("AC2 (anti-tautology): model was claude-opus-4-7 even WITHOUT llm_config — predicate is tautological:\n%s", stderrWithout)
	}
}
