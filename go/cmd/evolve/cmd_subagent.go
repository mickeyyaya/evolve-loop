package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/subagent"
)

const subagentUsage = `Usage: evolve subagent <subcommand> [arguments]

Subcommands:
  cache-prefix       Write deterministic cache-prefix file (sibling fan-out)
                     ( --cycle N --agent NAME --workspace PATH --out PATH
                       [--project-root PATH] )
  resolve-tier       Resolve model tier per agent profile + env overrides
                     ( --profile PATH --cycle N [--project-root PATH]
                       [--worktree PATH] )
  check-token        Verify artifact contains challenge token
                     ( check-token <artifact_path> <token> )
                     Exit 0 = OK, 2 = integrity fail.
  check-ctx-advisory Emit token-budget advisory when over profile threshold
                     ( check-ctx-advisory <profile_json> <tokens> )
                     Always exits 0; advisory printed to stderr.
  validate-profile   Validate agent profile JSON + adapter capabilities +
                     dispatch plan log; runs adapter with VALIDATE_ONLY=1
                     ( validate-profile <agent> )
                     Honors EVOLVE_PROFILES_DIR_OVERRIDE,
                     EVOLVE_ADAPTERS_DIR_OVERRIDE,
                     EVOLVE_DISPATCH_PLAN_LOG,
                     EVOLVE_LLM_CONFIG_PATH.
  run                Execute one phase agent end-to-end (v2 cache-prefix
                     prompt + adapter exec + verify + ledger).
                     ( run <agent> <cycle> <workspace_path> )
                     Prompt read from stdin (or PROMPT_FILE_OVERRIDE).
                     Honors MODEL_TIER_HINT, ADVERSARIAL_AUDIT,
                     EVOLVE_CACHE_PREFIX_V2, LEGACY_AGENT_DISPATCH.
`

// runSubagent dispatches the `evolve subagent <subcommand>` family. Mirrors
// the cmd_* subroutines in legacy/scripts/dispatch/subagent-run.sh:
//   --check-token        → check-token   (cmd_check_token, subagent-run.sh:597)
//   --check-ctx-advisory → check-ctx-advisory (cmd_check_ctx_advisory:605)
//   (new)                → cache-prefix  (_write_cache_prefix:292)
//   (new)                → resolve-tier  (resolve_model_tier:189)
func runSubagent(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprint(stderr, subagentUsage)
		return 2
	}
	switch args[0] {
	case "-h", "--help", "help":
		fmt.Fprint(stdout, subagentUsage)
		return 0
	case "cache-prefix":
		return runSubagentCachePrefix(args[1:], stdout, stderr)
	case "resolve-tier":
		return runSubagentResolveTier(args[1:], stdout, stderr)
	case "check-token":
		return runSubagentCheckToken(args[1:], stdout, stderr)
	case "check-ctx-advisory":
		return runSubagentCheckCtxAdvisory(args[1:], stdout, stderr)
	case "validate-profile":
		return runSubagentValidateProfile(args[1:], stdout, stderr)
	case "run":
		return runSubagentRun(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve subagent: unknown subcommand %q\n\n%s", args[0], subagentUsage)
		return 2
	}
}

func runSubagentCachePrefix(args []string, stdout, stderr io.Writer) int {
	var (
		cycleStr, agent, workspace, out, projectRoot string
	)
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--help", "-h":
			fmt.Fprintln(stdout, "Usage: evolve subagent cache-prefix --cycle N --agent NAME --workspace PATH --out PATH [--project-root PATH]")
			return 0
		case "--cycle":
			i++
			cycleStr = nextArg(args, i)
		case "--agent":
			i++
			agent = nextArg(args, i)
		case "--workspace":
			i++
			workspace = nextArg(args, i)
		case "--out":
			i++
			out = nextArg(args, i)
		case "--project-root":
			i++
			projectRoot = nextArg(args, i)
		default:
			fmt.Fprintf(stderr, "evolve subagent cache-prefix: unknown arg %q\n", a)
			return 2
		}
	}
	if cycleStr == "" || agent == "" || workspace == "" || out == "" {
		fmt.Fprintln(stderr, "evolve subagent cache-prefix: --cycle, --agent, --workspace, --out are required")
		return 2
	}
	cycle, err := strconv.Atoi(cycleStr)
	if err != nil {
		fmt.Fprintf(stderr, "evolve subagent cache-prefix: --cycle must be int: %v\n", err)
		return 2
	}
	if projectRoot == "" {
		projectRoot = envOrCwd("EVOLVE_PROJECT_ROOT")
	}
	if err := subagent.WriteCachePrefix(subagent.CachePrefixRequest{
		Cycle:       cycle,
		Agent:       agent,
		Workspace:   workspace,
		OutPath:     out,
		ProjectRoot: projectRoot,
	}, subagent.CachePrefixOptions{}); err != nil {
		fmt.Fprintf(stderr, "evolve subagent cache-prefix: %v\n", err)
		return 1
	}
	return 0
}

