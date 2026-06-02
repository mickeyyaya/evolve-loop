package subagent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/capability"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
)

// ValidateProfileRequest captures every input cmd_validate_profile reads
// from argv + environment. ProfilesDir + AdaptersDir mirror the bash
// EVOLVE_PROFILES_DIR_OVERRIDE + EVOLVE_ADAPTERS_DIR_OVERRIDE knobs.
//
// CapabilityDir is intentionally separate from AdaptersDir: bash uses
// REAL_ADAPTERS_DIR (script-relative, override-immune) for capability
// manifest lookups so test seams can't lie about installed capabilities.
// Go callers SHOULD set CapabilityDir to the real plugin install path —
// the CLI does this automatically. When CapabilityDir is empty, it
// defaults to AdaptersDir so unit tests have one less knob to set.
type ValidateProfileRequest struct {
	Agent           string
	ProfilesDir     string // immutable plugin profiles dir (.evolve/profiles)
	AdaptersDir     string // adapter script dir (honors override)
	CapabilityDir   string // manifest dir (real install; ignores override)
	ProjectRoot     string // writable project root (host repo)
	WorktreePath    string // optional WORKTREE_PATH override
	LLMConfigPath   string // EVOLVE_LLM_CONFIG_PATH override (empty → default)
	DispatchPlanLog string // EVOLVE_DISPATCH_PLAN_LOG path; empty disables emission
}

// ValidateProfileOptions injects the I/O + sub-process seams. Production
// wires defaults; tests supply doubles for ReadProfile, ResolveLLM,
// InspectCapability, ExecAdapter.
type ValidateProfileOptions struct {
	ReadProfile       func(path string) (string, error)
	ResolveLLM        func(agent, configPath string) (resolvellm.Result, error)
	InspectCapability func(adaptersDir, cli string) (capability.Inspection, error)
	// ExecAdapter runs the bash adapter with VALIDATE_ONLY=1. Returns the
	// CLI's exit code + any execution error. Tests supply a fake.
	ExecAdapter func(ctx context.Context, adapterPath string, env map[string]string) (exitCode int, err error)
	// AdapterExists tests whether the adapter script exists + is executable.
	// Defaults to os.Stat + executable-bit check.
	AdapterExists func(path string) bool
	// WriteFile writes the dispatch plan log. Defaults to os.WriteFile.
	WriteFile func(path string, data []byte, mode os.FileMode) error
}

// ValidateProfileResult carries every field cmd_validate_profile printed to
// stderr or returned via exit code. Callers can choose to log or assert.
type ValidateProfileResult struct {
	CLI              string
	Model            string
	CLIResolutionSrc string // "llm_config" | "llm_config_fallback" | "profile"
	Warns            []string
	AdapterOverrides AdapterOverrides
	AdapterExitCode  int
}

// AdapterOverrides mirrors profile.adapter_overrides.<cli> — the tool +
// extra-flag arrays the adapter receives via env vars.
type AdapterOverrides struct {
	ToolsJSON      string // raw JSON array string, "" when absent
	ExtraFlagsJSON string // raw JSON array string, "" when absent
}

