package apicover

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Config parameterizes a Run.
type Config struct {
	Dirs       []string // package directories to measure
	CoverPath  string   // optional `go tool cover -func` output file
	RequireDoc bool     // also flag exported decls missing a godoc comment
	Enforce    bool     // exit non-zero when uncovered/false-green symbols exist
	// ChangedFilesByDir diff-scopes enforcement (cycle-1048: four PRE-EXISTING
	// false-green core symbols floor-blocked every core-touching lane). Keyed
	// by the Dirs entry; the inner set holds changed file basenames in that
	// package. When set, violations in files OUTSIDE the change are reported
	// under a PRE-EXISTING DEBT header and do NOT count toward the Enforce
	// exit code. Nil = classic behavior (CI's repo-wide run is unchanged).
	ChangedFilesByDir map[string]map[string]bool
}

// Run measures each configured directory and writes a per-package report to w.
// It returns the process exit code: 0 in warning-only mode (the default), or 1
// under Enforce when any uncovered or false-green symbol remains. A non-nil
// error (returned with code 2) means the measurement itself failed.
//
// ctx bounds the measurement (checked at each dir and file boundary): the
// audit's in-process enforce gate runs Run under its apicoverTimeout, and
// without the ctx thread a wedged AST walk (pathological package, hung
// filesystem) escaped the gate's own deadline (apicover-inprocess-ctx-timeout).
// Cancellation surfaces as the code-2 measurement error with ctx.Err() in the
// chain, so callers can distinguish infra interruption from a real finding.
func Run(ctx context.Context, cfg Config, w io.Writer) (int, error) {
	// Join coverage to symbols on the import-path-qualified path + line. A method
	// prints under its bare name in `go tool cover -func`, so name-keying is
	// impossible; the qualified path:line is exact and collision-free across
	// packages that share a filename (config.go, doc.go, …).
	coverByPath := map[string]float64{}
	if cfg.CoverPath != "" {
		f, err := os.Open(cfg.CoverPath)
		if err != nil {
			return 2, err
		}
		defer func() { _ = f.Close() }() // read-only file; close error is not actionable
		entries, err := ParseCoverFunc(f)
		if err != nil {
			return 2, err
		}
		for _, e := range entries {
			coverByPath[e.Path+":"+strconv.Itoa(e.Line)] = e.Pct
		}
	}

	totalProblems := 0
	for _, dir := range cfg.Dirs {
		if err := ctx.Err(); err != nil {
			return 2, err
		}
		syms, err := Enumerate(ctx, dir)
		if err != nil {
			return 2, err
		}
		named, err := NamesReferencedInTests(ctx, dir)
		if err != nil {
			return 2, err
		}
		imp, err := packageImportPath(dir)
		if err != nil {
			return 2, err
		}
		coverPct := map[string]float64{}
		for _, s := range syms {
			if s.Kind != KindFunc && s.Kind != KindMethod {
				continue
			}
			key := imp + "/" + s.File + ":" + strconv.Itoa(s.Line)
			if pct, ok := coverByPath[key]; ok {
				coverPct[s.Name] = pct
			}
		}
		rep := Classify(syms, named, coverPct)
		if changed, scoped := cfg.ChangedFilesByDir[dir]; scoped {
			var pre []Symbol
			rep, pre = splitPreExisting(rep, changed)
			printReport(w, dir, rep, cfg.RequireDoc, syms)
			if len(pre) > 0 {
				printList(w, "PRE-EXISTING DEBT (files untouched by this change — WARN only, pay down separately):", pre)
			}
			totalProblems += len(rep.Uncovered) + len(rep.FalseGreens)
			continue
		}
		printReport(w, dir, rep, cfg.RequireDoc, syms)
		totalProblems += len(rep.Uncovered) + len(rep.FalseGreens)
	}

	if cfg.Enforce && totalProblems > 0 {
		return 1, nil
	}
	return 0, nil
}

// packageImportPath derives the import path of the package rooted at dir by
// walking up to the enclosing go.mod, reading its module line, and appending the
// directory's path relative to the module root. `go/build` cannot resolve
// module import paths (it predates modules), and we avoid golang.org/x/tools, so
// this stdlib-only derivation is what makes the import-path-qualified cover join
// exact.
func packageImportPath(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for d := abs; ; {
		if b, err := os.ReadFile(filepath.Join(d, "go.mod")); err == nil {
			mod := moduleLine(b)
			if mod == "" {
				return "", fmt.Errorf("no module line in %s/go.mod", d)
			}
			rel, err := filepath.Rel(d, abs)
			if err != nil {
				return "", err
			}
			if rel == "." {
				return mod, nil
			}
			return mod + "/" + filepath.ToSlash(rel), nil
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", fmt.Errorf("go.mod not found above %s", dir)
		}
		d = parent
	}
}

func moduleLine(gomod []byte) string {
	for _, line := range strings.Split(string(gomod), "\n") {
		line = strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(line, "module "); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

func printReport(w io.Writer, dir string, r Report, requireDoc bool, syms []Symbol) {
	fmt.Fprintf(w, "== %s ==\n", dir)
	printList(w, "UNCOVERED (no test names it)", r.Uncovered)
	printList(w, "FALSE-GREEN (named by a test but 0% executed)", r.FalseGreens)
	printIgnored(w, r.Ignored)
	if requireDoc {
		printList(w, "MISSING-DOC (exported, no godoc)", MissingDoc(syms))
	}
	fmt.Fprintf(w, "summary: %d exported, %d covered, %d uncovered, %d false-green, %d ignored\n\n",
		len(syms), len(r.Covered), len(r.Uncovered), len(r.FalseGreens), len(r.Ignored))
}

func printList(w io.Writer, header string, syms []Symbol) {
	if len(syms) == 0 {
		return
	}
	lines := make([]string, len(syms))
	for i, s := range syms {
		lines[i] = fmt.Sprintf("  %-7s %s (%s)", s.Kind, s.Name, s.File)
	}
	sort.Strings(lines)
	fmt.Fprintf(w, "%s: %d\n", header, len(syms))
	for _, l := range lines {
		fmt.Fprintln(w, l)
	}
}

// printIgnored always prints the full //apicover:ignore list (with reasons) so a
// suppression is never silent.
func printIgnored(w io.Writer, syms []Symbol) {
	if len(syms) == 0 {
		return
	}
	lines := make([]string, len(syms))
	for i, s := range syms {
		lines[i] = fmt.Sprintf("  %s — %s", s.Name, s.IgnoreReason)
	}
	sort.Strings(lines)
	fmt.Fprintf(w, "IGNORED (//apicover:ignore): %d\n", len(syms))
	for _, l := range lines {
		fmt.Fprintln(w, l)
	}
}

// splitPreExisting removes violations whose defining file is NOT in the
// change set, returning them separately: a lane owns the hygiene of the files
// it touches, never the debt of files it didn't (cycle-1048).
func splitPreExisting(r Report, changed map[string]bool) (Report, []Symbol) {
	var pre []Symbol
	keep := func(in []Symbol) []Symbol {
		out := in[:0]
		for _, s := range in {
			if changed[s.File] {
				out = append(out, s)
			} else {
				pre = append(pre, s)
			}
		}
		return out
	}
	r.Uncovered = keep(r.Uncovered)
	r.FalseGreens = keep(r.FalseGreens)
	return r, pre
}
