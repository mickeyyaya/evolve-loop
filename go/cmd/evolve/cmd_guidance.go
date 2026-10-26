package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/guidance"
)

const (
	guidanceUsage = `usage: evolve guidance build [--project-root P]
       evolve guidance check [--project-root P]
       evolve guidance index [--audience all|console|phase|reviewer|brief] [--phase NAME] [--cli NAME] [--json] [--project-root P]
build writes guidance/index.md, guidance/index.json and the L0 region of AGENTS.md (exit 0 written or current, 2 I/O, 11 bad registry);
check reports stale output, duplicates, budgets, conflicts and absent enforcers (exit 0 clean, 1 problems, 2 I/O);
index prints the active rules that match each filter (exit 0, 2 I/O, 10 usage, 11 bad registry)`
	guidancePrefix      = "evolve guidance: "
	guidanceIndexSchema = "evolve.guidance.index/1"
	exitGuidanceFailed  = 1
	exitGuidanceInput   = 11
)

var (
	guidanceRepoRoot      = resolveRepoRoot
	guidanceManifestNames = bridge.ManifestNames
	guidanceLoadManifest  = bridge.LoadManifest
)

type guidanceArgs struct {
	audience, phase, cli, projectRoot string
	json, help                        bool
}

type guidanceFilterOut struct {
	Audience string `json:"audience"`
	Phase    string `json:"phase"`
	CLI      string `json:"cli"`
	Family   string `json:"family"`
}

type guidanceIndexData struct {
	Filter  guidanceFilterOut   `json:"filter"`
	Entries []guidance.IndexRow `json:"entries"`
}

type guidanceError struct {
	Code    int    `json:"code"`
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

type guidanceEnvelope struct {
	Schema   string             `json:"schema"`
	OK       bool               `json:"ok"`
	Data     *guidanceIndexData `json:"data"`
	Error    *guidanceError     `json:"error"`
	Warnings []string           `json:"warnings"`
}

func runGuidance(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, guidanceUsage)
		return exitUsage
	}
	verb, rest := args[0], args[1:]
	a, err := parseGuidanceArgs(rest, verb == "index")
	switch {
	case verb == "--help" || verb == "-h" || (err == nil && a.help):
		fmt.Fprintln(stdout, guidanceUsage)
		return 0
	case verb != "build" && verb != "check" && verb != "index":
		fmt.Fprintf(stderr, "%sunknown subcommand %q\n%s\n", guidancePrefix, verb, guidanceUsage)
		return exitUsage
	case verb == "index":
		return guidanceIndex(a, err, stdout, stderr)
	case err != nil:
		fmt.Fprintf(stderr, "%s%v\n%s\n", guidancePrefix, err, guidanceUsage)
		return exitUsage
	}
	root, err := guidanceRepoRoot(a.projectRoot)
	if err != nil {
		return guidanceFail(stderr, err, exitIO)
	}
	if verb == "build" {
		return guidanceBuild(root, stdout, stderr)
	}
	return guidanceCheck(root, stdout, stderr)
}

func parseGuidanceArgs(args []string, withIndexFlags bool) (guidanceArgs, error) {
	var a guidanceArgs
	f := cliFlags{
		bools:  map[string]*bool{"--help": &a.help, "-h": &a.help},
		values: map[string]*string{"--project-root": &a.projectRoot},
	}
	if withIndexFlags {
		f.bools["--json"] = &a.json
		f.values["--audience"] = &a.audience
		f.values["--phase"] = &a.phase
		f.values["--cli"] = &a.cli
	}
	operands, err := f.parse(args)
	switch {
	case err != nil:
		a.json = withIndexFlags && slices.Contains(args, "--json")
		return a, err
	case len(operands) > 0:
		return a, fmt.Errorf("unexpected operand %q", operands[0])
	case a.audience != "" && !guidance.ValidAudience(a.audience):
		return a, fmt.Errorf("unknown audience %q", a.audience)
	}
	return a, nil
}

func guidanceBuild(root string, stdout, stderr io.Writer) int {
	rules, err := guidance.Load(root)
	if err != nil {
		return guidanceFail(stderr, err, guidanceLoadCode(err))
	}
	outs, err := guidance.Outputs(root, rules)
	if err != nil {
		rc := exitIO
		if errors.Is(err, guidance.ErrNoL0Region) {
			rc = exitGuidanceInput
		}
		return guidanceFail(stderr, err, rc)
	}
	written, err := guidance.Write(root, outs)
	if err != nil {
		return guidanceFail(stderr, err, exitIO)
	}
	for _, p := range written {
		fmt.Fprintf(stdout, "wrote %s\n", p)
	}
	fmt.Fprintf(stdout, "guidance build: %d rule(s), %d file(s) written; run `evolve skills generate` next\n", len(rules), len(written))
	return 0
}

