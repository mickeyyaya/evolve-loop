package main

import (
	"fmt"
	"io"
	"os"
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
