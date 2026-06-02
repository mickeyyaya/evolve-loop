package subagent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/capability"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
)

// happyOpts builds a Options blob that lets every step succeed against the
// provided profile body. Tests override one or two seams at a time.
func happyOpts(profileBody string, cli string) ValidateProfileOptions {
	return ValidateProfileOptions{
		ReadProfile: func(string) (string, error) { return profileBody, nil },
		ResolveLLM: func(string) (resolvellm.Result, error) {
			return resolvellm.Result{CLI: cli, ModelTier: "sonnet", Source: "profile"}, nil
		},
		InspectCapability: func(string, string) (capability.Inspection, error) {
			return capability.Inspection{
				Manifest: capability.Manifest{BudgetNative: true, PermissionScoping: true},
			}, nil
		},
		AdapterExists: func(string) bool { return true },
		ExecAdapter: func(context.Context, string, map[string]string) (int, error) {
			return 0, nil
		},
		WriteFile: func(string, []byte, os.FileMode) error { return nil },
	}
}

func TestValidateProfile_HappyPath(t *testing.T) {
	body := `{"role":"scout","cli":"claude","model_tier_default":"sonnet","output_artifact":".evolve/runs/cycle-{cycle}/scout-report.md"}`
	res, err := ValidateProfile(
		context.Background(),
		ValidateProfileRequest{
			Agent:       "scout",
			ProfilesDir: "/p",
			AdaptersDir: "/a",
			ProjectRoot: "/r",
		},
		happyOpts(body, "claude"),
	)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if res.CLI != "claude" {
		t.Errorf("CLI=%q", res.CLI)
	}
	if res.Model != "sonnet" {
		t.Errorf("Model=%q", res.Model)
	}
	if res.CLIResolutionSrc != "profile" {
		t.Errorf("Source=%q", res.CLIResolutionSrc)
	}
	if len(res.Warns) != 0 {
		t.Errorf("expected no warns, got %v", res.Warns)
	}
}

// TestValidateProfile_NilOptionsWireDefaults covers the six `if opts.X == nil`
// default-wiring branches (validateprofile.go:85-102). Passing a zero
// ValidateProfileOptions forces every default to be installed; the call then
// fails at the real defaultReadProfile (missing file) — the point is that the
// default seams are exercised, not that the validation succeeds.
func TestValidateProfile_NilOptionsWireDefaults(t *testing.T) {
	_, err := ValidateProfile(context.Background(), ValidateProfileRequest{
		Agent:       "scout",
		ProfilesDir: filepath.Join(t.TempDir(), "profiles"),
		AdaptersDir: filepath.Join(t.TempDir(), "adapters"),
	}, ValidateProfileOptions{})
	// defaultReadProfile on a nonexistent profile path errors with
	// "profile not found".
	if err == nil || !strings.Contains(err.Error(), "profile not found") {
		t.Errorf("expected profile-not-found through defaults, got %v", err)
	}
}

func TestValidateProfile_MissingAgentName(t *testing.T) {
	_, err := ValidateProfile(context.Background(),
		ValidateProfileRequest{ProfilesDir: "/p", AdaptersDir: "/a"},
		happyOpts(`{}`, "claude"))
	if err == nil || !strings.Contains(err.Error(), "agent required") {
		t.Errorf("expected agent-required error, got %v", err)
	}
}

func TestValidateProfile_MissingProfilesDir(t *testing.T) {
	_, err := ValidateProfile(context.Background(),
		ValidateProfileRequest{Agent: "x", AdaptersDir: "/a"},
		happyOpts(`{}`, "claude"))
	if err == nil || !strings.Contains(err.Error(), "ProfilesDir required") {
		t.Errorf("got %v", err)
	}
}

func TestValidateProfile_MissingAdaptersDir(t *testing.T) {
	_, err := ValidateProfile(context.Background(),
		ValidateProfileRequest{Agent: "x", ProfilesDir: "/p"},
		happyOpts(`{}`, "claude"))
	if err == nil || !strings.Contains(err.Error(), "AdaptersDir required") {
		t.Errorf("got %v", err)
	}
}

func TestValidateProfile_ProfileNotFound(t *testing.T) {
	opts := happyOpts(`{}`, "claude")
	opts.ReadProfile = func(string) (string, error) { return "", os.ErrNotExist }
	_, err := ValidateProfile(context.Background(),
		ValidateProfileRequest{Agent: "x", ProfilesDir: "/p", AdaptersDir: "/a"},
		opts)
	if err == nil || !strings.Contains(err.Error(), "profile not found") {
		t.Errorf("got %v", err)
	}
}

