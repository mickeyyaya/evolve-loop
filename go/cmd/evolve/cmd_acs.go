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

// resolveACSSuiteRoot returns the cycle's active_worktree, or "" when the cycle
// state is absent or malformed.
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

// suiteProjectRoot resolves the plane root that holds the cycle's runtime
// state. Only the orchestrator-minted cycle-state.json proves a plane: `evolve
// acs run` mints a bare runs/cycle-N directory under any cwd, worktrees included.
func suiteProjectRoot(evolveDir string, cycle int, root string) string {
	if abs, err := filepath.Abs(evolveDir); err == nil {
		state := filepath.Join(abs, "runs", fmt.Sprintf("cycle-%d", cycle), "cycle-state.json")
		if _, serr := os.Stat(state); serr == nil {
			return filepath.Dir(abs)
		}
	}
	return mainProjectRoot(root)
}

// mainProjectRoot follows a git worktree back to its main checkout, or returns
// dir. It is wrong when the plane is itself a linked worktree, so it serves only
// invocations with no plane evolve dir.
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

// runACS implements `evolve acs run|suite`; both write the cycle's
// acs-verdict.json.
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

// runACSSuite runs the Go predicate lane and exits 2 on any RED predicate and 1
// on a hard error such as a predicate package that fails to compile.
// See ADR-0042.
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
	fs.StringVar(&root, "root", ".", "repo root (the Go module's parent)")
	fs.StringVar(&evolveDir, "evolve-dir", ".evolve", "path to .evolve/ state directory")
	fs.BoolVar(&writeJSON, "json", true, "write acs-verdict.json (default true)")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	if cycle <= 0 {
		fmt.Fprintln(stderr, "evolve acs suite: --cycle is required (must be >0)")
		return 10
	}
	if root == "." {
		if resolved := resolveACSSuiteRoot(evolveDir, cycle); resolved != "" {
			root = resolved
		}
	}
	v, err := acssuite.Run(acssuite.Options{Root: root, ProjectRoot: suiteProjectRoot(evolveDir, cycle, root), Cycle: cycle})
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
		// Fall through: the partial verdict is still emitted.
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
