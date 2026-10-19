package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/yamlcodec"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const floorUsage = "usage: evolve floor run [--tier fmt,vet,...] [--root DIR] [--dry-run]"

const floorTailLines = 40

type floorStatus string

const (
	floorPass  floorStatus = "PASS"
	floorRed   floorStatus = "RED"
	floorError floorStatus = "ERROR"
)

type floorExecFunc func(dir, command string) (output string, code int, err error)

type floorTier struct {
	name     string
	commands func(root string) ([]string, error)
}

type floorVerdict struct {
	tier   string
	status floorStatus
	detail []string
}

func staticFloorTier(name string, commands ...string) floorTier {
	return floorTier{name: name, commands: func(string) ([]string, error) { return commands, nil }}
}

var floorTierTable = []floorTier{
	{name: "fmt", commands: floorFmtCommands},
	staticFloorTier("vet", "go vet ./...", "GOOS=darwin go vet ./..."),
	staticFloorTier("integration", "make test-integration"),
	staticFloorTier("e2e", "make test-e2e"),
	staticFloorTier("acs-durable", "go test -count=1 -tags acs ./acs/regression/..."),
	staticFloorTier("apicover", "make apicover-check"),
	staticFloorTier("cover", "make cover-strict"),
	staticFloorTier("ratchet", "go run ./cmd/evolve ratchet check --root ."),
}

type floorLintWorkflow struct {
	Jobs struct {
		Lint struct {
			Steps []struct {
				With map[string]any `yaml:"with"`
			} `yaml:"steps"`
		} `yaml:"lint"`
	} `yaml:"jobs"`
}

func floorLintGoVersion(workflowPath string) (string, error) {
	raw, err := os.ReadFile(workflowPath)
	if err != nil {
		return "", fmt.Errorf("the toolchain CI pins is unknown: %w", err)
	}
	var wf floorLintWorkflow
	if err := yamlcodec.Unmarshal(raw, &wf); err != nil {
		return "", fmt.Errorf("parse %s: %w", workflowPath, err)
	}
	for _, step := range wf.Jobs.Lint.Steps {
		if v, ok := step.With["go-version"]; ok {
			return fmt.Sprint(v), nil
		}
	}
	return "", fmt.Errorf("%s: the lint job pins no go-version", workflowPath)
}

func floorFmtCommands(root string) ([]string, error) {
	version, err := floorLintGoVersion(filepath.Join(root, ".github", "workflows", "go.yml"))
	if err != nil {
		return nil, err
	}
	if strings.Count(version, ".") == 1 {
		version += ".0"
	}
	return []string{fmt.Sprintf(`export GOTOOLCHAIN=go%s; d=$($(go env GOROOT)/bin/gofmt -d -s .); s=$?; if [ -n "$d" ]; then echo "$d"; exit 1; fi; exit $s`, version)}, nil
}

func selectFloorTiers(csv string) ([]floorTier, error) {
	if csv == "" {
		return floorTierTable, nil
	}
	var selected []floorTier
	seen := map[string]bool{}
	for _, name := range strings.Split(csv, ",") {
		name = strings.TrimSpace(name)
		tier, ok := findFloorTier(name)
		if !ok {
			return nil, fmt.Errorf("unknown tier %q (known: %s)", name, floorTierNames())
		}
		if !seen[name] {
			seen[name] = true
			selected = append(selected, tier)
		}
	}
	return selected, nil
}

func findFloorTier(name string) (floorTier, bool) {
	for _, tier := range floorTierTable {
		if tier.name == name {
			return tier, true
		}
	}
	return floorTier{}, false
}

func floorTierNames() string {
	names := make([]string, len(floorTierTable))
	for i, tier := range floorTierTable {
		names[i] = tier.name
	}
	return strings.Join(names, ", ")
}

func execFloorCommand(dir, command string) (string, int, error) {
	cmd := sysexec.Command(context.Background(), "sh", "-c", command)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), exitErr.ExitCode(), nil
	}
	return string(out), 0, err
}

