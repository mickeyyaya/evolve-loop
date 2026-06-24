package main

// cmd_names.go — `evolve names check|fix`, the operator surface for the
// config-driven naming guard (pkg/naminguard, SSOT .evolve/naming.json). The
// same scanner backs the legacynames acs gate and the release preflight, so all
// three agree on what counts as a dead naming token. `check` is read-only and
// exits 1 on any match; `fix` rewrites tokens in place (staged for review,
// never committed).

import (
	"flag"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/pkg/naminguard"
)

func runNames(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	sub := "check"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		sub, args = args[0], args[1:]
	}
	fs := flag.NewFlagSet("names "+sub, flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("project-root", "", "repo root (default: git toplevel of cwd)")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	repoRoot, err := resolveRepoRoot(*root)
	if err != nil {
		fmt.Fprintf(stderr, "evolve names: %v\n", err)
		return 1
	}
	m, err := naminguard.Load(filepath.Join(repoRoot, naminguard.DefaultManifestPath))
	if err != nil {
		fmt.Fprintf(stderr, "evolve names: %v\n", err)
		return 1
	}

	switch sub {
	case "check":
		vs, err := naminguard.Scan(repoRoot, m)
		if err != nil {
			fmt.Fprintf(stderr, "evolve names: %v\n", err)
			return 1
		}
		if len(vs) == 0 {
			fmt.Fprintln(stdout, "names: clean — no dead naming tokens in tracked files")
			return 0
		}
		for _, v := range vs {
			fmt.Fprintln(stdout, v.String())
		}
		fmt.Fprintf(stderr, "evolve names: %d dead naming token(s) — run `evolve names fix`\n", len(vs))
		return 1
	case "fix":
		changed, err := naminguard.Fix(repoRoot, m)
		if err != nil {
			fmt.Fprintf(stderr, "evolve names: %v\n", err)
			return 1
		}
		if len(changed) == 0 {
			fmt.Fprintln(stdout, "names: nothing to fix — already clean")
			return 0
		}
		for _, f := range changed {
			fmt.Fprintf(stdout, "fixed %s\n", f)
		}
		fmt.Fprintf(stdout, "names: rewrote %d file(s) — review and commit\n", len(changed))
		return 0
	default:
		fmt.Fprintf(stderr, "evolve names: unknown subcommand %q (want check|fix)\n", sub)
		return 10
	}
}

// resolveRepoRoot returns explicit if set, else the git toplevel of cwd.
func resolveRepoRoot(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("resolve repo root (not in a git work tree?): %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