func runSubagentResolveTier(args []string, stdout, stderr io.Writer) int {
	var (
		profile, cycleStr, projectRoot, worktree string
	)
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--help", "-h":
			fmt.Fprintln(stdout, "Usage: evolve subagent resolve-tier --profile PATH --cycle N [--project-root PATH] [--worktree PATH]")
			fmt.Fprintln(stdout, "Honors MODEL_TIER_HINT, EVOLVE_AUDITOR_TIER_OVERRIDE, EVOLVE_DIFF_COMPLEXITY_DISABLE.")
			return 0
		case "--profile":
			i++
			profile = nextArg(args, i)
		case "--cycle":
			i++
			cycleStr = nextArg(args, i)
		case "--project-root":
			i++
			projectRoot = nextArg(args, i)
		case "--worktree":
			i++
			worktree = nextArg(args, i)
		default:
			fmt.Fprintf(stderr, "evolve subagent resolve-tier: unknown arg %q\n", a)
			return 2
		}
	}
	if profile == "" || cycleStr == "" {
		fmt.Fprintln(stderr, "evolve subagent resolve-tier: --profile and --cycle are required")
		return 2
	}
	cycle, err := strconv.Atoi(cycleStr)
	if err != nil {
		fmt.Fprintf(stderr, "evolve subagent resolve-tier: --cycle must be int: %v\n", err)
		return 2
	}
	if projectRoot == "" {
		projectRoot = envOrCwd("EVOLVE_PROJECT_ROOT")
	}
	tier, err := subagent.ResolveModelTier(
		subagent.ResolveModelTierRequest{
			ProfilePath:            profile,
			Cycle:                  cycle,
			ProjectRoot:            projectRoot,
			WorktreePath:           worktree,
			ModelTierHint:          os.Getenv("MODEL_TIER_HINT"),
			AuditorTierOverride:    os.Getenv("EVOLVE_AUDITOR_TIER_OVERRIDE"),
			DiffComplexityDisabled: os.Getenv("EVOLVE_DIFF_COMPLEXITY_DISABLE") == "1",
		},
		subagent.ResolveModelTierOptions{},
	)
	if err != nil {
		fmt.Fprintf(stderr, "evolve subagent resolve-tier: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, tier)
	return 0
}

func runSubagentCheckToken(args []string, stdout, stderr io.Writer) int {
	// Strip any single -h/--help so positional parsing is uniform.
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Fprintln(stdout, "Usage: evolve subagent check-token <artifact_path> <token>")
			return 0
		}
	}
	if len(args) != 2 {
		fmt.Fprintln(stderr, "evolve subagent check-token: expected <artifact_path> <token>")
		return 2
	}
	res := subagent.CheckToken(args[0], args[1])
	if !res.OK {
		fmt.Fprintf(stderr, "[subagent-run] INTEGRITY-FAIL: %s\n", res.Reason)
		return 2
	}
	fmt.Fprintf(stderr, "[subagent-run] %s\n", res.Reason)
	return 0
}

func runSubagentCheckCtxAdvisory(args []string, stdout, stderr io.Writer) int {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Fprintln(stdout, "Usage: evolve subagent check-ctx-advisory <profile_json> <tokens>")
			return 0
		}
	}
	if len(args) != 2 {
		fmt.Fprintln(stderr, "evolve subagent check-ctx-advisory: expected <profile_json> <tokens>")
		return 2
	}
	tokens, err := strconv.Atoi(args[1])
	if err != nil {
		fmt.Fprintf(stderr, "evolve subagent check-ctx-advisory: tokens must be int: %v\n", err)
		return 2
	}
	res, err := subagent.CheckCtxAdvisory(args[0], tokens)
	if err != nil {
		// Bash WARNs + exits 0 on missing profile. Mirror that.
		fmt.Fprintf(stderr, "[subagent-run] WARN: %v\n", err)
		return 0
	}
	if res.Emit {
		fmt.Fprintf(stderr, "[subagent-run] INFO: %s\n", res.Message)
	}
	return 0
}

