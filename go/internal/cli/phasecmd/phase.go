package phasecmd

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"

	// Blank imports register every built-in phase through its package init().
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/build"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/debugger"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/intent"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/retro"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/scout"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/tdd"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/triage"
)

type BuildHandoffFloorFor func(projectRoot string) core.BuildHandoffFloor

type RouterInstaller func(projectRoot string) error

const exitRoutingRefused = 2

type phaseCommand struct {
	floor   BuildHandoffFloorFor
	install RouterInstaller
}

func NewRunPhase(floor BuildHandoffFloorFor, install RouterInstaller) func(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return phaseCommand{floor: floor, install: install}.run
}

func (c phaseCommand) run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintf(stderr, "evolve phase: missing phase name (%s)\n", strings.Join(registry.Names(), "|"))
		return 10
	}
	if strings.ToLower(args[0]) == "verify" {
		return c.runPhaseVerify(args[1:], stdout, stderr)
	}
	if strings.ToLower(args[0]) == "lint" {
		return runPhaseLint(args[1:], stdout, stderr)
	}
	name := strings.ToLower(args[0])
	if !IsKnownPhase(name, "") {
		fmt.Fprintln(stderr, FormatUnknownPhaseError("evolve phase", name, ""))
		return 10
	}

	req, rc := phaseRequest(args[1:], stdin, stderr)
	if rc != 0 {
		return rc
	}

	if err := c.installRouter(req.ProjectRoot); err != nil {
		fmt.Fprintf(stderr, "evolve phase: %s: %v\n", name, err)
		return exitRoutingRefused
	}
	runner, rc := resolvePhaseRunner(name, req, stderr)
	if rc != 0 {
		return rc
	}
	resp, err := runner.Run(context.Background(), req)
	if err != nil {
		// Emit the partial response anyway so the parent can read its diagnostics.
		buf, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(stdout, string(buf))
		fmt.Fprintf(stderr, "evolve phase: %s: %v\n", name, err)
		return 1
	}
	buf, mErr := json.MarshalIndent(resp, "", "  ")
	if mErr != nil {
		fmt.Fprintf(stderr, "evolve phase: marshal response: %v\n", mErr)
		return 1
	}
	fmt.Fprintln(stdout, string(buf))
	return 0
}

func resolvePhaseRunner(name string, req core.PhaseRequest, stderr io.Writer) (core.PhaseRunner, int) {
	runner, found, err := ResolveRunner(name, req)
	if err != nil {
		fmt.Fprintf(stderr, "evolve phase: %s: %v\n", name, err)
		return nil, 1
	}
	if !found {
		fmt.Fprintln(stderr, FormatUnknownPhaseError("evolve phase", name, req.ProjectRoot))
		return nil, 10
	}
	return runner, 0
}

func phaseRequest(flags []string, stdin io.Reader, stderr io.Writer) (core.PhaseRequest, int) {
	fs := flag.NewFlagSet("phase", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cycle := fs.Int("cycle", 0, "derive the request from .evolve/runs/cycle-N/cycle-state.json instead of stdin")
	projectRoot := fs.String("project-root", "", "project root for --cycle (default EVOLVE_PROJECT_ROOT or cwd)")
	if err := fs.Parse(flags); err != nil {
		return core.PhaseRequest{}, exitCycleUsage
	}
	if *cycle != 0 {
		req, err := RequestForCycle(*projectRoot, *cycle, stdin)
		if err != nil {
			fmt.Fprintf(stderr, "evolve phase: %v\n", err)
			return core.PhaseRequest{}, CycleRequestExitCode(err)
		}
		return req, 0
	}
	var req core.PhaseRequest
	if err := json.NewDecoder(stdin).Decode(&req); err != nil {
		fmt.Fprintf(stderr, "evolve phase: parse stdin JSON: %v\n", err)
		return core.PhaseRequest{}, 11
	}
	return req, 0
}

func (c phaseCommand) installRouter(projectRoot string) error {
	if c.install == nil {
		return nil
	}
	return c.install(projectRoot)
}
