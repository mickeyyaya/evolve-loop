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
)

// runPhaseVerify implements `evolve phase verify <phase> --workspace DIR
// [--worktree DIR] [--evolve-dir DIR] [--json]`. It is the agent-callable
// self-check (the Deliverable Contract block tells each agent to run it before
// finishing) and shares its verifier with the host-side contract gate so the
// two run byte-identical logic. ADR-0034.
//
// Exit codes:
//
//	0  — deliverable well-formed
//	1  — confirmed contract violation (agent must fix)
//	10 — usage error (missing/unknown phase)
//	2  — ambiguity/infra (e.g. unreadable dir) — caller should fail OPEN
func runPhaseVerify(args []string, stdout, stderr io.Writer) int {
	// Pull the positional phase name out FIRST (it precedes the flags in the
	// natural form `verify build --workspace X`), then flag-parse the remainder.
	// This supports both `--flag value` and `--flag=value` — unlike reorderArgs,
	// which groups flags together and lets a space-separated flag swallow the
	// next flag as its value.
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
	// Resolve through the SAME merged catalog the host-side contract gate uses, so
	// the agent's self-check and the gate agree on user/minted phases (no drift —
	// ADR-0034). A catalog-load failure degrades to built-in-only resolution.
	resolver := phaseVerifyResolver()
	if _, ok := resolver.Resolve(phase); !ok {
		fmt.Fprintf(stderr, "evolve phase verify: unknown phase %q\n", phase)
		return 10
	}

	roots := phasecontract.Roots{Workspace: *workspace, Worktree: *worktree, EvolveDir: *evolveDir}
	res, err := verifyDeliverable(phase, roots, resolver)
	if err != nil {
		// Ambiguity/infra — fail OPEN at the call site.
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

// verifyDeliverable runs the well-formedness checks, adding the ADR-0077
// documentation floor when — and only when — this invocation has a diff to
// judge: phase `build` with a `--worktree`. That is exactly the shape the
// host-side docs-floor reviewer sees, and the changed-path set comes from the
// same derivation it uses (core.ChangedWorktreePaths), so the agent's
// self-check and the gate cannot drift (ADR-0034).
//
// Fail-open everywhere else, byte-identical to before: no `--worktree` means no
// diff to classify, and the floor is build-scoped (ADR-0077) so no other
// phase's deliverable is taxed by it. A non-architecture-class diff never
// yields the violation, so ordinary cycles are unaffected.
func verifyDeliverable(phase string, roots phasecontract.Roots, resolver phasecontract.Resolver) (deliverable.Result, error) {
	stage := phaseVerifyPhaseIO()
	if phase != "build" || roots.Worktree == "" {
		return deliverable.VerifyWithStage(phase, roots, resolver, stage)
	}
	changed := core.ChangedWorktreePaths(context.Background(), roots.Worktree)
	return deliverable.VerifyBuildWithChangedPathsStage(roots, changed, resolver, stage)
}

// phaseVerifyPhaseIO resolves the EVOLVE_PHASE_IO rollout stage the SAME way the
// host gate does (config.Load over the phase registry + env), so the agent's
// self-check and the gate apply identical PhaseIO-gated checks — the package's
// no-drift invariant (ADR-0050 §3.8). A registry that cannot be read degrades to
// env + code defaults, never a hard failure.
func phaseVerifyPhaseIO() config.Stage {
	registryPath := filepath.Join(cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT"), "docs", "architecture", "phase-registry.json")
	cfg, _ := config.Load(registryPath, cmdutil.FilterEvolveEnv(os.Environ()))
	return cfg.PhaseIO
}

// phaseVerifyResolver builds a contract resolver from the merged phase catalog
// (built-in registry + .evolve/phases overlays). A load failure degrades to
// built-in-only resolution so the self-check never hard-fails on a catalog
// glitch — built-in phases always verify.
func phaseVerifyResolver() phasecontract.Resolver {
	project := cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT")
	cat, _, _, err := mergedCatalog(project)
	if err != nil {
		return phasecontract.BuiltinResolver{}
	}
	return phasecontract.NewCatalogResolver(cat.Get)
}