func guidanceCheck(root string, stdout, stderr io.Writer) int {
	families, err := guidanceFamilies()
	if err != nil {
		return guidanceFail(stderr, err, exitIO)
	}
	problems, err := guidance.Check(root, families)
	if err != nil {
		return guidanceFail(stderr, err, exitIO)
	}
	for _, p := range problems {
		fmt.Fprintln(stderr, p)
	}
	if len(problems) > 0 {
		fmt.Fprintf(stderr, "guidance check: %d problem(s)\n", len(problems))
		return exitGuidanceFailed
	}
	fmt.Fprintln(stdout, "guidance check OK")
	return 0
}

func guidanceFail(stderr io.Writer, err error, rc int) int {
	fmt.Fprintf(stderr, "%s%v\n", guidancePrefix, err)
	return rc
}

func guidanceLoadCode(err error) int {
	if guidance.IsDecodeError(err) {
		return exitGuidanceInput
	}
	return exitIO
}

func guidanceFamilies() ([]string, error) {
	var families []string
	for _, name := range guidanceManifestNames() {
		m, err := guidanceLoadManifest(name)
		if err != nil {
			return nil, fmt.Errorf("bridge manifest %s: %w", name, err)
		}
		if m.ModelFamily != "" && !slices.Contains(families, m.ModelFamily) {
			families = append(families, m.ModelFamily)
		}
	}
	slices.Sort(families)
	return families, nil
}

func guidanceIndex(a guidanceArgs, parseErr error, stdout, stderr io.Writer) int {
	data, rc, msg := guidanceIndexRun(a, parseErr)
	if rc != 0 {
		fmt.Fprintf(stderr, "%s%s\n", guidancePrefix, msg)
	}
	if !a.json {
		if rc == 0 {
			fmt.Fprint(stdout, guidance.RowsMarkdown(data.rules, "guidance/rules/"))
		}
		return rc
	}
	env := guidanceEnvelope{Schema: guidanceIndexSchema, OK: rc == 0, Warnings: []string{}}
	if rc == 0 {
		env.Data = &guidanceIndexData{Filter: data.filter, Entries: guidance.Rows(data.rules)}
	} else {
		env.Error = &guidanceError{Code: rc, Kind: guidanceErrorKinds[rc], Message: msg}
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(env); err != nil {
		return guidanceFail(stderr, err, exitIO)
	}
	return rc
}

var guidanceErrorKinds = map[int]string{exitIO: "io", exitUsage: "usage", exitGuidanceInput: "input"}

type guidanceSelection struct {
	filter guidanceFilterOut
	rules  []guidance.Rule
}

func guidanceIndexRun(a guidanceArgs, parseErr error) (guidanceSelection, int, string) {
	if parseErr != nil {
		return guidanceSelection{}, exitUsage, parseErr.Error()
	}
	filter := guidanceFilterOut{Audience: a.audience, Phase: a.phase, CLI: a.cli}
	if a.cli != "" {
		if !slices.Contains(guidanceManifestNames(), a.cli) {
			return guidanceSelection{}, exitUsage, fmt.Sprintf("no bridge manifest for the CLI %q", a.cli)
		}
		m, err := guidanceLoadManifest(a.cli)
		if err != nil {
			return guidanceSelection{}, exitIO, err.Error()
		}
		if m.ModelFamily == "" {
			return guidanceSelection{}, exitUsage, fmt.Sprintf("the bridge manifest of the CLI %q has no model family", a.cli)
		}
		filter.Family = m.ModelFamily
	}
	root, err := guidanceRepoRoot(a.projectRoot)
	if err != nil {
		return guidanceSelection{}, exitIO, err.Error()
	}
	rules, err := guidance.Load(root)
	if err != nil {
		return guidanceSelection{}, guidanceLoadCode(err), err.Error()
	}
	selected := guidance.Select(rules, guidance.Filter{Audience: a.audience, Phase: a.phase, Family: filter.Family})
	return guidanceSelection{filter: filter, rules: selected}, 0, ""
}