func floorStepStatus(code int, err error) floorStatus {
	switch {
	case err != nil || code == 127:
		return floorError
	case code != 0:
		return floorRed
	}
	return floorPass
}

func worseFloorStatus(a, b floorStatus) floorStatus {
	rank := map[floorStatus]int{floorPass: 0, floorRed: 1, floorError: 2}
	if rank[b] > rank[a] {
		return b
	}
	return a
}

func floorTail(output string) []string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) > floorTailLines {
		lines = lines[len(lines)-floorTailLines:]
	}
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}

func runFloorTier(run floorExecFunc, root string, tier floorTier) floorVerdict {
	commands, err := tier.commands(root)
	if err != nil {
		return floorVerdict{tier: tier.name, status: floorError, detail: []string{err.Error()}}
	}
	verdict := floorVerdict{tier: tier.name, status: floorPass}
	for _, command := range commands {
		output, code, err := run(filepath.Join(root, "go"), command)
		status := floorStepStatus(code, err)
		if status == floorPass {
			continue
		}
		verdict.status = worseFloorStatus(verdict.status, status)
		verdict.detail = append(verdict.detail, "$ "+command)
		if err != nil {
			verdict.detail = append(verdict.detail, err.Error())
		}
		verdict.detail = append(verdict.detail, floorTail(output)...)
	}
	return verdict
}

func printFloorVerdict(w io.Writer, v floorVerdict) {
	fmt.Fprintf(w, "%s %s\n", v.status, v.tier)
	for _, line := range v.detail {
		fmt.Fprintf(w, "    %s\n", line)
	}
}

func floorExitCode(verdicts []floorVerdict) int {
	code := 0
	for _, v := range verdicts {
		switch v.status {
		case floorError:
			return 2
		case floorRed:
			code = 1
		}
	}
	return code
}

func planFloor(tiers []floorTier, root string, stdout, stderr io.Writer) int {
	code := 0
	for _, tier := range tiers {
		commands, err := tier.commands(root)
		if err != nil {
			fmt.Fprintf(stderr, "evolve floor: tier %s: %v\n", tier.name, err)
			code = 2
			continue
		}
		for _, command := range commands {
			fmt.Fprintf(stdout, "%s: %s\n", tier.name, command)
		}
	}
	return code
}

func runFloorTiers(run floorExecFunc, tiers []floorTier, root string, stdout io.Writer) int {
	verdicts := make([]floorVerdict, 0, len(tiers))
	for _, tier := range tiers {
		verdict := runFloorTier(run, root, tier)
		printFloorVerdict(stdout, verdict)
		verdicts = append(verdicts, verdict)
	}
	return floorExitCode(verdicts)
}

func runFloor(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return runFloorWith(execFloorCommand, args, stdout, stderr)
}

func runFloorWith(run floorExecFunc, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "run" {
		fmt.Fprintln(stderr, floorUsage)
		return 2
	}
	fs := flag.NewFlagSet("floor run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	tierCSV := fs.String("tier", "", "comma-separated tiers to run (default: every tier)")
	root := fs.String("root", ".", "repository root holding go/ and .github/workflows/")
	dryRun := fs.Bool("dry-run", false, "print each tier's commands instead of running them")
	positional, err := parseInterspersed(fs, args[1:])
	if err != nil {
		return 2
	}
	if len(positional) > 0 {
		fmt.Fprintf(stderr, "evolve floor: unexpected arguments %q\n%s\n", positional, floorUsage)
		return 2
	}
	tiers, err := selectFloorTiers(*tierCSV)
	if err != nil {
		fmt.Fprintf(stderr, "evolve floor: %v\n%s\n", err, floorUsage)
		return 2
	}
	if *dryRun {
		return planFloor(tiers, *root, stdout, stderr)
	}
	return runFloorTiers(run, tiers, *root, stdout)
}