func TestValidateProfile_InvalidJSON(t *testing.T) {
	opts := happyOpts(`{not json`, "claude")
	_, err := ValidateProfile(context.Background(),
		ValidateProfileRequest{Agent: "x", ProfilesDir: "/p", AdaptersDir: "/a"},
		opts)
	if err == nil || !strings.Contains(err.Error(), "not valid JSON") {
		t.Errorf("got %v", err)
	}
}

func TestValidateProfile_LLMResolveFailureFallsThroughToProfile(t *testing.T) {
	body := `{"cli":"claude","model_tier_default":"opus"}`
	opts := happyOpts(body, "claude")
	opts.ResolveLLM = func(string) (resolvellm.Result, error) {
		return resolvellm.Result{}, errors.New("config missing")
	}
	res, err := ValidateProfile(context.Background(),
		ValidateProfileRequest{Agent: "auditor", ProfilesDir: "/p", AdaptersDir: "/a"},
		opts)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if res.CLI != "claude" || res.Model != "opus" || res.CLIResolutionSrc != "profile" {
		t.Errorf("expected fallback: CLI=%q Model=%q Source=%q", res.CLI, res.Model, res.CLIResolutionSrc)
	}
}

func TestValidateProfile_LLMReturnsEmptyCLIFallsThrough(t *testing.T) {
	body := `{"cli":"claude","model_tier_default":"sonnet"}`
	opts := happyOpts(body, "claude")
	opts.ResolveLLM = func(string) (resolvellm.Result, error) {
		return resolvellm.Result{CLI: ""}, nil
	}
	res, err := ValidateProfile(context.Background(),
		ValidateProfileRequest{Agent: "x", ProfilesDir: "/p", AdaptersDir: "/a"},
		opts)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if res.CLIResolutionSrc != "profile" {
		t.Errorf("Source=%q, want profile", res.CLIResolutionSrc)
	}
}

func TestValidateProfile_AntigravityRemappedToAgy(t *testing.T) {
	body := `{"cli":"antigravity","model_tier_default":"sonnet"}`
	opts := happyOpts(body, "antigravity")
	res, err := ValidateProfile(context.Background(),
		ValidateProfileRequest{Agent: "x", ProfilesDir: "/p", AdaptersDir: "/a"},
		opts)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if res.CLI != "agy" {
		t.Errorf("expected agy remap, got %q", res.CLI)
	}
}

func TestValidateProfile_UnresolvedCLI(t *testing.T) {
	body := `{"model_tier_default":"sonnet"}` // no cli field
	opts := happyOpts(body, "")
	opts.ResolveLLM = func(string) (resolvellm.Result, error) {
		return resolvellm.Result{}, errors.New("nope")
	}
	_, err := ValidateProfile(context.Background(),
		ValidateProfileRequest{Agent: "x", ProfilesDir: "/p", AdaptersDir: "/a"},
		opts)
	if err == nil || !strings.Contains(err.Error(), "cli unresolved") {
		t.Errorf("got %v", err)
	}
}

func TestValidateProfile_AdapterMissing(t *testing.T) {
	opts := happyOpts(`{"cli":"claude","model_tier_default":"sonnet"}`, "claude")
	opts.AdapterExists = func(string) bool { return false }
	_, err := ValidateProfile(context.Background(),
		ValidateProfileRequest{Agent: "x", ProfilesDir: "/p", AdaptersDir: "/a"},
		opts)
	if err == nil || !strings.Contains(err.Error(), "adapter not executable") {
		t.Errorf("got %v", err)
	}
}

func TestValidateProfile_DegradedCapabilityEmitsWarns(t *testing.T) {
	opts := happyOpts(`{"cli":"agy","model_tier_default":"sonnet"}`, "agy")
	opts.InspectCapability = func(string, string) (capability.Inspection, error) {
		return capability.Inspection{
			Manifest: capability.Manifest{BudgetNative: false, PermissionScoping: true},
			Warns:    []string{"[adapter-cap] WARN cli=agy missing=budget_cap_native substitute=wall_clock_timeout"},
		}, nil
	}
	res, err := ValidateProfile(context.Background(),
		ValidateProfileRequest{Agent: "x", ProfilesDir: "/p", AdaptersDir: "/a"},
		opts)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(res.Warns) != 1 {
		t.Errorf("expected 1 warn, got %v", res.Warns)
	}
}

