package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
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
                     ( validate-profile [--dispatch-plan-log PATH] <agent> )
                     Honors EVOLVE_PROFILES_DIR_OVERRIDE,
                     EVOLVE_ADAPTERS_DIR_OVERRIDE.
  run                Execute one phase agent end-to-end (v2 cache-prefix
                     prompt + adapter exec + verify + ledger).
                     ( run <agent> <cycle> <workspace_path> )
                     Prompt read from stdin (or PROMPT_FILE_OVERRIDE).
                     Honors MODEL_TIER_HINT, ADVERSARIAL_AUDIT.
                     (LEGACY_AGENT_DISPATCH is retired — bridge is the only path.)
  dispatch-parallel  Fan-out N workers per profile.parallel_subtasks[],
                     run via fanoutdispatch + aggregator merge.
                     ( dispatch-parallel <agent> <cycle> <workspace_path> )
                     Refuses if profile.parallel_eligible != true.
                     Config via .evolve/policy.json fanout.{concurrency,
                     cache_prefix_enabled,track_workers,test_executor}.
`

// runSubagent dispatches the `evolve subagent <subcommand>` family. Mirrors
// the cmd_* subroutines in legacy/scripts/dispatch/subagent-run.sh:
//
//	--check-token        → check-token   (cmd_check_token, subagent-run.sh:597)
//	--check-ctx-advisory → check-ctx-advisory (cmd_check_ctx_advisory:605)
//	(new)                → cache-prefix  (_write_cache_prefix:292)
//	(new)                → resolve-tier  (resolve_model_tier:189)
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
	case "dispatch-parallel":
		return runSubagentDispatchParallel(args[1:], stdout, stderr)
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
			fmt.Fprintln(stdout, "Honors MODEL_TIER_HINT and .evolve/policy.json workflow settings.")
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
	wc := loadWorkflowConfig(filepath.Join(projectRoot, ".evolve"))
	tier, err := subagent.ResolveModelTier(
		subagent.ResolveModelTierRequest{
			ProfilePath:            profile,
			Cycle:                  cycle,
			ProjectRoot:            projectRoot,
			WorktreePath:           worktree,
			ModelTierHint:          os.Getenv("MODEL_TIER_HINT"),
			AuditorTierOverride:    wc.AuditorTierOverride,
			DiffComplexityDisabled: wc.DiffComplexityDisable,
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
	if cmdutil.HasHelp(args) {
		fmt.Fprintln(stdout, "Usage: evolve subagent check-token <artifact_path> <token>")
		return 0
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
	if cmdutil.HasHelp(args) {
		fmt.Fprintln(stdout, "Usage: evolve subagent check-ctx-advisory <profile_json> <tokens>")
		return 0
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
	if cmdutil.HasHelp(args) {
		fmt.Fprintln(stdout, "Usage: evolve subagent validate-profile [--dispatch-plan-log PATH] <agent>")
		fmt.Fprintln(stdout, "Env: EVOLVE_PROFILES_DIR_OVERRIDE, EVOLVE_ADAPTERS_DIR_OVERRIDE")
		return 0
	}
	var dispatchPlanLog string
	for len(args) > 0 && strings.HasPrefix(args[0], "--") {
		a := args[0]
		switch {
		case a == "--dispatch-plan-log" && len(args) > 1:
			dispatchPlanLog = args[1]
			args = args[2:]
		case strings.HasPrefix(a, "--dispatch-plan-log="):
			dispatchPlanLog = strings.TrimPrefix(a, "--dispatch-plan-log=")
			args = args[1:]
		default:
			fmt.Fprintf(stderr, "evolve subagent validate-profile: unknown flag: %s\n", a)
			return 2
		}
	}
	if len(args) != 1 {
		fmt.Fprintln(stderr, "evolve subagent validate-profile: expected <agent>")
		return 2
	}
	agent := args[0]

	layout := paths.ResolveFromEnv()

	res, err := subagent.ValidateProfile(context.Background(),
		subagent.ValidateProfileRequest{
			Agent:           agent,
			ProfilesDir:     layout.ProfilesDir,
			AdaptersDir:     layout.AdaptersDir,
			CapabilityDir:   layout.CapabilityDir,
			ProjectRoot:     layout.ProjectRoot,
			WorktreePath:    os.Getenv("WORKTREE_PATH"),
			DispatchPlanLog: dispatchPlanLog,
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
	if cmdutil.HasHelp(args) {
		fmt.Fprintln(stdout, "Usage: evolve subagent run <agent> <cycle> <workspace_path>")
		fmt.Fprintln(stdout, "Prompt: read from stdin or set PROMPT_FILE_OVERRIDE")
		fmt.Fprintln(stdout, "Env: MODEL_TIER_HINT, ADVERSARIAL_AUDIT,")
		fmt.Fprintln(stdout, "     WORKTREE_PATH")
		fmt.Fprintln(stdout, "Config: .evolve/policy.json workflow settings")
		fmt.Fprintln(stdout, "Note: LEGACY_AGENT_DISPATCH is retired — the bridge is the only dispatch path.")
		return 0
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

	layout := paths.ResolveFromEnv()

	var promptReader io.Reader
	if override := os.Getenv("PROMPT_FILE_OVERRIDE"); override != "" {
		f, err := os.Open(override)
		if err != nil {
			fmt.Fprintf(stderr, "[subagent-run] FAIL: PROMPT_FILE_OVERRIDE missing: %s\n", override)
			return 1
		}
		defer func() { _ = f.Close() }()
		promptReader = f
	} else {
		// Read from stdin.
		promptReader = os.Stdin
	}

	flags := readSubagentRunFlags()
	wc := loadWorkflowConfig(layout.EvolveDir)

	res, err := subagent.Run(context.Background(), subagent.RunRequest{
		Agent:                  agent,
		Cycle:                  cycle,
		WorkspacePath:          workspace,
		ProfilesDir:            layout.ProfilesDir,
		AdaptersDir:            layout.AdaptersDir,
		CapabilityDir:          layout.CapabilityDir,
		ProjectRoot:            layout.ProjectRoot,
		PluginRoot:             layout.PluginRoot,
		WorktreePath:           os.Getenv("WORKTREE_PATH"),
		LedgerPath:             layout.LedgerFile,
		PromptReader:           promptReader,
		ModelTierHint:          os.Getenv("MODEL_TIER_HINT"),
		AuditorTierOverride:    wc.AuditorTierOverride,
		DiffComplexityDisabled: wc.DiffComplexityDisable,
		AdversarialAudit:       flags.adversarialAudit,
		LegacyAgentDispatch:    flags.legacyAgentDispatch,
		DispatchDepth:          subagent.ReadDispatchDepth(os.Getenv),
		ChallengeTokenOverride: os.Getenv(subagent.FanoutWorkerTokenEnv),
	}, subagent.RunOptions{})
	if err != nil {
		fmt.Fprintf(stderr, "[subagent-run] FAIL: %v\n", err)
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

func runSubagentDispatchParallel(args []string, stdout, stderr io.Writer) int {
	if cmdutil.HasHelp(args) {
		fmt.Fprintln(stdout, "Usage: evolve subagent dispatch-parallel <agent> <cycle> <workspace_path>")
		fmt.Fprintln(stdout, "Config: .evolve/policy.json fanout.{concurrency,cache_prefix_enabled,track_workers,test_executor},")
		fmt.Fprintln(stdout, "     WORKTREE_PATH")
		return 0
	}
	if len(args) != 3 {
		fmt.Fprintln(stderr, "evolve subagent dispatch-parallel: expected <agent> <cycle> <workspace>")
		return 2
	}
	agent := args[0]
	cycle, err := strconv.Atoi(args[1])
	if err != nil {
		fmt.Fprintf(stderr, "evolve subagent dispatch-parallel: cycle must be integer: %v\n", err)
		return 2
	}
	workspace := args[2]

	layout := paths.ResolveFromEnv()

	pol, err := policy.Load(filepath.Join(layout.EvolveDir, "policy.json"))
	if err != nil {
		fmt.Fprintf(stderr, "[dispatch-parallel] WARN: policy load: %v; using defaults\n", err)
		pol = policy.Policy{}
	}
	fc := pol.FanoutConfig()

	res, err := subagent.DispatchParallel(context.Background(), subagent.DispatchParallelRequest{
		Agent:              agent,
		Cycle:              cycle,
		WorkspacePath:      workspace,
		ProfilesDir:        layout.ProfilesDir,
		AdaptersDir:        layout.AdaptersDir,
		CapabilityDir:      layout.CapabilityDir,
		ProjectRoot:        layout.ProjectRoot,
		PluginRoot:         layout.PluginRoot,
		LedgerPath:         layout.LedgerFile,
		WorktreePath:       os.Getenv("WORKTREE_PATH"),
		Concurrency:        fc.Concurrency,
		CachePrefixEnabled: *fc.CachePrefixEnabled,
		TrackWorkers:       *fc.TrackWorkers,
		TestExecutor:       fc.TestExecutor,
		DispatchDepth:      subagent.ReadDispatchDepth(os.Getenv),
	}, subagent.DispatchParallelOptions{})
	if err != nil {
		fmt.Fprintf(stderr, "[subagent-run] FAIL: %v\n", err)
		// parallel_eligible refusal → exit 2 to match bash semantics
		if strings.Contains(err.Error(), "not parallel_eligible") {
			return 2
		}
		return 1
	}
	fmt.Fprintf(stderr,
		"[dispatch-parallel] DONE agent=%s cycle=%d workers=%d fanout_rc=%d agg_rc=%d aggregate=%s tier=%s\n",
		agent, cycle, res.WorkerCount, res.FanoutExitCode, res.AggregatorExit, res.AggregatePath, res.QualityTier,
	)
	if res.FanoutExitCode != 0 || res.AggregatorExit != 0 {
		return 1
	}
	return 0
}

func nextArg(args []string, i int) string {
	if i >= len(args) {
		return ""
	}
	return args[i]
}

// envOrCwd forwards to cmdutil.EnvOrCwd — the implementation lives there so
// the decomposed internal/cli/* command groups share one definition.
func envOrCwd(env string) string { return cmdutil.EnvOrCwd(env) }

// sourceRoot resolves the root for reading SOURCE/DOC artifacts that are part
// of a cycle's committed deliverable — generated-from-source docs such as
// docs/architecture/control-flags.md (from the flagregistry) or skills/*/SKILL.md
// (from phase facts). These live in the WORKTREE the cycle commits to, not the
// main checkout.
//
// This is the source half of the dual-root pattern. EVOLVE_PROJECT_ROOT is the
// STATE root: the ACS suite pins it to MAIN so predicates resolve `.evolve/`
// runtime data there (issue #12). But a generated SOURCE doc must be validated
// against the worktree, so acssuite also exports EVOLVE_WORKTREE_ROOT=<worktree>.
// Precedence: EVOLVE_WORKTREE_ROOT (the active worktree, exported by the ACS
// suite) > EVOLVE_PROJECT_ROOT (explicit root in CI/dev) > cwd. Outside the
// suite EVOLVE_WORKTREE_ROOT is conventionally unset, making this byte-identical
// to envOrCwd("EVOLVE_PROJECT_ROOT"); if it is present in a developer's shell
// from a prior session it takes precedence (unset it if a command reads the
// wrong root). Root-cause fix for the cycle-355 trap, where `flags check` read
// main's stale control-flags.md and red-failed correct work.
func sourceRoot() string {
	if v := os.Getenv(ipcenv.WorktreeRootKey); v != "" {
		return paths.AbsoluteRoot(ipcenv.WorktreeRootKey, v, nil)
	}
	return envOrCwd("EVOLVE_PROJECT_ROOT")
}

// subagentRunFlags holds the run-specific env-derived boolean knobs for
// `subagent run`, read through envchain so the truthy/falsy/default vocabulary
// is uniform (P2). AdversarialAudit is default-on (`!= "0"`);
// LegacyAgentDispatch is default-off (`== "1"`). DiffComplexityDisabled is read
// inline (shared with the resolve-tier handler), not via this struct.
type subagentRunFlags struct {
	adversarialAudit    bool
	legacyAgentDispatch bool
}

func readSubagentRunFlags() subagentRunFlags {
	return subagentRunFlags{
		adversarialAudit:    envchain.Bool("ADVERSARIAL_AUDIT", nil, true),
		legacyAgentDispatch: envchain.Bool("LEGACY_AGENT_DISPATCH", nil, false),
	}
}
