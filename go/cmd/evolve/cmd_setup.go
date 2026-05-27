package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/setup"
)

// runSetup implements `evolve setup <subcommand>` — the deterministic core
// behind the in-session /setup skill. Subcommands:
//
//	detect   [--json]                         onboarding digest (CLIs + per-phase)
//	validate [--config P] [--strict] [--json] clamp llm_config against the floor
//	complete                                  stamp the first-run marker
//
// Exit codes: 0 OK, 2 validate found error-severity violation, 10 bad args,
// 1 runtime error.
func runSetup(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "evolve setup: missing subcommand (detect|validate|complete)")
		return 10
	}
	switch args[0] {
	case "detect":
		return runSetupDetect(args[1:], stdout, stderr)
	case "validate":
		return runSetupValidate(args[1:], stdout, stderr)
	case "complete":
		return runSetupComplete(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve setup: unknown subcommand %q\n", args[0])
		return 10
	}
}

// setupRoots resolves project/plugin/evolve/adapters dirs from env + cwd,
// matching the convention used across the cmd package.
func setupRoots(evolveDirFlag string) (project, plugin, evolveDir, adapters string) {
	project = os.Getenv("EVOLVE_PROJECT_ROOT")
	if project == "" {
		project, _ = os.Getwd()
	}
	plugin = os.Getenv("EVOLVE_PLUGIN_ROOT")
	if plugin == "" {
		plugin = project
	}
	evolveDir = evolveDirFlag
	if evolveDir == "" {
		evolveDir = filepath.Join(project, ".evolve")
	}
	adapters = filepath.Join(plugin, "adapters")
	return
}

func runSetupDetect(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve setup detect", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var asJSON bool
	var evolveDirFlag string
	fs.BoolVar(&asJSON, "json", false, "emit the digest as JSON (default human table)")
	fs.StringVar(&evolveDirFlag, "evolve-dir", "", "path to .evolve/ (default <project>/.evolve)")
	if err := fs.Parse(reorderArgs(args)); err != nil {
		return 10
	}
	project, plugin, evolveDir, adapters := setupRoots(evolveDirFlag)
	rep := setup.Detect(context.Background(), setup.DetectOptions{
		ProjectRoot: project, EvolveDir: evolveDir, PluginRoot: plugin, AdaptersDir: adapters,
	})
	if asJSON {
		buf, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "evolve setup detect: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "%s\n", buf)
		return 0
	}
	printDetectHuman(stdout, rep)
	return 0
}

func printDetectHuman(w io.Writer, rep setup.DetectReport) {
	fmt.Fprintln(w, "Detected LLM CLIs:")
	for _, c := range rep.CLIs {
		bin := "✗ binary"
		if c.BinaryPresent {
			bin = "✓ binary"
		}
		auth := c.AuthMode
		if c.AuthConfigured {
			auth = "✓ " + c.AuthMode
		}
		fmt.Fprintf(w, "  %-7s %-9s %-22s tier:%-9s %s\n", c.CLI, bin, auth, c.CapabilityTier, c.Verdict)
	}
	fmt.Fprintln(w, "\nPer-phase routing (current):")
	for _, p := range rep.Phases {
		model := p.CurrentModel
		if model == "" {
			model = p.CurrentTier
		}
		fmt.Fprintf(w, "  %-14s %-8s %-8s (%s)  envelope:[%s..%s]\n",
			p.Role, p.CurrentCLI, model, p.Source, p.Envelope.Min, p.Envelope.Max)
	}
	if rep.SetupCompletedAt == "" {
		fmt.Fprintln(w, "\nSetup: NOT yet completed (run /setup).")
	} else {
		fmt.Fprintf(w, "\nSetup: completed at %s (v%d).\n", rep.SetupCompletedAt, rep.SetupVersion)
	}
}

func runSetupValidate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve setup validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		configPath    string
		evolveDirFlag string
		strict        bool
		asJSON        bool
	)
	fs.StringVar(&configPath, "config", "", "llm_config.json path (default <evolve-dir>/llm_config.json)")
	fs.StringVar(&evolveDirFlag, "evolve-dir", "", "path to .evolve/ (default <project>/.evolve)")
	fs.BoolVar(&strict, "strict", false, "treat the builder≠auditor cross-family check as an error (default warn)")
	fs.BoolVar(&asJSON, "json", false, "emit the report as JSON")
	if err := fs.Parse(reorderArgs(args)); err != nil {
		return 10
	}
	_, _, evolveDir, _ := setupRoots(evolveDirFlag)
	if configPath == "" {
		configPath = filepath.Join(evolveDir, "llm_config.json")
	}
	rep, err := setup.Validate(setup.ValidateOptions{
		ConfigPath: configPath, EvolveDir: evolveDir, Strict: strict,
	})
	if err != nil {
		fmt.Fprintf(stderr, "evolve setup validate: %v\n", err)
		return 1
	}
	if asJSON {
		buf, _ := json.MarshalIndent(rep, "", "  ")
		fmt.Fprintf(stdout, "%s\n", buf)
	} else {
		for _, v := range rep.Violations {
			fmt.Fprintf(stdout, "  [%s] %s: %s\n", v.Severity, v.Role, v.Message)
		}
		if rep.OK {
			fmt.Fprintf(stdout, "[setup validate] OK (%s)\n", configPath)
		} else {
			fmt.Fprintf(stdout, "[setup validate] FAIL — error-severity violations present (%s)\n", configPath)
		}
	}
	if !rep.OK {
		return 2
	}
	return 0
}

// maybePrintSetupNudge prints a one-line, non-blocking first-run hint when the
// onboarding marker (state.SetupCompletedAt) is absent. Best-effort and never
// blocks the loop — defaults work without setup. Reads the marker directly to
// stay decoupled from the (lossy-view) core.State round-trip.
func maybePrintSetupNudge(stderr io.Writer, evolveDir string) {
	if b, err := os.ReadFile(filepath.Join(evolveDir, "state.json")); err == nil {
		var m struct {
			SetupCompletedAt string `json:"setupCompletedAt"`
		}
		if json.Unmarshal(b, &m) == nil && m.SetupCompletedAt != "" {
			return // already onboarded
		}
	}
	fmt.Fprintln(stderr, "[setup] First run — run /setup to pick per-phase models + learn the pipeline (using defaults for now).")
}

func runSetupComplete(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve setup complete", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var evolveDirFlag string
	fs.StringVar(&evolveDirFlag, "evolve-dir", "", "path to .evolve/ (default <project>/.evolve)")
	if err := fs.Parse(reorderArgs(args)); err != nil {
		return 10
	}
	_, _, evolveDir, _ := setupRoots(evolveDirFlag)
	stamp, err := setup.Complete(setup.CompleteOptions{EvolveDir: evolveDir})
	if err != nil {
		fmt.Fprintf(stderr, "evolve setup complete: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "[setup complete] marker stamped at %s (v%d)\n", stamp, setup.Version)
	return 0
}