func TestValidateProfile_CapabilityInspectError(t *testing.T) {
	opts := happyOpts(`{"cli":"x","model_tier_default":"sonnet"}`, "x")
	opts.InspectCapability = func(string, string) (capability.Inspection, error) {
		return capability.Inspection{}, errors.New("disk gone")
	}
	_, err := ValidateProfile(context.Background(),
		ValidateProfileRequest{Agent: "x", ProfilesDir: "/p", AdaptersDir: "/a"},
		opts)
	if err == nil || !strings.Contains(err.Error(), "capability inspect") {
		t.Errorf("got %v", err)
	}
}

func TestValidateProfile_DispatchPlanLogWritten(t *testing.T) {
	tmp := t.TempDir()
	logPath := filepath.Join(tmp, "plan.json")
	opts := happyOpts(`{"cli":"claude","model_tier_default":"opus"}`, "claude")
	_, err := ValidateProfile(context.Background(),
		ValidateProfileRequest{
			Agent: "x", ProfilesDir: "/p", AdaptersDir: "/a",
			DispatchPlanLog: logPath,
		},
		ValidateProfileOptions{
			ReadProfile:       opts.ReadProfile,
			ResolveLLM:        opts.ResolveLLM,
			InspectCapability: opts.InspectCapability,
			AdapterExists:     opts.AdapterExists,
			ExecAdapter:       opts.ExecAdapter,
			WriteFile:         os.WriteFile, // real write
		})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	body, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read plan log: %v", err)
	}
	if !strings.Contains(string(body), `"cli":"claude"`) {
		t.Errorf("plan log missing cli: %s", body)
	}
	if !strings.Contains(string(body), `"cap_budget_native":true`) {
		t.Errorf("plan log missing budget flag: %s", body)
	}
	if !strings.HasSuffix(string(body), "\n") {
		t.Errorf("plan log missing trailing newline")
	}
}

func TestValidateProfile_DispatchPlanLogWriteError(t *testing.T) {
	opts := happyOpts(`{"cli":"claude","model_tier_default":"opus"}`, "claude")
	opts.WriteFile = func(string, []byte, os.FileMode) error { return errors.New("disk full") }
	_, err := ValidateProfile(context.Background(),
		ValidateProfileRequest{
			Agent: "x", ProfilesDir: "/p", AdaptersDir: "/a",
			DispatchPlanLog: "/anywhere/plan.json",
		},
		opts)
	if err == nil || !strings.Contains(err.Error(), "write dispatch plan log") {
		t.Errorf("got %v", err)
	}
}

func TestValidateProfile_AdapterOverridesExtracted(t *testing.T) {
	body := `{
		"cli":"claude",
		"model_tier_default":"sonnet",
		"adapter_overrides":{
			"claude":{
				"tools":["Read","Write"],
				"extra_flags":["--max-budget-usd","2.00"]
			}
		}
	}`
	opts := happyOpts(body, "claude")
	captured := map[string]string{}
	opts.ExecAdapter = func(_ context.Context, _ string, env map[string]string) (int, error) {
		for k, v := range env {
			captured[k] = v
		}
		return 0, nil
	}
	res, err := ValidateProfile(context.Background(),
		ValidateProfileRequest{Agent: "x", ProfilesDir: "/p", AdaptersDir: "/a"},
		opts)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if res.AdapterOverrides.ToolsJSON != `["Read","Write"]` {
		t.Errorf("tools=%q", res.AdapterOverrides.ToolsJSON)
	}
	if res.AdapterOverrides.ExtraFlagsJSON != `["--max-budget-usd","2.00"]` {
		t.Errorf("extra_flags=%q", res.AdapterOverrides.ExtraFlagsJSON)
	}
	if captured["ADAPTER_TOOLS_OVERRIDE"] == "" {
		t.Errorf("env var not set")
	}
	if captured["VALIDATE_ONLY"] != "1" {
		t.Errorf("VALIDATE_ONLY=%q", captured["VALIDATE_ONLY"])
	}
}

func TestValidateProfile_AdapterOverridesAbsent(t *testing.T) {
	body := `{"cli":"claude","model_tier_default":"sonnet"}`
	opts := happyOpts(body, "claude")
	res, err := ValidateProfile(context.Background(),
		ValidateProfileRequest{Agent: "x", ProfilesDir: "/p", AdaptersDir: "/a"},
		opts)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if res.AdapterOverrides.ToolsJSON != "" {
		t.Errorf("expected empty tools, got %q", res.AdapterOverrides.ToolsJSON)
	}
}

