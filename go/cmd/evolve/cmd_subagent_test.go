package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCLI(args ...string) (stdout, stderr string, rc int) {
	var outBuf, errBuf bytes.Buffer
	rc = runSubagent(args, nil, &outBuf, &errBuf)
	return outBuf.String(), errBuf.String(), rc
}

func TestRunSubagent_NoArgsPrintsUsage(t *testing.T) {
	_, errOut, rc := runCLI()
	if rc != 2 {
		t.Errorf("rc=%d, want 2", rc)
	}
	if !strings.Contains(errOut, "Subcommands:") {
		t.Errorf("usage not in stderr: %q", errOut)
	}
}

func TestRunSubagent_HelpExitsZero(t *testing.T) {
	for _, h := range []string{"--help", "-h", "help"} {
		out, _, rc := runCLI(h)
		if rc != 0 {
			t.Errorf("%s: rc=%d", h, rc)
		}
		if !strings.Contains(out, "Subcommands:") {
			t.Errorf("%s: missing usage in stdout", h)
		}
	}
}

func TestRunSubagent_UnknownSubcommand(t *testing.T) {
	_, errOut, rc := runCLI("nope")
	if rc != 2 {
		t.Errorf("rc=%d, want 2", rc)
	}
	if !strings.Contains(errOut, "unknown subcommand") {
		t.Errorf("stderr: %q", errOut)
	}
}