// ValidateProfile runs the full validate pipeline:
//  1. Profile load + JSON validate.
//  2. resolvellm.Resolve → cli + model + source. "antigravity" → "agy".
//  3. Adapter existence check.
//  4. capability.Inspect → warns + manifest.
//  5. adapter_overrides extraction from profile.
//  6. Optional EVOLVE_DISPATCH_PLAN_LOG emission.
//  7. VALIDATE_ONLY=1 adapter exec.
//
// Returns the full result + nil on success. Returns (result-so-far, error)
// when any step fails — caller can inspect partial result for debugging.
func ValidateProfile(ctx context.Context, req ValidateProfileRequest, opts ValidateProfileOptions) (ValidateProfileResult, error) {
	if opts.ReadProfile == nil {
		opts.ReadProfile = defaultReadProfile
	}
	if opts.ResolveLLM == nil {
		opts.ResolveLLM = defaultResolveLLM
	}
	if opts.InspectCapability == nil {
		opts.InspectCapability = capability.Inspect
	}
	if opts.AdapterExists == nil {
		opts.AdapterExists = defaultAdapterExists
	}
	if opts.ExecAdapter == nil {
		opts.ExecAdapter = defaultExecAdapter
	}
	if opts.WriteFile == nil {
		opts.WriteFile = os.WriteFile
	}

	if req.Agent == "" {
		return ValidateProfileResult{}, fmt.Errorf("subagent/validate: agent required")
	}
	if req.ProfilesDir == "" {
		return ValidateProfileResult{}, fmt.Errorf("subagent/validate: ProfilesDir required")
	}
	if req.AdaptersDir == "" {
		return ValidateProfileResult{}, fmt.Errorf("subagent/validate: AdaptersDir required")
	}

	profilePath := filepath.Join(req.ProfilesDir, req.Agent+".json")
	profileBody, err := opts.ReadProfile(profilePath)
	if err != nil {
		return ValidateProfileResult{}, fmt.Errorf("subagent/validate: profile not found: %s", profilePath)
	}
	if !json.Valid([]byte(profileBody)) {
		return ValidateProfileResult{}, fmt.Errorf("subagent/validate: profile is not valid JSON: %s", profilePath)
	}

	llm, llmErr := opts.ResolveLLM(req.Agent, req.LLMConfigPath)
	var cli, source, resolvedModel string
	if llmErr == nil && llm.CLI != "" {
		cli = llm.CLI
		source = llm.Source
		resolvedModel = llm.ModelTier // Step 9: resolvellm emits only a tier
	} else {
		// Fall through to profile.
		cli = extractProfileString(profileBody, "cli")
		source = "profile"
		resolvedModel = ""
	}
	// Cross-name resolver: antigravity → agy
	if cli == "antigravity" {
		cli = "agy"
	}
	if cli == "" {
		return ValidateProfileResult{}, fmt.Errorf("subagent/validate: cli unresolved for agent %s", req.Agent)
	}

	adapterPath := filepath.Join(req.AdaptersDir, cli+".sh")
	if !opts.AdapterExists(adapterPath) {
		return ValidateProfileResult{}, fmt.Errorf("subagent/validate: adapter not executable: %s", adapterPath)
	}

	model := resolvedModel
	if model == "" {
		model = extractProfileString(profileBody, "model_tier_default")
	}

	capDir := req.CapabilityDir
	if capDir == "" {
		capDir = req.AdaptersDir
	}
	insp, err := opts.InspectCapability(capDir, cli)
	if err != nil {
		return ValidateProfileResult{}, fmt.Errorf("subagent/validate: capability inspect: %w", err)
	}

	overrides := extractAdapterOverrides(profileBody, cli)

	res := ValidateProfileResult{
		CLI:              cli,
		Model:            model,
		CLIResolutionSrc: source,
		Warns:            insp.Warns,
		AdapterOverrides: overrides,
	}

	if req.DispatchPlanLog != "" {
		plan := capability.DispatchPlan{
			CLI:                cli,
			Model:              model,
			CLIResolutionSrc:   source,
			CapBudgetNative:    insp.Manifest.BudgetNative,
			CapPermissionScope: insp.Manifest.PermissionScoping,
			Warns:              insp.Warns,
		}
		body := plan.PlanJSON() + "\n"
		if err := opts.WriteFile(req.DispatchPlanLog, []byte(body), 0o644); err != nil {
			return res, fmt.Errorf("subagent/validate: write dispatch plan log: %w", err)
		}
	}

	// Build adapter env. Mirrors lines 575-589 of subagent-run.sh — every
	// VALIDATE_ONLY=1 invocation expects this exact env surface.
	artifactTemplate := extractProfileString(profileBody, "output_artifact")
	artifactPath := resolveArtifactPath(artifactTemplate, 0, req.ProjectRoot)
	worktreePath := req.WorktreePath
	if worktreePath == "" {
		worktreePath = req.ProjectRoot
	}
	env := map[string]string{
		"PROFILE_PATH":                 profilePath,
		"RESOLVED_MODEL":               model,
		"PROMPT_FILE":                  "", // caller may want to inject; bash mktemps
		"CYCLE":                        "0",
		"WORKSPACE_PATH":               filepath.Join(req.ProjectRoot, ".evolve", "runs", "cycle-0"),
		"WORKTREE_PATH":                worktreePath,
		"STDOUT_LOG":                   "/dev/null",
		"STDERR_LOG":                   "/dev/null",
		"ARTIFACT_PATH":                artifactPath,
		"RESOLVED_CLI":                 cli,
		"CLI_RESOLUTION_SOURCE":        source,
		"CAP_BUDGET_NATIVE":            capBoolEnv(insp.Manifest.BudgetNative),
		"ADAPTER_TOOLS_OVERRIDE":       overrides.ToolsJSON,
		"ADAPTER_EXTRA_FLAGS_OVERRIDE": overrides.ExtraFlagsJSON,
		"VALIDATE_ONLY":                "1",
	}

	exitCode, execErr := opts.ExecAdapter(ctx, adapterPath, env)
	res.AdapterExitCode = exitCode
	if execErr != nil {
		return res, fmt.Errorf("subagent/validate: adapter exec: %w", execErr)
	}
	if exitCode != 0 {
		return res, fmt.Errorf("subagent/validate: adapter validate-only returned non-zero: %d", exitCode)
	}
	return res, nil
}

