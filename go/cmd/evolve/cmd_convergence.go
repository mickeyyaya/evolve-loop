package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const (
	convergenceUsage     = "usage: evolve convergence decide --input <rounds.json> [--json]"
	exitConvergenceBad   = 10
	exitConvergenceRead  = 2
	exitConvergenceWrite = 1
)

var errConvergenceUsage = errors.New(convergenceUsage)

type convergenceRequest struct {
	input  string
	asJSON bool
}

func runConvergence(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "decide" {
		fmt.Fprintln(stderr, "evolve convergence: "+convergenceUsage)
		return exitConvergenceBad
	}
	req, err := parseConvergenceArgs(args[1:], stderr)
	if err != nil {
		fmt.Fprintf(stderr, "evolve convergence decide: %v\n", err)
		return exitConvergenceBad
	}
	cfg, err := loadConvergenceConfig(stderr)
	if err != nil {
		fmt.Fprintf(stderr, "convergence decide: %v\n", err)
		return exitConvergenceRead
	}
	in, err := readConvergenceInput(req.input, cfg)
	if err != nil {
		fmt.Fprintf(stderr, "convergence decide: %v\n", err)
		return exitConvergenceBad
	}
	d := convergence.Decide(in)
	if req.asJSON {
		return encodeConvergenceDecision(d, stdout, stderr)
	}
	fmt.Fprint(stdout, convergenceText(d, in))
	return 0
}

func parseConvergenceArgs(args []string, stderr io.Writer) (convergenceRequest, error) {
	var req convergenceRequest
	fs := flag.NewFlagSet("convergence decide", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&req.input, "input", "", "the rounds.json the decision reads (schema: docs/operations/runtime-reference.md)")
	fs.BoolVar(&req.asJSON, "json", false, "print the decision as JSON")
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 || req.input == "" {
		return req, errConvergenceUsage
	}
	return req, nil
}

func loadConvergenceConfig(stderr io.Writer) (policy.ConvergenceConfig, error) {
	root := envOrCwd("EVOLVE_PROJECT_ROOT")
	pol, err := policy.Load(filepath.Join(root, ".evolve", "policy.json"))
	if err != nil {
		return policy.ConvergenceConfig{}, err
	}
	cfg := pol.ConvergenceConfig()
	for _, warning := range cfg.Warnings {
		fmt.Fprintf(stderr, "convergence decide: WARN %s\n", warning)
	}
	return cfg, nil
}

func readConvergenceInput(path string, cfg policy.ConvergenceConfig) (convergence.Input, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return convergence.Input{}, err
	}
	in, err := convergence.Parse(data)
	if err != nil {
		return convergence.Input{}, fmt.Errorf("%s: %w", path, err)
	}
	if in.Loop != convergence.LoopConsoleLane {
		return convergence.Input{}, fmt.Errorf("%s: loop %q: the verb decides loop %q only; the pipeline loops call convergence.Decide in process "+
			"(audit-repair and explanation-reauthor: V5, code-review: V6, cycle: V10, inbox-item: V11, ship-recovery: V12)", path, in.Loop, convergence.LoopConsoleLane)
	}
	in.Config = cfg
	if in.Headroom, err = tierTables(in.Fixer.Family, in.Judge.Family); err != nil {
		return convergence.Input{}, err
	}
	if err := in.Validate(); err != nil {
		return convergence.Input{}, fmt.Errorf("%s: %w", path, err)
	}
	return in, nil
}

func tierTables(families ...string) (convergence.HeadroomTable, error) {
	table := convergence.HeadroomTable{}
	for _, family := range families {
		if family == "" || table[family] != nil {
			continue
		}
		m, err := bridge.LoadManifest(family)
		if err != nil {
			return nil, fmt.Errorf("tier table for family %q: %w", family, err)
		}
		if len(m.ModelTierMap) == 0 {
			return nil, fmt.Errorf("tier table for family %q: its driver manifest declares no model_tier_map", family)
		}
		tiers := map[string]convergence.ModelEffort{}
		for tier, model := range m.ModelTierMap {
			tiers[tier] = convergence.ModelEffort{Model: model}
		}
		table[family] = tiers
	}
	return table, nil
}

func encodeConvergenceDecision(d convergence.Decision, stdout, stderr io.Writer) int {
	enc := json.NewEncoder(stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")
	if err := enc.Encode(d); err != nil {
		fmt.Fprintf(stderr, "convergence decide: encode: %v\n", err)
		return exitConvergenceWrite
	}
	return 0
}

func convergenceText(d convergence.Decision, in convergence.Input) string {
	var b strings.Builder
	fmt.Fprintf(&b, "convergence: %s after round %d: %s (rung %d, bar %s)\n", in.Loop, in.Round, d.Action, d.Rung, d.BlockingBar)
	fmt.Fprintf(&b, "land round: %d\n", d.LandRound)
	writeConvergenceLine(&b, "verify-only", yesWhen(d.VerifyOnly))
	writeConvergenceLine(&b, "fresh context", yesWhen(d.FreshContext))
	writeConvergenceLine(&b, "fixer raise", raiseText(d.FixerRaise))
	writeConvergenceLine(&b, "judge raise", raiseText(d.JudgeRaise))
	writeConvergenceLine(&b, "split", d.SplitComponent)
	writeConvergenceLine(&b, "defer", strings.Join(d.Defer, ", "))
	writeConvergenceLine(&b, "file", strings.Join(d.File, ", "))
	writeConvergenceLine(&b, "redesign", strings.Join(d.Redesign, ", "))
	b.WriteString("reasons:\n")
	for _, reason := range d.Reasons {
		b.WriteString("  " + reason + "\n")
	}
	return b.String()
}

func writeConvergenceLine(b *strings.Builder, label, value string) {
	if value != "" {
		fmt.Fprintf(b, "%s: %s\n", label, value)
	}
}

func yesWhen(on bool) string {
	if on {
		return "yes"
	}
	return ""
}

func raiseText(t convergence.TierEffort) string {
	if t == (convergence.TierEffort{}) {
		return ""
	}
	return fmt.Sprintf("%s %s (%s)", t.Family, t.Tier, strings.TrimSpace(t.Model+" "+t.Effort))
}