func TestRunSubagent_CheckToken_OKAndFail(t *testing.T) {
	tmp := t.TempDir()
	artifact := filepath.Join(tmp, "a.md")
	if err := os.WriteFile(artifact, []byte("body\n<!-- challenge-token: 1234567890abcdef -->\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, errOut, rc := runCLI("check-token", artifact, "1234567890abcdef")
	if rc != 0 {
		t.Errorf("OK case rc=%d, want 0", rc)
	}
	if !strings.Contains(errOut, "OK:") {
		t.Errorf("stderr: %q", errOut)
	}
	_, errOut, rc = runCLI("check-token", artifact, "missing")
	if rc != 2 {
		t.Errorf("fail case rc=%d, want 2", rc)
	}
	if !strings.Contains(errOut, "INTEGRITY-FAIL") {
		t.Errorf("stderr: %q", errOut)
	}
}

func TestRunSubagent_CheckToken_ArgMismatch(t *testing.T) {
	_, errOut, rc := runCLI("check-token", "only-one-arg")
	if rc != 2 {
		t.Errorf("rc=%d", rc)
	}
	if !strings.Contains(errOut, "expected") {
		t.Errorf("stderr: %q", errOut)
	}
	out, _, rc := runCLI("check-token", "-h")
	if rc != 0 || !strings.Contains(out, "Usage") {
		t.Errorf("help broken: rc=%d out=%q", rc, out)
	}
}

func TestRunSubagent_CheckCtxAdvisory_EmitAndSuppress(t *testing.T) {
	tmp := t.TempDir()
	profile := filepath.Join(tmp, "p.json")
	if err := os.WriteFile(profile, []byte(`{"context_clear_trigger_tokens":1000}`), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, errOut, rc := runCLI("check-ctx-advisory", profile, "5000")
	if rc != 0 {
		t.Errorf("rc=%d (advisory always exits 0)", rc)
	}
	if !strings.Contains(errOut, "INFO:") || !strings.Contains(errOut, "Tool-Result Hygiene") {
		t.Errorf("advisory not emitted: %q", errOut)
	}
	_, errOut, rc = runCLI("check-ctx-advisory", profile, "500")
	if rc != 0 {
		t.Errorf("rc=%d", rc)
	}
	if strings.Contains(errOut, "INFO:") {
		t.Errorf("advisory should suppress at low token count: %q", errOut)
	}
}

func TestRunSubagent_CheckCtxAdvisory_MissingProfileWarnExitZero(t *testing.T) {
	_, errOut, rc := runCLI("check-ctx-advisory", "/nope.json", "100")
	if rc != 0 {
		t.Errorf("rc=%d, want 0 (bash semantics)", rc)
	}
	if !strings.Contains(errOut, "WARN") {
		t.Errorf("expected WARN: %q", errOut)
	}
}

func TestRunSubagent_CheckCtxAdvisory_ArgErrors(t *testing.T) {
	_, errOut, rc := runCLI("check-ctx-advisory", "only-one")
	if rc != 2 {
		t.Errorf("rc=%d", rc)
	}
	if !strings.Contains(errOut, "expected") {
		t.Errorf("stderr: %q", errOut)
	}
	_, errOut, rc = runCLI("check-ctx-advisory", "/p.json", "notanumber")
	if rc != 2 {
		t.Errorf("rc=%d", rc)
	}
	if !strings.Contains(errOut, "tokens must be int") {
		t.Errorf("stderr: %q", errOut)
	}
	out, _, rc := runCLI("check-ctx-advisory", "-h")
	if rc != 0 || !strings.Contains(out, "Usage") {
		t.Errorf("help broken: rc=%d out=%q", rc, out)
	}
}

func TestRunSubagent_CachePrefix_Smoke(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "out.md")
	_, errOut, rc := runCLI(
		"cache-prefix",
		"--cycle", "9",
		"--agent", "scout",
		"--workspace", tmp,
		"--out", out,
		"--project-root", tmp,
	)
	if rc != 0 {
		t.Fatalf("rc=%d errOut=%q", rc, errOut)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("output not written: %v", err)
	}
	if !strings.Contains(string(body), "agent=scout cycle=9") {
		t.Errorf("metadata not propagated: %s", body)
	}
}

func TestRunSubagent_CachePrefix_MissingRequiredArgs(t *testing.T) {
	_, errOut, rc := runCLI("cache-prefix", "--agent", "x")
	if rc != 2 {
		t.Errorf("rc=%d", rc)
	}
	if !strings.Contains(errOut, "required") {
		t.Errorf("stderr: %q", errOut)
	}
}

func TestRunSubagent_CachePrefix_CycleParseError(t *testing.T) {
	_, errOut, rc := runCLI(
		"cache-prefix",
		"--cycle", "not-an-int",
		"--agent", "x", "--workspace", "/w", "--out", "/o.md",
	)
	if rc != 2 {
		t.Errorf("rc=%d", rc)
	}
	if !strings.Contains(errOut, "--cycle must be int") {
		t.Errorf("stderr: %q", errOut)
	}
}

func TestRunSubagent_CachePrefix_UnknownArg(t *testing.T) {
	_, errOut, rc := runCLI("cache-prefix", "--bogus", "1")
	if rc != 2 {
		t.Errorf("rc=%d", rc)
	}
	if !strings.Contains(errOut, "unknown arg") {
		t.Errorf("stderr: %q", errOut)
	}
}

func TestRunSubagent_CachePrefix_Help(t *testing.T) {
	out, _, rc := runCLI("cache-prefix", "--help")
	if rc != 0 || !strings.Contains(out, "Usage") {
		t.Errorf("help broken")
	}
}

func TestRunSubagent_ResolveTier_Smoke(t *testing.T) {
	tmp := t.TempDir()
	profile := filepath.Join(tmp, "scout.json")
	if err := os.WriteFile(profile, []byte(`{"role":"scout","model_tier_default":"sonnet"}`), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	out, _, rc := runCLI(
		"resolve-tier",
		"--profile", profile,
		"--cycle", "1",
		"--project-root", tmp,
	)
	if rc != 0 {
		t.Errorf("rc=%d", rc)
	}
	if strings.TrimSpace(out) != "sonnet" {
		t.Errorf("got %q, want sonnet", out)
	}
}

func TestRunSubagent_ResolveTier_ErrorPaths(t *testing.T) {
	_, errOut, rc := runCLI("resolve-tier")
	if rc != 2 || !strings.Contains(errOut, "required") {
		t.Errorf("missing args: rc=%d err=%q", rc, errOut)
	}
	_, errOut, rc = runCLI("resolve-tier", "--bogus", "x")
	if rc != 2 || !strings.Contains(errOut, "unknown arg") {
		t.Errorf("unknown arg: rc=%d err=%q", rc, errOut)
	}
	_, errOut, rc = runCLI(
		"resolve-tier", "--profile", "/p", "--cycle", "not-int",
	)
	if rc != 2 || !strings.Contains(errOut, "--cycle must be int") {
		t.Errorf("cycle parse: rc=%d err=%q", rc, errOut)
	}
	_, errOut, rc = runCLI(
		"resolve-tier", "--profile", "/missing.json", "--cycle", "1",
	)
	if rc != 1 {
		t.Errorf("read err: rc=%d err=%q", rc, errOut)
	}
	out, _, rc := runCLI("resolve-tier", "-h")
	if rc != 0 || !strings.Contains(out, "Usage") {
		t.Errorf("help broken")
	}
}

func TestRunSubagent_ValidateProfile_ArgMismatch(t *testing.T) {
	_, errOut, rc := runCLI("validate-profile")
	if rc != 2 {
		t.Errorf("rc=%d, want 2", rc)
	}
	if !strings.Contains(errOut, "expected <agent>") {
		t.Errorf("stderr: %q", errOut)
	}
}

func TestRunSubagent_ValidateProfile_Help(t *testing.T) {
	out, _, rc := runCLI("validate-profile", "-h")
	if rc != 0 {
		t.Errorf("rc=%d", rc)
	}
	if !strings.Contains(out, "Usage") || !strings.Contains(out, "EVOLVE_PROFILES_DIR_OVERRIDE") {
		t.Errorf("help missing env vars: %q", out)
	}
}

func TestRunSubagent_ValidateProfile_MissingProfileFails(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("EVOLVE_PROFILES_DIR_OVERRIDE", tmp)
	t.Setenv("EVOLVE_ADAPTERS_DIR_OVERRIDE", tmp)
	t.Setenv("EVOLVE_DISPATCH_PLAN_LOG", "")
	t.Setenv("EVOLVE_LLM_CONFIG_PATH", filepath.Join(tmp, "absent.json"))
	t.Setenv("EVOLVE_PROJECT_ROOT", tmp)
	t.Setenv("EVOLVE_PLUGIN_ROOT", tmp)
	_, errOut, rc := runCLI("validate-profile", "no-such-agent")
	if rc != 1 {
		t.Errorf("rc=%d, want 1", rc)
	}
	if !strings.Contains(errOut, "FAIL:") || !strings.Contains(errOut, "profile not found") {
		t.Errorf("stderr: %q", errOut)
	}
}

func TestEnvOrCwd_FallsBackToCwd(t *testing.T) {
	t.Setenv("X_NEVER_SET_99", "")
	got := envOrCwd("X_NEVER_SET_99")
	if got == "" {
		t.Errorf("expected fallback path, got empty")
	}
}

func TestNextArgEmptyWhenOutOfRange(t *testing.T) {
	if nextArg([]string{"a"}, 5) != "" {
		t.Errorf("nextArg should return \"\" past end")
	}
}
