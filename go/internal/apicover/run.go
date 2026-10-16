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

type Config struct {
	Dirs              []string
	CoverPath         string
	RequireDoc        bool
	Enforce           bool
	ChangedFilesByDir map[string]map[string]bool
}

func Run(ctx context.Context, cfg Config, w io.Writer) (int, error) {
	coverByPath, err := loadCoverByPath(cfg.CoverPath)
	if err != nil {
		return 2, err
	}

	totalProblems := 0
	for _, dir := range cfg.Dirs {
		if err := ctx.Err(); err != nil {
			return 2, err
		}
		problems, err := measureDir(ctx, cfg, dir, coverByPath, w)
		if err != nil {
			return 2, err
		}
		totalProblems += problems
	}

	if cfg.Enforce && totalProblems > 0 {
		return 1, nil
	}
	return 0, nil
}

func loadCoverByPath(coverPath string) (map[string]float64, error) {
	coverByPath := map[string]float64{}
	if coverPath != "" {
		f, err := os.Open(coverPath)
		if err != nil {
			return nil, err
		}
		defer func() { _ = f.Close() }()
		entries, err := ParseCoverFunc(f)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			coverByPath[e.Path+":"+strconv.Itoa(e.Line)] = e.Pct
		}
	}
	return coverByPath, nil
}

func measureDir(ctx context.Context, cfg Config, dir string, coverByPath map[string]float64, w io.Writer) (int, error) {
	syms, err := Enumerate(ctx, dir)
	if err != nil {
		return 0, err
	}
	named, err := NamesReferencedInTests(ctx, dir)
	if err != nil {
		return 0, err
	}
	imp, err := packageImportPath(dir)
	if err != nil {
		return 0, err
	}
	rep := Classify(syms, named, coverPctByName(syms, imp, coverByPath))
	if changed, scoped := cfg.ChangedFilesByDir[dir]; scoped {
		var pre []Symbol
		rep, pre = splitPreExisting(rep, changed)
		printReport(w, dir, rep, cfg.RequireDoc, syms)
		if len(pre) > 0 {
			printList(w, "PRE-EXISTING DEBT (files untouched by this change — WARN only, pay down separately):", pre)
		}
		return len(rep.Uncovered) + len(rep.FalseGreens), nil
	}
	printReport(w, dir, rep, cfg.RequireDoc, syms)
	return len(rep.Uncovered) + len(rep.FalseGreens), nil
}

func coverPctByName(syms []Symbol, imp string, coverByPath map[string]float64) map[string]float64 {
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
	return coverPct
}

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
