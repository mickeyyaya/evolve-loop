package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/acsrunner"
	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
)

// resolveACSSuiteRoot reads <evolveDir>/runs/cycle-<cycle>/cycle-state.json and
// returns its active_worktree value, or "" when absent, malformed, or empty.
func resolveACSSuiteRoot(evolveDir string, cycle int) string {
	path := filepath.Join(evolveDir, "runs", fmt.Sprintf("cycle-%d", cycle), "cycle-state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var state struct {
		ActiveWorktree string `json:"active_worktree"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return ""
	}
	return state.ActiveWorktree
}

// mainProjectRoot resolves the MAIN project root from dir, following a git worktree
// back to its main checkout via --git-common-dir (whose parent is the main root).
// Predicates read `.evolve/` runtime data from there, so the suite must point
// EVOLVE_PROJECT_ROOT at it even when invoked from a worktree (issue #12). Falls
// back to dir when git resolution is unavailable.
func mainProjectRoot(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil {
		return dir
	}
	gitCommon := strings.TrimSpace(string(out))
	if gitCommon == "" {
		return dir
	}
	return filepath.Dir(gitCommon)
}

// runACS implements `evolve acs <subcommand>`. Subcommands:
//
//	run --cycle N <pkg>   execute go test -json on <pkg>, write
//	                       <evolve-dir>/runs/cycle-N/acs-verdict.json
//	suite --cycle N       execute the bash EGPS predicate suite (cycle-N +
//	                       regression-suite + red-team), write acs-verdict.json
func runACS(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "evolve acs: missing subcommand (try: run|suite)")
		return 10
	}
	switch args[0] {
	case "run":
		return runACSRun(args[1:], stdout, stderr)
	case "suite":
		return runACSSuite(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve acs: unknown subcommand %q\n", args[0])
		return 10
	}
}

// runACSSuite implements `evolve acs suite --cycle N [--root .] [--evolve-dir .evolve]`.
// It is the deterministic host-side replacement for the deleted bash
// run-acs-suite.sh (ADR-0025): globs + runs the bash predicate suite and writes
// acs-verdict.json. Exit 2 when any predicate is RED, 0 when all green.
func runACSSuite(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve acs suite", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		cycle     int
		root      string
		evolveDir string
		writeJSON bool
	)
	fs.IntVar(&cycle, "cycle", 0, "cycle number (required)")
	fs.StringVar(&root, "root", ".", "repo root containing acs/")
	fs.StringVar(&evolveDir, "evolve-dir", ".evolve", "path to .evolve/ state directory")
	fs.BoolVar(&writeJSON, "json", true, "write acs-verdict.json (default true)")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	if cycle <= 0 {
		fmt.Fprintln(stderr, "evolve acs suite: --cycle is required (must be >0)")
		return 10
	}
	// Auto-resolve suite root from kernel-owned cycle-state.json when --root
	// was not explicitly overridden from the default (mode 5, ADR-0025).
	if root == "." {
		if resolved := resolveACSSuiteRoot(evolveDir, cycle); resolved != "" {
			root = resolved
		}
	}
	v, err := acssuite.Run(acssuite.Options{Root: root, ProjectRoot: mainProjectRoot(root), Cycle: cycle})
	if err != nil {
		fmt.Fprintf(stderr, "evolve acs suite: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "[acs suite] cycle=%d verdict=%s green=%d red=%d skip=%d total=%d (cycle=%d regression=%d red-team=%d)\n",
		v.Cycle, v.Verdict, v.GreenCount, v.RedCount, v.SkipCount, v.PredicateSuite.Total,
		v.PredicateSuite.ThisCycleCount, v.PredicateSuite.RegressionSuiteCount, v.PredicateSuite.RedTeamCount)
	for _, r := range v.Results {
		if r.ResultStr == "red" {
			fmt.Fprintf(stdout, "  RED %s (exit=%d)\n", r.ACID, r.ExitCode)
		}
	}
	if writeJSON {
		dst, wErr := acssuite.WriteVerdict(evolveDir, v)
		if wErr != nil {
			fmt.Fprintf(stderr, "evolve acs suite: write verdict: %v\n", wErr)
			return 1
		}
		fmt.Fprintf(stderr, "[acs suite] verdict written to %s\n", dst)
	}
	if v.RedCount > 0 {
		return 2
	}
	return 0
}

func runACSRun(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve acs run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		cycle     int
		evolveDir string
		writeJSON bool
	)
	fs.IntVar(&cycle, "cycle", 0, "cycle number (required)")
	fs.StringVar(&evolveDir, "evolve-dir", ".evolve", "path to .evolve/ state directory")
	fs.BoolVar(&writeJSON, "json", true, "write acs-verdict.json (default true)")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	if cycle <= 0 {
		fmt.Fprintln(stderr, "evolve acs run: --cycle is required (must be >0)")
		return 10
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "evolve acs run: usage: evolve acs run --cycle N <pkg>")
		return 10
	}
	pkg := fs.Arg(0)
	v, err := acsrunner.Run(context.Background(), cycle, pkg)
	if err != nil {
		fmt.Fprintf(stderr, "evolve acs run: %v\n", err)
		// Still emit the partial verdict if we have one.
	}
	buf, mErr := json.MarshalIndent(v, "", "  ")
	if mErr != nil {
		fmt.Fprintf(stderr, "evolve acs run: marshal: %v\n", mErr)
		return 1
	}
	fmt.Fprintf(stdout, "%s\n", buf)
	if writeJSON {
		dst, wErr := acsrunner.WriteVerdict(evolveDir, v)
		if wErr != nil {
			fmt.Fprintf(stderr, "evolve acs run: write verdict: %v\n", wErr)
			return 1
		}
		fmt.Fprintf(stderr, "[acs] verdict written to %s (red=%d/%d)\n", dst, v.RedCount, v.Total)
	}
	if v.RedCount > 0 {
		return 2
	}
	return 0
}
