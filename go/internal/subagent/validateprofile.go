package subagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/capability"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/detectcli"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
	"github.com/mickeyyaya/evolve-loop/go/internal/subagent/subagentrun"
)

type ValidateProfileRequest struct {
	Agent           string
	ProfilesDir     string
	AdaptersDir     string
	CapabilityDir   string
	ProjectRoot     string
	WorktreePath    string
	DispatchPlanLog string
}

type ValidateProfileOptions struct {
	ReadProfile       func(path string) (string, error)
	ResolveLLM        func(agent string) (resolvellm.Result, error)
	InspectCapability func(adaptersDir, cli string) (capability.Inspection, error)
	ExecAdapter       func(ctx context.Context, adapterPath string, env map[string]string) (exitCode int, err error)
	AdapterExists     func(path string) bool
	WriteFile         func(path string, data []byte, mode os.FileMode) error
}

type ValidateProfileResult struct {
	CLI              string
	Model            string
	CLIResolutionSrc string
	Warns            []string
	AdapterOverrides AdapterOverrides
	AdapterExitCode  int
}

type AdapterOverrides struct {
	ToolsJSON      string
	ExtraFlagsJSON string
}

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

	cli, source, resolvedModel, err := resolvedCLI(opts.ResolveLLM, req.Agent, profileBody)
	if err != nil {
		return ValidateProfileResult{}, err
	}
	cli = detectcli.Canonical(cli)
	if cli == "" {
		return ValidateProfileResult{}, fmt.Errorf("subagent/validate: cli unresolved for agent %s", req.Agent)
	}

	adapterPath := filepath.Join(req.AdaptersDir, cli+".sh")
	if !opts.AdapterExists(adapterPath) {
		return ValidateProfileResult{}, fmt.Errorf("subagent/validate: adapter not executable: %s", adapterPath)
	}

	model := resolvedModel
	if model == "" {
		model = matchField(profileBody, reFieldTierDefault)
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

	artifactTemplate := matchField(profileBody, reFieldOutputArtifact)
	artifactPath := resolveArtifactPath(artifactTemplate, 0, req.ProjectRoot)
	worktreePath := req.WorktreePath
	if worktreePath == "" {
		worktreePath = req.ProjectRoot
	}
	env := map[string]string{
		"PROFILE_PATH":                 profilePath,
		"RESOLVED_MODEL":               model,
		"PROMPT_FILE":                  "",
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

func capBoolEnv(v bool) string { return subagentrun.BoolEnv(v) }

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

func resolvedCLI(resolve func(string) (resolvellm.Result, error), agent, profileBody string) (string, string, string, error) {
	llm, err := resolve(agent)
	switch {
	case errors.Is(err, cliroute.ErrRefused):
		return "", "", "", fmt.Errorf("subagent/validate: %w", err)
	case err == nil && llm.CLI != "":
		return llm.CLI, llm.Source, llm.ModelTier, nil
	}
	return matchField(profileBody, reFieldCLI), "profile", "", nil
}

func defaultResolveLLM(agent string) (resolvellm.Result, error) {
	router, err := cliroute.NewSingleProfileRouter(policy.Policy{}, cliroute.SingleProfile{}, cliroute.Host{})
	if err != nil {
		return resolvellm.Result{}, err
	}
	return router.ResolveRole(agent, resolvellm.Options{})
}

func defaultAdapterExists(path string) bool {
	return driverExists(strings.TrimSuffix(filepath.Base(path), ".sh"))
}
