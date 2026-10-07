package phasecmd

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecoherence"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// RunPhases implements `evolve phases`; it exits 0 ok, 1 I/O error, 2 validation failure, 10 usage error.
func RunPhases(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve phases", flag.ContinueOnError)
	fs.SetOutput(stderr)
	profileDir := fs.String("profile-dir", "", "profiles directory (default: <project>/.evolve/profiles)")
	personaOverride := fs.String("persona-override", "", "substitute persona file (<path>:<name>)")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	overrides, err := parsePersonaOverride(*personaOverride)
	if err != nil {
		fmt.Fprintf(stderr, "evolve phases: %v\n", err)
		return 10
	}
	remaining := fs.Args()
	if len(remaining) == 0 {
		fmt.Fprintf(stderr, "usage: evolve phases [--profile-dir <dir>] [--persona-override <path>:<name>] <%s>\n", phasesUsage())
		return 10
	}
	sub, ok := lookupPhasesSubcommand(remaining[0])
	if !ok {
		fmt.Fprintf(stderr, "unknown subcommand %q (want %s)\n", remaining[0], strings.Join(phasesSubcommandNames(), "|"))
		return 10
	}
	return sub.run(phasesCall{
		project:    cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT"),
		profileDir: *profileDir,
		overrides:  overrides,
		args:       remaining[1:],
		stdin:      stdin,
		stdout:     stdout,
		stderr:     stderr,
	})
}

type phasesCall struct {
	project    string
	profileDir string
	overrides  map[string]string
	args       []string
	stdin      io.Reader
	stdout     io.Writer
	stderr     io.Writer
}

type phasesSubcommand struct {
	name  string
	usage string
	run   func(phasesCall) int
}

var phasesSubcommands = []phasesSubcommand{
	{"list", "list", func(c phasesCall) int { return phasesList(c.project, c.stdout, c.stderr) }},
	{"validate", "validate [name] [--strict-provenance]", func(c phasesCall) int {
		return phasesValidate(c.project, c.profileDir, c.args, c.stdout, c.stderr)
	}},
	{"check-coherence", "check-coherence [--strict]", func(c phasesCall) int {
		return phasesCoherence(c, "check-coherence", phasecoherence.Check)
	}},
	{"check-artifact-coherence", "check-artifact-coherence [--strict]", func(c phasesCall) int {
		return phasesCoherence(c, "check-artifact-coherence", phasecoherence.CheckArtifactNames)
	}},
	{"check-provenance", "check-provenance --cycle N [--json]", phasesCheckProvenance},
	{"add", "add <name>", func(c phasesCall) int { return phasesAdd(c.project, c.args, c.stdout, c.stderr) }},
	{"create", "create --spec <file|->", func(c phasesCall) int {
		return phasesCreate(c.project, c.args, c.stdin, c.stdout, c.stderr)
	}},
}

func lookupPhasesSubcommand(name string) (phasesSubcommand, bool) {
	for _, sub := range phasesSubcommands {
		if sub.name == name {
			return sub, true
		}
	}
	return phasesSubcommand{}, false
}

func phasesSubcommandNames() []string {
	names := make([]string, 0, len(phasesSubcommands))
	for _, sub := range phasesSubcommands {
		names = append(names, sub.name)
	}
	return names
}

func phasesUsage() string {
	usages := make([]string, 0, len(phasesSubcommands))
	for _, sub := range phasesSubcommands {
		usages = append(usages, sub.usage)
	}
	return strings.Join(usages, "|")
}

func parsePersonaOverride(value string) (map[string]string, error) {
	overrides := make(map[string]string)
	if value == "" {
		return overrides, nil
	}
	path, name, ok := strings.Cut(value, ":")
	if !ok || path == "" || name == "" {
		return nil, fmt.Errorf("--persona-override %q: want <path>:<name>", value)
	}
	overrides[name] = path
	return overrides, nil
}

func mergedCatalog(project string) (phasespec.Catalog, map[string]string, []string, error) {
	return phasespec.MergedCatalog(project)
}

