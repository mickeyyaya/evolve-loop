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

func runACSSuite(args []string, stdout, stderr io.Writer) int {
	const name = "evolve acs suite"
	f := acsFlags{}
	fs := f.flagSet(name, stderr)
	fs.StringVar(&f.root, "root", ".", "repo root (the Go module's parent)")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	f, ok := f.resolved(name, stderr)
	if !ok {
		return 10
	}
	if f.root == "." {
		if resolved := resolveACSSuiteRoot(f.evolveDir, f.cycle); resolved != "" {
			f.root = resolved
		}
	}
	v, err := acssuite.Run(acssuite.Options{Root: f.root, ProjectRoot: suiteProjectRoot(f.evolveDir, f.cycle, f.root), Cycle: f.cycle})
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return 1
	}
	printACSSuiteVerdict(stdout, v)
	if f.writeJSON {
		dst, wErr := acssuite.WriteVerdict(f.evolveDir, v)
		if wErr != nil {
			fmt.Fprintf(stderr, "%s: write verdict: %v\n", name, wErr)
			return 1
		}
		fmt.Fprintf(stderr, "[acs suite] verdict written to %s\n", dst)
	}
	if v.RedCount > 0 {
		return 2
	}
	return 0
}

type acsFlags struct {
	cycle     int
	root      string
	evolveDir string
	writeJSON bool
}

func (f *acsFlags) flagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.IntVar(&f.cycle, "cycle", 0, "cycle number (required)")
	fs.StringVar(&f.evolveDir, "evolve-dir", ".evolve", "path to an existing .evolve/ state directory")
	fs.BoolVar(&f.writeJSON, "json", true, "write acs-verdict.json (default true)")
	return fs
}

func (f acsFlags) resolved(name string, stderr io.Writer) (acsFlags, bool) {
	if f.cycle <= 0 {
		fmt.Fprintf(stderr, "%s: --cycle is required (must be >0)\n", name)
		return f, false
	}
	evolveDir, err := existingEvolveDir(f.evolveDir)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return f, false
	}
	f.evolveDir = evolveDir
	return f, true
}

func existingEvolveDir(evolveDir string) (string, error) {
	abs, err := filepath.Abs(evolveDir)
	if err != nil {
		return "", fmt.Errorf("--evolve-dir %q: %w", evolveDir, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("--evolve-dir %s: %w; run from the project root or pass --evolve-dir <project>/.evolve", abs, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("--evolve-dir %s is not a directory; pass --evolve-dir <project>/.evolve", abs)
	}
	return abs, nil
}

func printACSSuiteVerdict(stdout io.Writer, v acssuite.Verdict) {
	fmt.Fprintf(stdout, "[acs suite] cycle=%d verdict=%s green=%d red=%d skip=%d total=%d (cycle=%d regression=%d red-team=%d)\n",
		v.Cycle, v.Verdict, v.GreenCount, v.RedCount, v.SkipCount, v.PredicateSuite.Total,
		v.PredicateSuite.ThisCycleCount, v.PredicateSuite.RegressionSuiteCount, v.PredicateSuite.RedTeamCount)
	for _, r := range v.Results {
		if r.ResultStr == "red" {
			fmt.Fprintf(stdout, "  RED %s (exit=%d)\n", r.ACID, r.ExitCode)
		}
	}
}

func runACSRun(args []string, stdout, stderr io.Writer) int {
	const name = "evolve acs run"
	f := acsFlags{}
	fs := f.flagSet(name, stderr)
	if err := fs.Parse(args); err != nil {
		return 10
	}
	if f.cycle > 0 && fs.NArg() != 1 {
		fmt.Fprintf(stderr, "%s: usage: evolve acs run --cycle N <pkg>\n", name)
		return 10
	}
	f, ok := f.resolved(name, stderr)
	if !ok {
		return 10
	}
	v, runErr := acsrunner.Run(context.Background(), f.cycle, fs.Arg(0))
	if runErr != nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, runErr)
	}
	buf, mErr := json.MarshalIndent(v, "", "  ")
	if mErr != nil {
		fmt.Fprintf(stderr, "%s: marshal: %v\n", name, mErr)
		return 1
	}
	fmt.Fprintf(stdout, "%s\n", buf)
	if f.writeJSON {
		dst, wErr := acsrunner.WriteVerdict(f.evolveDir, v)
		if wErr != nil {
			fmt.Fprintf(stderr, "%s: write verdict: %v\n", name, wErr)
			return 1
		}
		fmt.Fprintf(stderr, "[acs] verdict written to %s (red=%d/%d)\n", dst, v.RedCount, v.Total)
	}
	switch {
	case runErr != nil:
		return 1
	case v.RedCount > 0:
		return 2
	}
	return 0
}