// capBoolEnv mirrors bash's `"true"`/`"false"` env emission for booleans.
func capBoolEnv(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

// adapterOverridesRE captures `"adapter_overrides":{ ... }` and inside that
// the entry for the resolved cli. Bash uses jq:
//
//	.adapter_overrides."${vp_cli}" | .tools / .extra_flags | tojson
//
// We rebuild with a narrower regex pair.
var (
	toolsArrayRE      = regexp.MustCompile(`"tools"\s*:\s*(\[[^\]]*\])`)
	extraFlagsArrayRE = regexp.MustCompile(`"extra_flags"\s*:\s*(\[[^\]]*\])`)
)

func extractAdapterOverrides(profileBody, cli string) AdapterOverrides {
	overridesBlock, ok := capabilityExtractObject(profileBody, "adapter_overrides")
	if !ok {
		return AdapterOverrides{}
	}
	cliBlock, ok := capabilityExtractObject(overridesBlock, cli)
	if !ok {
		return AdapterOverrides{}
	}
	var out AdapterOverrides
	if m := toolsArrayRE.FindStringSubmatch(cliBlock); len(m) == 2 {
		out.ToolsJSON = m[1]
	}
	if m := extraFlagsArrayRE.FindStringSubmatch(cliBlock); len(m) == 2 {
		out.ExtraFlagsJSON = m[1]
	}
	return out
}

// capabilityExtractObject mirrors capability.extractObject without exposing
// it (different package). Inline a small copy here to avoid widening
// capability's API surface.
func capabilityExtractObject(body, name string) (string, bool) {
	needle := fmt.Sprintf("\"%s\"", name)
	idx := strings.Index(body, needle)
	if idx < 0 {
		return "", false
	}
	tail := strings.TrimSpace(body[idx+len(needle):])
	if len(tail) == 0 || tail[0] != ':' {
		return "", false
	}
	tail = strings.TrimSpace(tail[1:])
	if len(tail) == 0 || tail[0] != '{' {
		return "", false
	}
	depth := 0
	for i, r := range tail {
		switch r {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return tail[1:i], true
			}
		}
	}
	return "", false
}

// defaultResolveLLM bridges to resolvellm.Resolve. configPath is ignored as of
// Step 9 (llm_config.json removed; resolvellm reads the profile). The param +
// the LLMConfigPath plumbing are now dead and can be pruned in a follow-up; the
// seam signature is kept to avoid churning subagent callers + tests in the
// removal slice.
func defaultResolveLLM(agent, _ string) (resolvellm.Result, error) {
	return resolvellm.Resolve(agent, resolvellm.Options{})
}

// defaultAdapterExists checks the executable bit. Mirrors bash `[ -x ]`.
func defaultAdapterExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return false
	}
	return info.Mode()&0o111 != 0
}

// defaultExecAdapter shells out to bash <adapter> with the given env.
// Inherits stdout/stderr so the operator sees the adapter's VALIDATE-ONLY
// chatter. Mirrors `bash "$adapter"` at subagent-run.sh:590.
func defaultExecAdapter(ctx context.Context, adapterPath string, env map[string]string) (int, error) {
	cmd := exec.CommandContext(ctx, "bash", adapterPath)
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Stdout = os.Stderr // bash adapter writes its log lines to stderr; mirror.
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode(), nil
		}
		return -1, err
	}
	return 0, nil
}