func runSubagentValidateProfile(args []string, stdout, stderr io.Writer) int {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Fprintln(stdout, "Usage: evolve subagent validate-profile <agent>")
			fmt.Fprintln(stdout, "Env: EVOLVE_PROFILES_DIR_OVERRIDE, EVOLVE_ADAPTERS_DIR_OVERRIDE,")
			fmt.Fprintln(stdout, "     EVOLVE_DISPATCH_PLAN_LOG, EVOLVE_LLM_CONFIG_PATH")
			return 0
		}
	}
	if len(args) != 1 {
		fmt.Fprintln(stderr, "evolve subagent validate-profile: expected <agent>")
		return 2
	}
	agent := args[0]

	projectRoot := envOrCwd("EVOLVE_PROJECT_ROOT")
	pluginRoot := os.Getenv("EVOLVE_PLUGIN_ROOT")
	if pluginRoot == "" {
		pluginRoot = projectRoot
	}

	profilesDir := os.Getenv("EVOLVE_PROFILES_DIR_OVERRIDE")
	if profilesDir == "" {
		profilesDir = filepath.Join(pluginRoot, ".evolve", "profiles")
	}
	adaptersDir := os.Getenv("EVOLVE_ADAPTERS_DIR_OVERRIDE")
	if adaptersDir == "" {
		adaptersDir = filepath.Join(pluginRoot, "legacy", "scripts", "cli_adapters")
	}
	// CapabilityDir mirrors bash REAL_ADAPTERS_DIR: script-relative, never
	// honors EVOLVE_ADAPTERS_DIR_OVERRIDE. Resolves to the plugin install
	// path so capability manifests reflect actual installed capabilities,
	// not a test-seam sentinel dir.
	capabilityDir := filepath.Join(pluginRoot, "legacy", "scripts", "cli_adapters")

	llmConfigPath := os.Getenv("EVOLVE_LLM_CONFIG_PATH")
	if llmConfigPath == "" {
		llmConfigPath = filepath.Join(projectRoot, ".evolve", "llm_config.json")
	}

	res, err := subagent.ValidateProfile(context.Background(),
		subagent.ValidateProfileRequest{
			Agent:           agent,
			ProfilesDir:     profilesDir,
			AdaptersDir:     adaptersDir,
			CapabilityDir:   capabilityDir,
			ProjectRoot:     projectRoot,
			WorktreePath:    os.Getenv("WORKTREE_PATH"),
			LLMConfigPath:   llmConfigPath,
			DispatchPlanLog: os.Getenv("EVOLVE_DISPATCH_PLAN_LOG"),
		},
		subagent.ValidateProfileOptions{},
	)
	if err != nil {
		fmt.Fprintf(stderr, "[subagent-run] FAIL: %v\n", err)
		return 1
	}
	// Mirror bash stderr lines (cli_resolution + dispatch-resolve + profile valid).
	fmt.Fprintf(stderr, "[dispatch-resolve] cli=%s source=%s model=%s\n",
		res.CLI, res.CLIResolutionSrc, res.Model)
	fmt.Fprintf(stderr, "[subagent-run] cli_resolution: source=%s target_cli=%s\n",
		res.CLIResolutionSrc, res.CLI)
	for _, w := range res.Warns {
		fmt.Fprintln(stderr, w)
	}
	fmt.Fprintf(stderr, "[subagent-run] profile valid: %s\n", agent)
	return 0
}

