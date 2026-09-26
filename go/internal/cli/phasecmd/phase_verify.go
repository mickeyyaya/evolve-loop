package phasecmd

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/reachabilityprobe"
)

// runPhaseVerify is the agent's deliverable self-check; it shares the host gate's verifier.
// Exit: 0 well-formed, 1 confirmed violation, 2 infra ambiguity (callers fail open), 10 usage error.
// See ADR-0034.
func runPhaseVerify(args []string, stdout, stderr io.Writer) int {
	// Take the phase positional first, then flag-parse the rest, so both --flag value and --flag=value work.
	var phaseArg string
	var flags []string
	for _, a := range args {
		if phaseArg == "" && !strings.HasPrefix(a, "-") {
			phaseArg = a
			continue
		}
		flags = append(flags, a)
	}
	if phaseArg == "" {
		fmt.Fprintf(stderr, "evolve phase verify: missing phase name\n")
		return 10
	}

	fs := flag.NewFlagSet("phase verify", flag.ContinueOnError)
	fs.SetOutput(stderr)
	workspace := fs.String("workspace", "", "per-cycle workspace dir (.evolve/runs/cycle-N)")
	worktree := fs.String("worktree", "", "isolated build worktree (optional)")
	evolveDir := fs.String("evolve-dir", "", "project .evolve dir (for orchestrator deliverables)")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON")
	if err := fs.Parse(flags); err != nil {
		return 10
	}
	phase := strings.ToLower(phaseArg)
	resolver := phaseVerifyResolver()
	contract, ok := resolver.Resolve(phase)
	if !ok {
		fmt.Fprintf(stderr, "evolve phase verify: unknown phase %q\n", phase)
		return 10
	}

	// Default from the resolver's root so deliverables, cycle state and inbox resolve against one project.
	if *evolveDir == "" {
		*evolveDir = filepath.Join(cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT"), ".evolve")
	}
	roots := phasecontract.Roots{Workspace: *workspace, Worktree: *worktree, EvolveDir: *evolveDir}
	// The gate judges explanation sections and declared effects from the cycle state, so the self-check
	// must too. Without it the section check is skipped, and an effect cannot be judged: verify exits 2.
	needsSections, needsEffects := len(contract.ExplanationSections) > 0, len(contract.Effects) > 0
	if needsSections || needsEffects {
		state, problem := persistedCycleState(*workspace, *evolveDir)
		if problem != "" && needsSections {
			fmt.Fprintf(stderr, "phase verify: WARN %s — the explanation-documentation section check is skipped; the host gate will still apply it\n", problem)
		}
		if problem != "" && needsEffects {
			fmt.Fprintf(stderr, "phase verify: WARN %s — the declared effect cannot be judged without the cycle, so verify aborts; the host gate will still apply it\n", problem)
		}
		roots.ExplanationDocumentationVersion = state.ExplanationDocumentationVersion
		roots.Cycle = state.CycleID
	}
	res, err := verifyDeliverable(phase, roots, resolver)
	if err != nil {
		fmt.Fprintf(stderr, "evolve phase verify: %v\n", err)
		return 2
	}

	if *asJSON {
		buf, _ := json.MarshalIndent(res, "", "  ")
		fmt.Fprintln(stdout, string(buf))
	} else if res.OK {
		fmt.Fprintf(stdout, "OK: %s deliverable well-formed at %s\n", phase, res.ArtifactPath)
	} else {
		fmt.Fprintf(stderr, "FAIL: %s deliverable has %d violation(s):\n", phase, len(res.Violations))
		for _, v := range res.Violations {
			fmt.Fprintf(stderr, "  - [%s] %s\n", v.Code, v.Message)
		}
	}
	if res.OK {
		return 0
	}
	return 1
}

// verifyDeliverable adds the architecture docs floor for build and the frozen-pin gate for tdd, each only
// with --worktree: the worktree is the one place the diff and the import graph exist.
// See ADR-0077.
func verifyDeliverable(phase string, roots phasecontract.Roots, resolver phasecontract.Resolver) (deliverable.Result, error) {
	stage := phaseVerifyPhaseIO()
	if phase == "build" && roots.Worktree != "" {
		changed := core.ChangedWorktreePaths(context.Background(), roots.Worktree)
		return deliverable.VerifyBuildWithChangedPathsStage(roots, changed, resolver, stage)
	}
	res, err := deliverable.VerifyWithStage(phase, roots, resolver, stage)
	if err != nil || phase != "tdd" || roots.Worktree == "" {
		return res, err
	}
	return withFrozenPinViolations(res, roots.Worktree), nil
}

// codeUnreachableFrozenPin is the stable violation code of the tdd reachability gate.
const codeUnreachableFrozenPin = "unreachable_frozen_pin"

// withFrozenPinViolations fails a tdd verdict whose frozen tests pin a call that could only build by
// closing an import cycle. Infra ambiguity fails open; only a compiler-provable cycle turns it red.
func withFrozenPinViolations(res deliverable.Result, worktree string) deliverable.Result {
	frozen, err := reachabilityprobe.FrozenTestFiles(res.ArtifactPath)
	if err != nil || len(frozen) == 0 {
		return res
	}
	violations, err := reachabilityprobe.CheckFrozenPins(worktree, frozen)
	if err != nil || len(violations) == 0 {
		return res
	}
	for i := range violations {
		res.Violations = append(res.Violations, deliverable.Violation{
			Code:    codeUnreachableFrozenPin,
			Message: violations[i].Error(),
		})
	}
	res.OK = false
	return res
}

// phaseVerifyPhaseIO resolves the PhaseIO stage exactly as the host gate does; an unreadable
// registry falls back to env and code defaults.
// See ADR-0050.
func phaseVerifyPhaseIO() config.Stage {
	cfg, _ := config.Load(config.RegistryPath(cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT")), cmdutil.FilterEvolveEnv(os.Environ()))
	return cfg.PhaseIO
}

// phaseVerifyResolver resolves contracts from the merged phase catalog the host gate uses; a load
// failure falls back to built-ins, so built-in phases always verify.
func phaseVerifyResolver() phasecontract.Resolver {
	project := cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT")
	cat, _, _, err := mergedCatalog(project)
	if err != nil {
		return phasecontract.BuiltinResolver{}
	}
	return phasecontract.NewCatalogResolver(cat.Get)
}

// persistedCycleState prefers the workspace's per-run mirror, authoritative under fleet lanes, over the
// global state file. A missing or unparseable file returns a problem description, never a silent zero state.
func persistedCycleState(workspace, evolveDir string) (state core.CycleState, problem string) {
	path := core.ResolveCycleStatePath(evolveDir)
	if workspace != "" {
		path = filepath.Join(workspace, core.RunStateFile)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return core.CycleState{}, fmt.Sprintf("%s unreadable (%v)", path, err)
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return core.CycleState{}, fmt.Sprintf("%s unparseable (%v)", path, err)
	}
	return state, ""
}
