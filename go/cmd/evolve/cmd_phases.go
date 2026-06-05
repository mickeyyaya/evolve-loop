// cmd_phases.go implements `evolve phases <list|validate|add>` — the operator
// surface for the user-definable-phases framework. It is READ-ONLY except for
// `add`, which scaffolds a new .evolve/phases/<name>/ skeleton. Distinct from
// `evolve phase` (run one phase in-process) and `evolve phase-order`.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// runPhases dispatches the phases subcommands. Exit codes: 0 ok, 2 validation
// failure, 10 usage error, 1 I/O error.
func runPhases(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: evolve phases <list|validate [name]|add <name>|create --spec <file|->>")
		return 10
	}
	project := envOrCwd("EVOLVE_PROJECT_ROOT")
	switch args[0] {
	case "list":
		return phasesList(project, stdout, stderr)
	case "validate":
		return phasesValidate(project, args[1:], stdout, stderr)
	case "add":
		return phasesAdd(project, args[1:], stdout, stderr)
	case "create":
		return phasesCreate(project, args[1:], stdin, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown subcommand %q (want list|validate|add|create)\n", args[0])
		return 10
	}
}

// mergedCatalog loads the built-in registry and overlays user phases.
func mergedCatalog(project string) (phasespec.Catalog, []string, error) {
	registryPath := filepath.Join(project, "docs", "architecture", "phase-registry.json")
	builtin, err := phasespec.Load(registryPath)
	if err != nil {
		return phasespec.Catalog{}, nil, err
	}
	user, warns := phasespec.DiscoverUserSpecs(filepath.Join(project, ".evolve", "phases"))
	merged, mWarns := builtin.Merge(user)
	return merged, append(warns, mWarns...), nil
}

func phasesList(project string, stdout, stderr io.Writer) int {
	cat, warns, err := mergedCatalog(project)
	if err != nil {
		fmt.Fprintf(stderr, "load phase catalog: %v\n", err)
		return 1
	}
	for _, w := range warns {
		fmt.Fprintln(stderr, "WARN:", w)
	}
	tw := tabwriter.NewWriter(stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tKIND\tOPTIONAL\tSOURCE")
	for _, s := range cat.All() {
		source := "builtin"
		if cat.IsUser(s.Name) {
			source = "user"
		}
		fmt.Fprintf(tw, "%s\t%s\t%t\t%s\n", s.Name, s.KindOrDefault(), s.Optional, source)
	}
	if err := tw.Flush(); err != nil {
		fmt.Fprintf(stderr, "write: %v\n", err)
		return 1
	}
	return 0
}

// phasesValidate validates operator-authored phases. Discovery warnings go to
// stderr; the per-phase OK/FAIL verdicts + violations go to stdout (the
// machine-readable result). Exit 2 if any phase has violations.
func phasesValidate(project string, args []string, stdout, stderr io.Writer) int {
	user, warns := phasespec.DiscoverUserSpecs(filepath.Join(project, ".evolve", "phases"))
	for _, w := range warns {
		fmt.Fprintln(stderr, "WARN:", w)
	}
	if len(args) > 0 {
		user = filterByName(user, args[0])
		if len(user) == 0 {
			fmt.Fprintf(stderr, "no user phase named %q under .evolve/phases/\n", args[0])
			return 10
		}
	}
	if len(user) == 0 {
		fmt.Fprintln(stdout, "no user phases to validate")
		return 0
	}
	failed := false
	for _, s := range user {
		violations := phasespec.ValidateUserSpec(s)
		if len(violations) == 0 {
			fmt.Fprintf(stdout, "OK    %s\n", s.Name)
			continue
		}
		failed = true
		fmt.Fprintf(stdout, "FAIL  %s\n", s.Name)
		for _, v := range violations {
			fmt.Fprintf(stdout, "        - %s\n", v)
		}
	}
	if failed {
		return 2
	}
	return 0
}

func filterByName(specs []phasespec.PhaseSpec, name string) []phasespec.PhaseSpec {
	for _, s := range specs {
		if s.Name == name {
			return []phasespec.PhaseSpec{s}
		}
	}
	return nil
}

func phasesAdd(project string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: evolve phases add <name>")
		return 10
	}
	name := args[0]
	if v := phasespec.ValidateUserSpec(phasespec.PhaseSpec{Name: name, Optional: true}); len(v) > 0 {
		fmt.Fprintf(stderr, "invalid phase name %q: %s\n", name, v[0])
		return 10
	}
	dir := filepath.Join(project, ".evolve", "phases", name)
	if _, err := os.Stat(dir); err == nil {
		fmt.Fprintf(stderr, "phase %q already exists at %s\n", name, dir)
		return 1
	} else if !os.IsNotExist(err) {
		fmt.Fprintf(stderr, "stat %s: %v\n", dir, err)
		return 1
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(stderr, "mkdir %s: %v\n", dir, err)
		return 1
	}
	scaffoldFiles := []struct {
		name string
		body string
	}{
		{"phase.json", scaffoldPhaseJSON(name)},
		{"agent.md", scaffoldAgentMD(name)},
		{"profile.json", scaffoldProfileJSON(name)},
	}
	for _, f := range scaffoldFiles {
		if err := os.WriteFile(filepath.Join(dir, f.name), []byte(f.body), 0o644); err != nil {
			fmt.Fprintf(stderr, "write %s: %v\n", f.name, err)
			return 1
		}
	}
	fmt.Fprintf(stdout, "scaffolded %s/{phase.json,agent.md,profile.json}\n", dir)
	fmt.Fprintf(stdout, "next: edit the prompt in agent.md, then `evolve phases validate %s`\n", name)
	return 0
}

func scaffoldPhaseJSON(name string) string {
	return fmt.Sprintf(`{
  "name": %q,
  "kind": "llm",
  "optional": true,
  "agent": "evolve-%s",
  "model": "auto",
  "writes_source": false,
  "inputs":  { "files": [], "signals": [] },
  "outputs": { "files": [%q], "signals": [] },
  "prompt_context": ["goal"],
  "classify": { "require_sections": ["## Findings"], "fail_if_empty": true, "verdict_on_pass": "PASS" },
  "routing": { "insert_when": [ { "field": "build.files_touched", "op": "gt", "value": 0 } ] }
}
`, name, name, name+"-report.md")
}

func scaffoldAgentMD(name string) string {
	return fmt.Sprintf(`---
name: evolve-%s
description: <one-line description of what this phase does>
---

# %s phase

You are the **%s** phase of the evolve-loop pipeline. <Describe the task.>

Write your report to the contracted artifact with a "## Findings" section.
`, name, name, name)
}

func scaffoldProfileJSON(name string) string {
	return fmt.Sprintf(`{
  "name": %q,
  "role": %q,
  "cli": "claude-tmux",
  "model_tier_default": "sonnet",
  "allowed_tools": ["Read", "Grep", "Glob", "Bash", "Write"],
  "parallel_eligible": true,
  "sandbox": { "enabled": true, "read_only_repo": true }
}
`, name, name)
}