func runSubagentRun(args []string, stdout, stderr io.Writer) int {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Fprintln(stdout, "Usage: evolve subagent run <agent> <cycle> <workspace_path>")
			fmt.Fprintln(stdout, "Prompt: read from stdin or set PROMPT_FILE_OVERRIDE")
			fmt.Fprintln(stdout, "Env: MODEL_TIER_HINT, EVOLVE_AUDITOR_TIER_OVERRIDE, ADVERSARIAL_AUDIT,")
			fmt.Fprintln(stdout, "     EVOLVE_CACHE_PREFIX_V2, EVOLVE_DIFF_COMPLEXITY_DISABLE,")
			fmt.Fprintln(stdout, "     LEGACY_AGENT_DISPATCH, WORKTREE_PATH")
			return 0
		}
	}
	if len(args) != 3 {
		fmt.Fprintln(stderr, "evolve subagent run: expected <agent> <cycle> <workspace>")
		return 2
	}
	agent := args[0]
	cycle, err := strconv.Atoi(args[1])
	if err != nil {
		fmt.Fprintf(stderr, "evolve subagent run: cycle must be integer: %v\n", err)
		return 2
	}
	workspace := args[2]

	projectRoot := envOrCwd("EVOLVE_PROJECT_ROOT")
	pluginRoot := os.Getenv("EVOLVE_PLUGIN_ROOT")
	if pluginRoot == "" {
		pluginRoot = projectRoot
	}
	profilesDir := os.Getenv("EVOLVE_PROFILES_DIR_OVERRIDE")
	if profilesDir == "" {
		profilesDir = filepath.Join(pluginRoot, ".evolve", "profiles")
	}
	adaptersDir := os.Getenv("EVOLVE_ADAPTERS_DIR_OVERRIDE")
	if adaptersDir == "" {
		adaptersDir = filepath.Join(pluginRoot, "legacy", "scripts", "cli_adapters")
	}
	capabilityDir := filepath.Join(pluginRoot, "legacy", "scripts", "cli_adapters")
	llmConfigPath := os.Getenv("EVOLVE_LLM_CONFIG_PATH")
	if llmConfigPath == "" {
		llmConfigPath = filepath.Join(projectRoot, ".evolve", "llm_config.json")
	}
	ledgerPath := os.Getenv("EVOLVE_LEDGER_OVERRIDE")
	if ledgerPath == "" {
		ledgerPath = filepath.Join(projectRoot, ".evolve", "ledger.jsonl")
	}

	var promptReader io.Reader
	if override := os.Getenv("PROMPT_FILE_OVERRIDE"); override != "" {
		f, err := os.Open(override)
		if err != nil {
			fmt.Fprintf(stderr, "[subagent-run] FAIL: PROMPT_FILE_OVERRIDE missing: %s\n", override)
			return 1
		}
		defer f.Close()
		promptReader = f
	} else {
		// Read from stdin.
		promptReader = os.Stdin
	}

	cachePrefixV2 := os.Getenv("EVOLVE_CACHE_PREFIX_V2") != "0"
	adversarialAudit := os.Getenv("ADVERSARIAL_AUDIT") != "0"

	res, err := subagent.Run(context.Background(), subagent.RunRequest{
		Agent:                  agent,
		Cycle:                  cycle,
		WorkspacePath:          workspace,
		ProfilesDir:            profilesDir,
		AdaptersDir:            adaptersDir,
		CapabilityDir:          capabilityDir,
		ProjectRoot:            projectRoot,
		PluginRoot:             pluginRoot,
		WorktreePath:           os.Getenv("WORKTREE_PATH"),
		LLMConfigPath:          llmConfigPath,
		LedgerPath:             ledgerPath,
		PromptReader:           promptReader,
		ModelTierHint:          os.Getenv("MODEL_TIER_HINT"),
		AuditorTierOverride:    os.Getenv("EVOLVE_AUDITOR_TIER_OVERRIDE"),
		DiffComplexityDisabled: os.Getenv("EVOLVE_DIFF_COMPLEXITY_DISABLE") == "1",
		CachePrefixV2:          cachePrefixV2,
		AdversarialAudit:       adversarialAudit,
		LegacyAgentDispatch:    os.Getenv("LEGACY_AGENT_DISPATCH") == "1",
	}, subagent.RunOptions{})
	if err != nil {
		fmt.Fprintf(stderr, "[subagent-run] FAIL: %v\n", err)
		return 1
	}
	if res.LegacyDispatch {
		fmt.Fprintln(stderr, "[subagent-run] LEGACY_AGENT_DISPATCH=1 — orchestrator should fall back to in-process Agent tool")
		fmt.Fprintln(stdout, "LEGACY_DISPATCH")
		return 1
	}
	for _, w := range res.Warns {
		fmt.Fprintln(stderr, w)
	}
	fmt.Fprintf(stderr, "[subagent-run] verdict=%s agent=%s cycle=%d artifact=%s exit=%d duration_ms=%d\n",
		res.Verdict, agent, cycle, res.ArtifactPath, res.ExitCode, res.DurationMS)
	switch res.Verdict {
	case subagent.VerdictPASS:
		return 0
	case subagent.VerdictIntegrityFail:
		return 2
	default:
		return 1
	}
}

func nextArg(args []string, i int) string {
	if i >= len(args) {
		return ""
	}
	return args[i]
}

func envOrCwd(env string) string {
	if v := os.Getenv(env); v != "" {
		return v
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}
