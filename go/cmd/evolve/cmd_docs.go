package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
)

const (
	steLintForm   = "docs ste-lint [--json] [--strict] [--go] [--changed <base-ref> [--added]] [--project-root P] [paths...]"
	steLintUsage  = "usage: evolve " + steLintForm
	steLintPrefix = "evolve docs ste-lint: "
	docsUsage     = "usage: evolve docs <verb>\n\nverbs:\n" +
		"  ste-lint   check text against the ASD-STE100 house rules in " + stelint.StandardPath + "\n" +
		"             ( " + steLintForm + " )\n" +
		"             The findings are a WARN: the exit code is 0, or 1 with --strict.\n" +
		"  index      write or check the dated entries of docs/research-index.md from docs/research-index/\n" +
		"             ( docs index <generate|check> [--project-root P] )\n"
)

type steMode struct {
	inScope   func(string) bool
	scopeDirs []string
	wanted    func(name string) bool
}

var (
	steDocsMode = steMode{inScope: stelint.InDocsScope, scopeDirs: []string{"docs"}, wanted: func(name string) bool {
		return strings.HasSuffix(name, ".md")
	}}
	steGoMode = steMode{inScope: stelint.InGoScope, scopeDirs: []string{"go"}, wanted: func(name string) bool {
		return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
	}}
)

type steLintArgs struct {
	asJSON, strict, goMode, help, added bool
	changed                             []string
	projectRoot                         string
	paths                               []string
}

var steAbs = filepath.Abs

type steTarget struct {
	abs, display string
	added        map[int]bool
}

func runDocs(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Fprint(stdout, docsUsage)
		return 0
	}
	if args[0] == "ste-lint" {
		return runSteLint(args[1:], stdout, stderr)
	}
	if args[0] == "index" {
		return docsIndexVerb.run(args[1:], stdout, stderr)
	}
	fmt.Fprintf(stderr, "evolve docs: unknown verb %q\n%s", args[0], docsUsage)
	return exitUsage
}

func parseSteLintArgs(args []string) (steLintArgs, error) {
	var a steLintArgs
	operands, err := cliFlags{
		bools:  map[string]*bool{"--json": &a.asJSON, "--strict": &a.strict, "--go": &a.goMode, "--added": &a.added, "--help": &a.help, "-h": &a.help},
		values: map[string]*string{"--project-root": &a.projectRoot},
		lists:  map[string]*[]string{"--changed": &a.changed},
	}.parse(args)
	a.paths = operands
	switch {
	case err != nil:
		return a, err
	case len(a.changed) > 1:
		return a, errors.New("--changed is given more than once")
	case a.added && len(a.changed) == 0:
		return a, errors.New("--added needs --changed")
	case len(a.changed) == 1 && (a.changed[0] == "" || strings.HasPrefix(a.changed[0], "-")):
		return a, fmt.Errorf("--changed needs a base ref, not %q", a.changed[0])
	case len(a.changed) == 1 && len(a.paths) > 0:
		return a, errors.New("use --changed or paths, not both")
	}
	return a, nil
}

func runSteLint(args []string, stdout, stderr io.Writer) int {
	a, err := parseSteLintArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n%s\n", steLintPrefix, err, steLintUsage)
		return exitUsage
	}
	if a.help {
		fmt.Fprintln(stdout, steLintUsage)
		return 0
	}
	root, err := steLintRoot(a.projectRoot)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v; pass --project-root\n", steLintPrefix, err)
		return exitIO
	}
	words, err := stelint.LoadStandard(root)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", steLintPrefix, err)
		return exitIO
	}
	targets, err := steLintTargets(a, root)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", steLintPrefix, err)
		return exitIO
	}
	report, failed := lintSteTargets(targets, stelint.Options{Words: words}, stderr)
	write := report.writeText
	if a.asJSON {
		write = report.writeJSON
	}
	if err := write(stdout); err != nil {
		fmt.Fprintf(stderr, "%s%v\n", steLintPrefix, err)
		return exitIO
	}
	switch {
	case failed:
		return exitIO
	case a.strict && report.FindingCount > 0:
		return 1
	}
	return 0
}

func steLintRoot(explicit string) (string, error) {
	for _, candidate := range []string{explicit, os.Getenv(ipcenv.WorktreeRootKey)} {
		if candidate != "" {
			return filepath.Abs(candidate)
		}
	}
	root, err := resolveRepoRoot("")
	if err != nil && os.Getenv("EVOLVE_PROJECT_ROOT") != "" {
		return filepath.Abs(os.Getenv("EVOLVE_PROJECT_ROOT"))
	}
	return root, err
}

