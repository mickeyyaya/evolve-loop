package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/setup"
)

func runSetup(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "evolve setup: missing subcommand (detect|recommend|latest|apply|complete)")
		return 10
	}
	switch args[0] {
	case "detect":
		return runSetupDetect(args[1:], stdout, stderr)
	case "recommend":
		return runSetupRecommend(args[1:], stdout, stderr)
	case "latest":
		return runSetupLatest(args[1:], stdout, stderr)
	case "apply":
		return runSetupApply(args[1:], stdout, stderr)
	case "complete":
		return runSetupComplete(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve setup: unknown subcommand %q\n", args[0])
		return 10
	}
}

func setupRoots(projectRootFlag, evolveDirFlag string, stderr io.Writer) (project, plugin, evolveDir, adapters string) {
	project = projectRootFlag
	if project == "" {
		project = os.Getenv("EVOLVE_PROJECT_ROOT")
	}
	if project == "" {
		if cwd, err := os.Getwd(); err == nil {
			project = cwd
		} else {
			// Surfaced rather than silently falling through to a relative path.
			fmt.Fprintf(stderr, "[setup] WARN: could not determine cwd (%v); marker/state paths may be relative\n", err)
		}
	}
	project = paths.AbsoluteRoot("--project-root", project, func(m string) {
		fmt.Fprintf(stderr, "[setup] WARN: %s\n", m)
	})
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
	var evolveDirFlag, projectRootFlag string
	fs.BoolVar(&asJSON, "json", false, "emit the digest as JSON (default human table)")
	fs.StringVar(&evolveDirFlag, "evolve-dir", "", "path to .evolve/ (default <project>/.evolve)")
	fs.StringVar(&projectRootFlag, "project-root", "", "project root (default $EVOLVE_PROJECT_ROOT or cwd)")
	// No positional args, so parsed directly: reorderArgs would let a
	// space-separated flag value swallow the next flag (see cmd_phase_verify.go).
	if err := fs.Parse(args); err != nil {
		return 10
	}
	project, plugin, evolveDir, adapters := setupRoots(projectRootFlag, evolveDirFlag, stderr)
	rep := setup.Detect(context.Background(), setup.DetectOptions{
		ProjectRoot: project, EvolveDir: evolveDir, PluginRoot: plugin, AdaptersDir: adapters,
		Router: detectRouter(project, stderr),
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

func runSetupRecommend(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve setup recommend", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var asJSON bool
	var evolveDirFlag, projectRootFlag string
	fs.BoolVar(&asJSON, "json", false, "emit presets as JSON (default human table)")
	fs.StringVar(&evolveDirFlag, "evolve-dir", "", "path to .evolve/ (default <project>/.evolve)")
	fs.StringVar(&projectRootFlag, "project-root", "", "project root (default $EVOLVE_PROJECT_ROOT or cwd)")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	project, plugin, evolveDir, adapters := setupRoots(projectRootFlag, evolveDirFlag, stderr)
	rep := setup.Detect(context.Background(), setup.DetectOptions{
		ProjectRoot: project, EvolveDir: evolveDir, PluginRoot: plugin, AdaptersDir: adapters,
	})
	cfg, err := setup.LoadPresets(evolveDir)
	if err != nil {
		fmt.Fprintf(stderr, "evolve setup recommend: %v\n", err)
		return 1
	}
	rr := setup.Recommend(rep, cfg)
	if asJSON {
		buf, err := json.MarshalIndent(rr, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "evolve setup recommend: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "%s\n", buf)
		return 0
	}
	printRecommendHuman(stdout, rr)
	return 0
}

func printRecommendHuman(w io.Writer, rr setup.RecommendReport) {
	fmt.Fprintf(w, "Available families: %v  (cross-family OK: %v)\n", rr.AvailableFamilies, rr.CrossFamilyOK)
	for _, p := range rr.Presets {
		tag := ""
		if p.Name == rr.Default {
			tag = "  ← recommended"
		}
		if p.Degraded {
			tag += "  [DEGRADED — not all phases satisfiable]"
		}
		fmt.Fprintf(w, "\n• %s%s\n  %s\n", p.Name, tag, p.Description)
		for _, a := range p.Assignments {
			line := fmt.Sprintf("    %-14s %-7s %-9s", a.Role, a.CLI, a.Tier)
			if a.Model != "" {
				line += " " + a.Model
			}
			if a.Warning != "" {
				line += "  ⚠ " + a.Warning
			}
			fmt.Fprintln(w, line)
		}
	}
	fmt.Fprintf(w, "\nApply with:  evolve setup apply --preset %s\n", rr.Default)
}

func runSetupApply(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve setup apply", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var preset, evolveDirFlag, projectRootFlag string
	var dryRun bool
	fs.StringVar(&preset, "preset", "", "preset name to apply (required): see `evolve setup recommend`")
	fs.BoolVar(&dryRun, "dry-run", false, "print the merged policy.json to stdout, write nothing")
	fs.StringVar(&evolveDirFlag, "evolve-dir", "", "path to .evolve/ (default <project>/.evolve)")
	fs.StringVar(&projectRootFlag, "project-root", "", "project root (default $EVOLVE_PROJECT_ROOT or cwd)")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	if preset == "" {
		fmt.Fprintln(stderr, "evolve setup apply: --preset is required (see `evolve setup recommend`)")
		return 10
	}
	project, plugin, evolveDir, adapters := setupRoots(projectRootFlag, evolveDirFlag, stderr)
	rep := setup.Detect(context.Background(), setup.DetectOptions{
		ProjectRoot: project, EvolveDir: evolveDir, PluginRoot: plugin, AdaptersDir: adapters,
	})
	cfg, err := setup.LoadPresets(evolveDir)
	if err != nil {
		fmt.Fprintf(stderr, "evolve setup apply: %v\n", err)
		return 1
	}
	policyPath := filepath.Join(evolveDir, "policy.json")
	existing, rerr := os.ReadFile(policyPath)
	if rerr != nil && !os.IsNotExist(rerr) {
		fmt.Fprintf(stderr, "evolve setup apply: reading %s: %v\n", policyPath, rerr)
		return 1
	}
	profLoader := profiles.NewFromDir(filepath.Join(evolveDir, "profiles"))
	out, err := setup.Apply(rep, cfg, preset, existing, profLoader)
	if err != nil {
		fmt.Fprintf(stderr, "evolve setup apply: %v\n", err)
		return 1
	}
	if dryRun {
		fmt.Fprintf(stdout, "%s", out)
		return 0
	}
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "evolve setup apply: mkdir: %v\n", err)
		return 1
	}
	tmp := fmt.Sprintf("%s.tmp.%d", policyPath, os.Getpid())
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		fmt.Fprintf(stderr, "evolve setup apply: write temp: %v\n", err)
		return 1
	}
	defer func() { _ = os.Remove(tmp) }()
	if err := os.Rename(tmp, policyPath); err != nil {
		fmt.Fprintf(stderr, "evolve setup apply: atomic rename: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "[setup apply] wrote preset %q to %s\n", preset, policyPath)
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
		if len(c.TierModels) > 0 {
			fmt.Fprintf(w, "          models: fast=%s  balanced=%s  deep=%s\n",
				c.TierModels["fast"], c.TierModels["balanced"], c.TierModels["deep"])
		}
	}
	if rep.PolicyError != "" {
		fmt.Fprintf(w, "\n⚠ policy.json malformed (pins ignored): %s\n", rep.PolicyError)
	}
	fmt.Fprintln(w, "\nPer-phase routing (current):")
	for _, p := range rep.Phases {
		fmt.Fprintf(w, "  %-14s %-8s %-8s (%s)  envelope:[%s..%s]\n",
			p.Role, p.CurrentCLI, p.CurrentTier, p.Source, p.Envelope.Min, p.Envelope.Max)
		if p.PinViolation != "" {
			fmt.Fprintf(w, "      ✗ pin violation: %s\n", p.PinViolation)
		}
	}
	if rep.SetupCompletedAt == "" {
		fmt.Fprintln(w, "\nSetup: NOT yet completed (run /setup).")
	} else {
		fmt.Fprintf(w, "\nSetup: completed at %s (v%d).\n", rep.SetupCompletedAt, rep.SetupVersion)
	}
}

// Reads the marker directly, bypassing core.State's lossy round-trip.
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
	var evolveDirFlag, projectRootFlag string
	fs.StringVar(&evolveDirFlag, "evolve-dir", "", "path to .evolve/ (default <project>/.evolve)")
	fs.StringVar(&projectRootFlag, "project-root", "", "project root (default $EVOLVE_PROJECT_ROOT or cwd)")
	// No positional args; parsed directly, as in runSetupDetect.
	if err := fs.Parse(args); err != nil {
		return 10
	}
	_, _, evolveDir, _ := setupRoots(projectRootFlag, evolveDirFlag, stderr)
	stamp, err := setup.Complete(setup.CompleteOptions{EvolveDir: evolveDir})
	if err != nil {
		fmt.Fprintf(stderr, "evolve setup complete: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "[setup complete] marker stamped at %s (v%d)\n", stamp, setup.Version)
	return 0
}