func TestValidateProfile_AdapterOverridesOnlyForResolvedCLI(t *testing.T) {
	// Profile has overrides for "claude" but resolved cli is "agy" — must not match.
	body := `{"cli":"agy","adapter_overrides":{"claude":{"tools":["X"]}}}`
	opts := happyOpts(body, "agy")
	res, err := ValidateProfile(context.Background(),
		ValidateProfileRequest{Agent: "x", ProfilesDir: "/p", AdaptersDir: "/a"},
		opts)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if res.AdapterOverrides.ToolsJSON != "" {
		t.Errorf("expected empty (cli=agy, overrides=claude), got %q", res.AdapterOverrides.ToolsJSON)
	}
}

func TestValidateProfile_AdapterExecNonZeroExitFails(t *testing.T) {
	opts := happyOpts(`{"cli":"claude","model_tier_default":"sonnet"}`, "claude")
	opts.ExecAdapter = func(context.Context, string, map[string]string) (int, error) {
		return 7, nil
	}
	res, err := ValidateProfile(context.Background(),
		ValidateProfileRequest{Agent: "x", ProfilesDir: "/p", AdaptersDir: "/a"},
		opts)
	if err == nil || !strings.Contains(err.Error(), "returned non-zero: 7") {
		t.Errorf("got %v", err)
	}
	if res.AdapterExitCode != 7 {
		t.Errorf("AdapterExitCode=%d", res.AdapterExitCode)
	}
}

func TestValidateProfile_AdapterExecRuntimeErrorFails(t *testing.T) {
	opts := happyOpts(`{"cli":"claude","model_tier_default":"sonnet"}`, "claude")
	opts.ExecAdapter = func(context.Context, string, map[string]string) (int, error) {
		return -1, errors.New("bash missing")
	}
	_, err := ValidateProfile(context.Background(),
		ValidateProfileRequest{Agent: "x", ProfilesDir: "/p", AdaptersDir: "/a"},
		opts)
	if err == nil || !strings.Contains(err.Error(), "adapter exec") {
		t.Errorf("got %v", err)
	}
}

func TestValidateProfile_WorktreePathOverride(t *testing.T) {
	body := `{"cli":"claude","model_tier_default":"sonnet"}`
	opts := happyOpts(body, "claude")
	var captured string
	opts.ExecAdapter = func(_ context.Context, _ string, env map[string]string) (int, error) {
		captured = env["WORKTREE_PATH"]
		return 0, nil
	}
	_, _ = ValidateProfile(context.Background(),
		ValidateProfileRequest{
			Agent: "x", ProfilesDir: "/p", AdaptersDir: "/a",
			ProjectRoot:  "/root",
			WorktreePath: "/cycle-wt",
		},
		opts)
	if captured != "/cycle-wt" {
		t.Errorf("WORKTREE_PATH=%q, want /cycle-wt", captured)
	}
}

func TestValidateProfile_WorktreePathDefaultsToProjectRoot(t *testing.T) {
	body := `{"cli":"claude","model_tier_default":"sonnet"}`
	opts := happyOpts(body, "claude")
	var captured string
	opts.ExecAdapter = func(_ context.Context, _ string, env map[string]string) (int, error) {
		captured = env["WORKTREE_PATH"]
		return 0, nil
	}
	_, _ = ValidateProfile(context.Background(),
		ValidateProfileRequest{
			Agent: "x", ProfilesDir: "/p", AdaptersDir: "/a",
			ProjectRoot: "/root",
		},
		opts)
	if captured != "/root" {
		t.Errorf("WORKTREE_PATH=%q, want /root fallback", captured)
	}
}

func TestExtractAdapterOverrides_NestedArrayPreserved(t *testing.T) {
	body := `{"adapter_overrides":{"claude":{"tools":["A","B"],"extra_flags":["--x","y"]}}}`
	out := extractAdapterOverrides(body, "claude")
	if out.ToolsJSON != `["A","B"]` {
		t.Errorf("tools=%q", out.ToolsJSON)
	}
	if out.ExtraFlagsJSON != `["--x","y"]` {
		t.Errorf("extra_flags=%q", out.ExtraFlagsJSON)
	}
}