func steLintTargets(a steLintArgs, root string) ([]steTarget, error) {
	mode := steDocsMode
	if a.goMode {
		mode = steGoMode
	}
	switch {
	case len(a.changed) == 1:
		return changedSteTargets(root, a.changed[0], mode, a.added)
	case len(a.paths) > 0:
		return explicitSteTargets(root, a.paths, mode)
	}
	return scopeSteTargets(root, mode)
}

func scopeSteTargets(root string, mode steMode) ([]steTarget, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var rels []string
	for _, e := range entries {
		if !e.IsDir() && mode.inScope(e.Name()) {
			rels = append(rels, e.Name())
		}
	}
	for _, dir := range mode.scopeDirs {
		found, err := walkWanted(filepath.Join(root, dir), mode.wanted)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		for _, abs := range found {
			if rel := steDisplay(root, abs); mode.inScope(rel) {
				rels = append(rels, rel)
			}
		}
	}
	return relTargets(root, rels), nil
}

func walkWanted(dir string, wanted func(string) bool) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && (d.Name() == "testdata" || (strings.HasPrefix(d.Name(), ".") && path != dir)):
			return filepath.SkipDir
		case !d.IsDir() && wanted(d.Name()):
			out = append(out, path)
		}
		return nil
	})
	return out, err
}

func explicitSteTargets(root string, paths []string, mode steMode) ([]steTarget, error) {
	var out []steTarget
	for _, p := range paths {
		abs, err := steAbs(p)
		if err != nil {
			return nil, err
		}
		if _, statErr := os.Stat(abs); statErr != nil && !filepath.IsAbs(p) && root != "" {
			candidate := filepath.Join(root, p)
			if _, cErr := os.Stat(candidate); cErr == nil {
				if cAbs, aErr := steAbs(candidate); aErr == nil {
					abs = cAbs
				}
			}
		}
		info, err := os.Stat(abs)
		if err != nil || !info.IsDir() {
			out = append(out, steTarget{abs: abs, display: steDisplay(root, abs)})
			continue
		}
		found, err := walkWanted(abs, mode.wanted)
		if err != nil {
			return nil, err
		}
		for _, f := range found {
			out = append(out, steTarget{abs: f, display: steDisplay(root, f)})
		}
	}
	return out, nil
}

func changedSteTargets(root, base string, mode steMode, added bool) ([]steTarget, error) {
	var groups [][]string
	for _, args := range [][]string{
		{"diff", "--name-only", "--relative", "--diff-filter=d", base, "--"},
		{"ls-files", "--others", "--exclude-standard"},
	} {
		lines, err := steGitLines(root, args...)
		if err != nil {
			return nil, err
		}
		groups = append(groups, lines)
	}
	tracked, untrackedRels := groups[0], groups[1]
	untracked := map[string]bool{}
	for _, rel := range untrackedRels {
		untracked[rel] = true
	}
	seen := map[string]bool{}
	var rels []string
	for _, rel := range slices.Concat(tracked, untrackedRels) {
		if !seen[rel] && mode.inScope(rel) {
			seen[rel] = true
			rels = append(rels, rel)
		}
	}
	targets := relTargets(root, rels)
	if !added {
		return targets, nil
	}
	return scopeToAddedLines(root, base, targets, untracked)
}

func steGitLines(root string, args ...string) ([]string, error) {
	out, errOut, code, err := gitexec.Default(root).Capture(context.Background(), append([]string{"-c", "core.quotePath=false"}, args...)...)
	if err == nil && code != 0 {
		err = fmt.Errorf("exit %d: %s", code, strings.TrimSpace(errOut))
	}
	if err != nil {
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimRight(line, "\r"); line != "" {
			lines = append(lines, line)
		}
	}
	return lines, nil
}

func relTargets(root string, rels []string) []steTarget {
	sort.Strings(rels)
	out := make([]steTarget, 0, len(rels))
	for _, rel := range rels {
		out = append(out, steTarget{abs: filepath.Join(root, filepath.FromSlash(rel)), display: filepath.ToSlash(rel)})
	}
	return out
}

func steDisplay(root, abs string) string {
	if absRoot, err := filepath.Abs(root); err == nil {
		if rel, err := filepath.Rel(absRoot, abs); err == nil {
			slashed := filepath.ToSlash(rel)
			if slashed != ".." && !strings.HasPrefix(slashed, "../") {
				return slashed
			}
		}
	}
	return filepath.ToSlash(abs)
}
