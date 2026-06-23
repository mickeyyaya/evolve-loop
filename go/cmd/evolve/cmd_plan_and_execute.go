package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolveloop/go/internal/cli/phasecmd"

	"github.com/mickeyyaya/evolveloop/go/internal/envchain"
	"github.com/mickeyyaya/evolveloop/go/internal/phases/registry"
)

// runPlanAndExecute implements `evolve plan-and-execute <phase>`. It is
// the operator-facing convenience for v12.1 Capability 1's two-pass
// dispatch:
//
//  1. Pass A: run the phase with --permission-mode=plan, writing the
//     plan to EVOLVE_<PHASE>_PLAN_OUTPUT (default: <workspace>/<phase>-plan.md).
//  2. Pass B: re-run the phase with --permission-mode=acceptEdits,
//     prepending the plan to the prompt via EVOLVE_<PHASE>_PLAN_INPUT.
//
// Each pass sets EVOLVE_<PHASE>_PERMISSION_MODE; the phase runner resolves
// it into the typed BridgeRequest.PermissionMode, which the bridge realizes
// per-CLI via the LaunchIntent. This subcommand orchestrates the two
// dispatches and threads the plan artifact between them.
//
// Exit codes:
//   - 0  both passes succeeded
//   - 1  pass A failed
//   - 2  pass B failed (after pass A succeeded)
//   - 10 bad args
//   - 11 plan artifact missing after pass A
func runPlanAndExecute(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("plan-and-execute", flag.ContinueOnError)
	fs.SetOutput(stderr)
	planOutput := fs.String("plan-output", "", "path where pass A writes the plan (default: <workspace>/<phase>-plan.md)")
	workspace := fs.String("workspace", "", "workspace directory; used to derive the plan artifact path when --plan-output is absent")
	skipExecute := fs.Bool("skip-execute", false, "run only pass A (plan mode); useful for review-then-resume workflows")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	rest := fs.Args()
	if len(rest) < 1 {
		fmt.Fprintf(stderr, "evolve plan-and-execute: missing <phase> (known: %s)\n",
			joinNames(registry.Names()))
		return 10
	}
	phase := rest[0]
	if _, ok := registry.For(phase); !ok {
		fmt.Fprintf(stderr, "evolve plan-and-execute: unknown phase %q (known: %s)\n",
			phase, joinNames(registry.Names()))
		return 10
	}

	// Read the rest of args as phase-request JSON via stdin OR as
	// flags passed through. For simplicity at this slice, the phase
	// request envelope is read from stdin (same as `evolve phase`)
	// and re-fed to each pass.
	envReq, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "evolve plan-and-execute: read stdin: %v\n", err)
		return 1
	}

	// Resolve plan-output path. Default: <workspace>/<phase>-plan.md.
	// We don't have workspace from the bare CLI; derive from --workspace or use cwd.
	planPath := *planOutput
	if planPath == "" {
		if *workspace != "" {
			planPath = filepath.Join(*workspace, phase+"-plan.md")
		} else {
			planPath = filepath.Join(".", phase+"-plan.md")
		}
	}

	// Pass A: plan mode. Sets EVOLVE_<PHASE>_PERMISSION_MODE=plan and
	// EVOLVE_<PHASE>_PLAN_OUTPUT=<planPath>. The runner resolves the
	// permission mode into BridgeRequest.PermissionMode (realized per-CLI).
	planKey := envchain.PhaseEnvKey(phase, "PERMISSION_MODE")
	outputKey := envchain.PhaseEnvKey(phase, "PLAN_OUTPUT")

	fmt.Fprintf(stdout, "[plan-and-execute] pass A: %s in plan mode → %s\n", phase, planPath)
	if code := runPassWithEnv(phase, envReq, map[string]string{
		planKey:   "plan",
		outputKey: planPath,
	}, stdout, stderr); code != 0 {
		fmt.Fprintf(stderr, "[plan-and-execute] pass A failed (exit=%d); aborting\n", code)
		return 1
	}

	if *skipExecute {
		fmt.Fprintln(stdout, "[plan-and-execute] --skip-execute set; pass B skipped")
		return 0
	}

	// Verify the plan artifact exists before pass B.
	if _, err := os.Stat(planPath); err != nil {
		fmt.Fprintf(stderr, "[plan-and-execute] plan artifact missing at %s (pass A produced no plan): %v\n",
			planPath, err)
		return 11
	}

	// Pass B: execute mode. Sets EVOLVE_<PHASE>_PERMISSION_MODE=acceptEdits
	// and EVOLVE_<PHASE>_PLAN_INPUT=<planPath> so the agent reads the
	// plan as context.
	inputKey := envchain.PhaseEnvKey(phase, "PLAN_INPUT")

	fmt.Fprintf(stdout, "[plan-and-execute] pass B: %s in acceptEdits mode (plan ← %s)\n", phase, planPath)
	if code := runPassWithEnv(phase, envReq, map[string]string{
		planKey:  "acceptEdits",
		inputKey: planPath,
	}, stdout, stderr); code != 0 {
		fmt.Fprintf(stderr, "[plan-and-execute] pass B failed (exit=%d)\n", code)
		return 2
	}
	fmt.Fprintln(stdout, "[plan-and-execute] both passes PASS")
	return 0
}

// runPassWithEnv invokes runPhase with the given extra env vars set
// in the process env for the duration of the call. Saves and restores
// the previous values so concurrent invocations don't collide.
func runPassWithEnv(phase string, req []byte, env map[string]string, stdout, stderr io.Writer) int {
	saved := map[string]string{}
	for k, v := range env {
		saved[k] = os.Getenv(k)
		_ = os.Setenv(k, v)
	}
	defer func() {
		for k, v := range saved {
			if v == "" {
				_ = os.Unsetenv(k)
			} else {
				_ = os.Setenv(k, v)
			}
		}
	}()
	return phasecmd.RunPhase([]string{phase}, bytes.NewReader(req), stdout, stderr)
}

// joinNames is a small slice-to-comma-separated helper. Could use
// strings.Join but keeping the import surface minimal in this file.
func joinNames(names []string) string {
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ", "
		}
		out += n
	}
	return out
}
