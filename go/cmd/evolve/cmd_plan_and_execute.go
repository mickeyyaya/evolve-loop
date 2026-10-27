package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
)

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

	raw, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "evolve plan-and-execute: read stdin: %v\n", err)
		return 1
	}
	var req core.PhaseRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		fmt.Fprintf(stderr, "evolve plan-and-execute: decode the phase request: %v\n", err)
		return 1
	}

	planPath := *planOutput
	if planPath == "" {
		if *workspace != "" {
			planPath = filepath.Join(*workspace, phase+"-plan.md")
		} else {
			planPath = filepath.Join(".", phase+"-plan.md")
		}
	}

	fmt.Fprintf(stdout, "[plan-and-execute] pass A: %s in plan mode → %s\n", phase, planPath)
	if code := runPassInMode(phase, req, "plan", stdout, stderr); code != 0 {
		fmt.Fprintf(stderr, "[plan-and-execute] pass A failed (exit=%d); aborting\n", code)
		return 1
	}

	if *skipExecute {
		fmt.Fprintln(stdout, "[plan-and-execute] --skip-execute set; pass B skipped")
		return 0
	}

	if _, err := os.Stat(planPath); err != nil {
		fmt.Fprintf(stderr, "[plan-and-execute] plan artifact missing at %s (pass A produced no plan): %v\n",
			planPath, err)
		return 11
	}

	fmt.Fprintf(stdout, "[plan-and-execute] pass B: %s in acceptEdits mode (plan ← %s)\n", phase, planPath)
	if code := runPassInMode(phase, req, "acceptEdits", stdout, stderr); code != 0 {
		fmt.Fprintf(stderr, "[plan-and-execute] pass B failed (exit=%d)\n", code)
		return 2
	}
	fmt.Fprintln(stdout, "[plan-and-execute] both passes PASS")
	return 0
}

func runPassInMode(phase string, req core.PhaseRequest, mode string, stdout, stderr io.Writer) int {
	req.PermissionMode = mode
	body, err := json.Marshal(req)
	if err != nil {
		fmt.Fprintf(stderr, "evolve plan-and-execute: encode the phase request: %v\n", err)
		return 1
	}
	return runPhase([]string{phase}, bytes.NewReader(body), stdout, stderr)
}

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