func phasesList(project string, stdout, stderr io.Writer) int {
	cat, sources, warns, err := mergedCatalog(project)
	if err != nil {
		fmt.Fprintf(stderr, "load phase catalog: %v\n", err)
		return 1
	}
	for _, w := range warns {
		fmt.Fprintln(stderr, "WARN:", w)
	}
	tw := tabwriter.NewWriter(stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tKIND\tOPTIONAL\tSOURCE\tROOT")
	for _, s := range cat.All() {
		source, root := "builtin", ""
		if cat.IsUser(s.Name) {
			source = "user"
			root = relTo(project, sources[s.Name])
		}
		fmt.Fprintf(tw, "%s\t%s\t%t\t%s\t%s\n", s.Name, s.KindOrDefault(), s.Optional, source, root)
	}
	if err := tw.Flush(); err != nil {
		fmt.Fprintf(stderr, "write: %v\n", err)
		return 1
	}
	return 0
}

// phasesValidate prints per-phase verdicts on stdout (machine-readable) and discovery warnings on stderr.
func phasesValidate(project, profileDir string, args []string, stdout, stderr io.Writer) int {
	strictProvenance, args := splitStrictProvenance(args)
	if profileDir == "" {
		profileDir = filepath.Join(project, ".evolve", "profiles")
	}
	pnames, provenanceFailed := warnMissingProvenance(profileDir, strictProvenance, stdout)

	roots, warns := phasespec.RootsWithWarnings(project)
	user, _, discWarns := phasespec.DiscoverUserSpecsFromRoots(roots)
	for _, w := range append(warns, discWarns...) {
		fmt.Fprintln(stderr, "WARN:", w)
	}
	if len(args) > 0 {
		user = filterByName(user, args[0])
		if len(user) == 0 {
			fmt.Fprintf(stderr, "no user phase named %q in any phase root\n", args[0])
			if provenanceFailed {
				return 2
			}
			return 10
		}
	}

	failed := false
	if len(user) > 0 {
		failed = reportUserSpecVerdicts(user, stdout)
	} else if len(args) == 0 && len(pnames) == 0 {
		fmt.Fprintln(stdout, "no user phases to validate")
	}

	if failed || provenanceFailed {
		return 2
	}
	return 0
}

func splitStrictProvenance(args []string) (strictProvenance bool, cleanArgs []string) {
	for _, arg := range args {
		if arg == "--strict-provenance" {
			strictProvenance = true
		} else {
			cleanArgs = append(cleanArgs, arg)
		}
	}
	return strictProvenance, cleanArgs
}

func warnMissingProvenance(profileDir string, strictProvenance bool, stdout io.Writer) (pnames []string, provenanceFailed bool) {
	loader := profiles.NewFromDir(profileDir)
	pnames, err := loader.List()
	if err != nil {
		return pnames, false
	}
	for _, pname := range pnames {
		p, err := loader.Get(pname)
		if err == nil && p.GeneratedFrom == "" {
			fmt.Fprintf(stdout, "WARN: profile %s missing generated_from\n", pname)
			if strictProvenance {
				provenanceFailed = true
			}
		}
	}
	return pnames, provenanceFailed
}

func reportUserSpecVerdicts(user []phasespec.PhaseSpec, stdout io.Writer) (failed bool) {
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
	return failed
}

func phasesCoherence(c phasesCall, label string, check func(phasecoherence.Options) ([]phasecoherence.Violation, error)) int {
	strict := false
	for _, arg := range c.args {
		if arg == "--strict" {
			strict = true
		}
	}
	profileDir := c.profileDir
	if profileDir == "" {
		profileDir = filepath.Join(c.project, ".evolve", "profiles")
	}
	violations, err := check(phasecoherence.Options{
		AgentsFS:   os.DirFS(c.project),
		ProfilesFS: os.DirFS(profileDir),
		Overrides:  c.overrides,
	})
	if err != nil {
		fmt.Fprintf(c.stderr, "%s: %v\n", label, err)
		return 1
	}
	for _, v := range violations {
		fmt.Fprintf(c.stdout, "%s: %s: %s\n", v.Severity, v.Persona, v.Message)
	}
	if len(violations) > 0 && strict {
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
