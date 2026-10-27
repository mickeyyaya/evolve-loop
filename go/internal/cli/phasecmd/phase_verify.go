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
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func (c phaseCommand) runPhaseVerify(args []string, stdout, stderr io.Writer) int {
	phaseArg, flags := splitPhaseArg(args)
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

	if *evolveDir == "" {
		*evolveDir = filepath.Join(cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT"), ".evolve")
	}
	flagged := phasecontract.Roots{Workspace: *workspace, Worktree: *worktree, EvolveDir: *evolveDir}
	var res deliverable.Result
	var err error
	if phase == "build" {
		res, err = c.verifyBuild(flagged, stderr)
	} else {
		res, err = verifyDeliverable(phase, withCycleState(flagged, contract, stderr), resolver)
	}
	if err != nil {
		fmt.Fprintf(stderr, "evolve phase verify: %v\n", err)
		return 2
	}
	return reportVerifyResult(phase, res, *asJSON, stdout, stderr)
}

func (c phaseCommand) verifyBuild(flagged phasecontract.Roots, stderr io.Writer) (deliverable.Result, error) {
	probe := core.BuildHandoffProbe{Workspace: flagged.Workspace, EvolveDir: flagged.EvolveDir, ProjectRoot: cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT"), Worktree: flagged.Worktree}
	checked, err := BuildSelfCheck{Floor: c.floor, Probe: probe}.Verify(stderr)
	return checked.Result, err
}

func splitPhaseArg(args []string) (phaseArg string, flags []string) {
	for _, a := range args {
		if phaseArg == "" && !strings.HasPrefix(a, "-") {
			phaseArg = a
			continue
		}
		flags = append(flags, a)
	}
	return phaseArg, flags
}

func withCycleState(roots phasecontract.Roots, contract phasecontract.Contract, stderr io.Writer) phasecontract.Roots {
	needsSections, needsEffects := len(contract.ExplanationSections) > 0, len(contract.Effects) > 0
	if !needsSections && !needsEffects {
		return roots
	}
	state, err := core.ReadRunCycleState(roots.Workspace, roots.EvolveDir)
	if err != nil && needsSections {
		fmt.Fprintf(stderr, "phase verify: WARN %v — the explanation-documentation section check is skipped; the host gate will still apply it\n", err)
	}
	if err != nil && needsEffects {
		fmt.Fprintf(stderr, "phase verify: WARN %v — the declared effect cannot be judged without the cycle, so verify aborts; the host gate will still apply it\n", err)
	}
	roots.ExplanationDocumentationVersion = state.ExplanationDocumentationVersion
	roots.Cycle = state.CycleID
	return roots
}

func reportVerifyResult(phase string, res deliverable.Result, asJSON bool, stdout, stderr io.Writer) int {
	if asJSON {
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
	return withTddCommentViolations(withFrozenPinViolations(res, roots.Worktree), roots.Worktree), nil
}

const codeTddCommentFloor = "tdd_comment_floor"

func withTddCommentViolations(res deliverable.Result, worktree string) deliverable.Result {
	files, err := reachabilityprobe.HandoffTestFiles(res.ArtifactPath)
	if err != nil || len(files) == 0 {
		return res
	}
	lines := core.TddCommentFloorFailures(context.Background(), worktree, cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT"), files, io.Discard)
	if len(lines) == 0 {
		return res
	}
	res.Violations = append(res.Violations, deliverable.Violation{
		Code:    codeTddCommentFloor,
		Message: fmt.Sprintf("the predicate files add %d comment line(s); delete each one (docs/conventions/code-comments.md):\n    %s", len(lines), strings.Join(lines, "\n    ")),
	})
	res.OK = false
	return res
}

const codeUnreachableFrozenPin = "unreachable_frozen_pin"

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

func phaseVerifyPhaseIO() config.Stage {
	cfg, _ := router.ConfigForProject(cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT"), cmdutil.FilterEvolveEnv(os.Environ()), os.Stderr)
	return cfg.PhaseIO
}

func phaseVerifyResolver() phasecontract.Resolver {
	project := cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT")
	cat, _, _, err := mergedCatalog(project)
	if err != nil {
		return phasecontract.BuiltinResolver{}
	}
	return phasecontract.NewCatalogResolver(cat.Get)
}