func TestExtractAdapterOverrides_NoOverridesBlock(t *testing.T) {
	out := extractAdapterOverrides(`{"role":"x"}`, "claude")
	if out.ToolsJSON != "" || out.ExtraFlagsJSON != "" {
		t.Errorf("expected empty, got %+v", out)
	}
}

func TestExtractAdapterOverrides_CliMismatchInBlock(t *testing.T) {
	out := extractAdapterOverrides(`{"adapter_overrides":{"agy":{"tools":["X"]}}}`, "claude")
	if out.ToolsJSON != "" {
		t.Errorf("expected empty for cli mismatch, got %q", out.ToolsJSON)
	}
}

func TestCapabilityExtractObject_NotPresent(t *testing.T) {
	if _, ok := capabilityExtractObject(`{}`, "x"); ok {
		t.Errorf("expected false")
	}
	if _, ok := capabilityExtractObject(`{"x":"str"}`, "x"); ok {
		t.Errorf("string value should not match")
	}
	if _, ok := capabilityExtractObject(`{"x":  `, "x"); ok {
		t.Errorf("truncated should not match")
	}
}

// TestCapabilityExtractObject_KeyWithoutColon covers the branch where the
// key is found but the next non-space rune is not ':' — the parser must
// reject rather than mis-read the following token as a value.
func TestCapabilityExtractObject_KeyWithoutColon(t *testing.T) {
	if v, ok := capabilityExtractObject(`{"x" 5}`, "x"); ok {
		t.Errorf("key without colon should not match, got %q", v)
	}
}

// TestCapabilityExtractObject_UnterminatedObject covers the fall-through
// return when an opening brace is found but never balanced.
func TestCapabilityExtractObject_UnterminatedObject(t *testing.T) {
	if v, ok := capabilityExtractObject(`{"x":{"inner":1`, "x"); ok {
		t.Errorf("unterminated object should not match, got %q", v)
	}
}

// TestDefaultResolveLLM_BridgesToResolver exercises the defaultResolveLLM
// seam directly (0% before); the point is to execute the bridge line itself.
func TestDefaultResolveLLM_BridgesToResolver(t *testing.T) {
	// The call must not panic and must return through the resolver. We assert
	// only that the bridge returns the resolver's own result/error pair —
	// either a populated CLI or a non-nil error, never both empty+nil.
	res, err := defaultResolveLLM("scout")
	if err == nil && res.CLI == "" {
		t.Errorf("bridge returned empty result with nil error: %+v", res)
	}
}

func TestCapBoolEnv(t *testing.T) {
	if capBoolEnv(true) != "true" || capBoolEnv(false) != "false" {
		t.Errorf("capBoolEnv broken")
	}
}

func TestDefaultAdapterExists_RealFilesystem(t *testing.T) {
	tmp := t.TempDir()
	missing := filepath.Join(tmp, "nope.sh")
	if defaultAdapterExists(missing) {
		t.Errorf("expected false for missing")
	}
	notExec := filepath.Join(tmp, "not-exec.sh")
	if err := os.WriteFile(notExec, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if defaultAdapterExists(notExec) {
		t.Errorf("expected false for non-executable")
	}
	exec := filepath.Join(tmp, "exec.sh")
	if err := os.WriteFile(exec, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if !defaultAdapterExists(exec) {
		t.Errorf("expected true for executable")
	}
	if defaultAdapterExists(tmp) {
		t.Errorf("expected false for directory")
	}
}

func TestDefaultExecAdapter_RealBash(t *testing.T) {
	tmp := t.TempDir()
	script := filepath.Join(tmp, "ok.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("seed: %v", err)
	}
	rc, err := defaultExecAdapter(context.Background(), script, map[string]string{"K": "V"})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if rc != 0 {
		t.Errorf("rc=%d", rc)
	}

	failScript := filepath.Join(tmp, "fail.sh")
	if err := os.WriteFile(failScript, []byte("#!/bin/sh\nexit 3\n"), 0o755); err != nil {
		t.Fatalf("seed: %v", err)
	}
	rc, err = defaultExecAdapter(context.Background(), failScript, nil)
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if rc != 3 {
		t.Errorf("rc=%d, want 3", rc)
	}
}

func TestDefaultExecAdapter_MissingBinaryReturnsError(t *testing.T) {
	// Force a non-exit error by pointing at a non-existent shell.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel — exec.CommandContext will see context.Canceled
	rc, err := defaultExecAdapter(ctx, "/non/existent/script.sh", nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	if rc != -1 {
		t.Errorf("rc=%d, want -1", rc)
	}
}
