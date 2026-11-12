// Package testdup counts the extra copies of equal helper bodies in test files, the ratchet of docs/architecture/test-system.md section 6.4.
package testdup

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	minStatements   = 2
	BaselineRelPath = ".test-dup-baseline"
)

type Group struct {
	Body  string
	Sites []string
}

type Report struct {
	Copies int
	Groups []Group
}

type Baseline struct {
	Count int
	Date  time.Time
}

func Scan(root string, since time.Time) (Report, error) {
	if _, err := os.Stat(root); err != nil {
		return Report{}, err
	}
	bodies := map[string][]string{}
	ages := loadAcsAges(root, since)
	fsys := os.DirFS(root)
	walk := func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return dirDecision(ages, rel)
		}
		if !strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		return collect(fsys, rel, bodies)
	}
	if err := fs.WalkDir(fsys, ".", walk); err != nil {
		return Report{}, err
	}
	return buildReport(bodies), nil
}

func dirDecision(ages acsAges, rel string) error {
	if rel == "." {
		return nil
	}
	name := path.Base(rel)
	if name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
		return fs.SkipDir
	}
	if path.Dir(rel) == "acs" && ages.isOld(rel) {
		return fs.SkipDir
	}
	return nil
}

type acsAges struct {
	root    string
	since   time.Time
	tracked map[string]bool
	touched map[string]bool
}

func loadAcsAges(root string, since time.Time) acsAges {
	ages := acsAges{root: root, since: since, tracked: map[string]bool{}, touched: map[string]bool{}}
	for _, name := range gitLines(root, "ls-files", "--", "acs") {
		ages.tracked[acsDir(name)] = true
	}
	var committed int64
	for _, line := range gitLines(root, "log", "--relative", "--name-only", "--format=@%ct", "--", "acs") {
		if stamp, ok := strings.CutPrefix(line, "@"); ok {
			committed, _ = strconv.ParseInt(stamp, 10, 64)
			continue
		}
		if committed >= since.Unix() {
			ages.touched[acsDir(line)] = true
		}
	}
	for _, name := range gitLines(root, "diff", "--relative", "--name-only", "HEAD", "--", "acs") {
		ages.touched[acsDir(name)] = true
	}
	return ages
}

func acsDir(name string) string {
	parts := strings.SplitN(name, "/", 3)
	if len(parts) < 3 {
		return ""
	}
	return parts[0] + "/" + parts[1]
}

func gitLines(root string, args ...string) []string {
	out, err := sysexec.Command(context.Background(), "git", append([]string{"-C", root}, args...)...).Output()
	if err != nil {
		return nil
	}
	return strings.Fields(string(out))
}

func (a acsAges) isOld(rel string) bool {
	if a.tracked[rel] {
		return !a.touched[rel]
	}
	var newest time.Time
	_ = filepath.WalkDir(filepath.Join(a.root, filepath.FromSlash(rel)), func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if info, infoErr := d.Info(); infoErr == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
		return nil
	})
	return newest.Before(a.since)
}

func collect(fsys fs.FS, rel string, bodies map[string][]string) error {
	src, err := fs.ReadFile(fsys, rel)
	if err != nil {
		return err
	}
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, rel, src, 0)
	if err != nil {
		return fmt.Errorf("testdup: %s does not parse: %w", rel, err)
	}
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || len(fn.Body.List) < minStatements || isTestEntry(fn.Name.Name) {
			continue
		}
		key := normalize(fset, src, fn.Body)
		bodies[key] = append(bodies[key], rel+":"+fn.Name.Name)
	}
	return nil
}

func isTestEntry(name string) bool {
	for _, prefix := range []string{"Test", "Benchmark", "Fuzz", "Example"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func normalize(fset *token.FileSet, src []byte, body *ast.BlockStmt) string {
	start := fset.Position(body.Lbrace).Offset
	end := fset.Position(body.Rbrace).Offset + 1
	text := src[start:end]
	tokenFile := token.NewFileSet().AddFile("", -1, len(text))
	var s scanner.Scanner
	s.Init(tokenFile, text, nil, 0)
	var b strings.Builder
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			return b.String()
		}
		b.WriteString(tok.String())
		b.WriteByte(':')
		b.WriteString(lit)
		b.WriteByte(' ')
	}
}

func buildReport(bodies map[string][]string) Report {
	var report Report
	for body, sites := range bodies {
		if len(sites) < 2 {
			continue
		}
		sort.Strings(sites)
		report.Copies += len(sites) - 1
		report.Groups = append(report.Groups, Group{Body: body, Sites: sites})
	}
	sort.Slice(report.Groups, func(i, j int) bool { return report.Groups[i].Sites[0] < report.Groups[j].Sites[0] })
	return report
}

func GroupsUnder(report Report, dir string) []Group {
	prefix := strings.TrimSuffix(dir, "/") + "/"
	var out []Group
	for _, g := range report.Groups {
		if slices.ContainsFunc(g.Sites, func(site string) bool { return strings.HasPrefix(site, prefix) }) {
			out = append(out, g)
		}
	}
	return out
}

func Check(report Report, baseline Baseline) error {
	switch {
	case report.Copies > baseline.Count:
		var sites []string
		for _, g := range report.Groups {
			sites = append(sites, strings.Join(g.Sites, " = "))
		}
		return fmt.Errorf("testdup: %d copies of equal helper bodies, above the baseline %d; use the standard library or one shared helper instead of a new copy:\n%s", report.Copies, baseline.Count, strings.Join(sites, "\n"))
	case report.Copies < baseline.Count:
		return fmt.Errorf("testdup: %d copies, below the baseline %d; lower go/.test-dup-baseline to %d in the same commit", report.Copies, baseline.Count, report.Copies)
	}
	return nil
}

func ParseBaseline(raw []byte) (Baseline, error) {
	var baseline Baseline
	var haveCount, haveDate bool
	for _, line := range strings.Split(string(raw), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch key {
		case "count":
			n, err := strconv.Atoi(value)
			if err != nil || n < 0 {
				return Baseline{}, fmt.Errorf("testdup: baseline count %q is not a non-negative number", value)
			}
			baseline.Count, haveCount = n, true
		case "date":
			d, err := time.Parse("2006-01-02", value)
			if err != nil {
				return Baseline{}, fmt.Errorf("testdup: baseline date %q is not YYYY-MM-DD", value)
			}
			baseline.Date, haveDate = d, true
		}
	}
	if !haveCount || !haveDate {
		return Baseline{}, fmt.Errorf("testdup: baseline needs a count line and a date line")
	}
	return baseline, nil
}

func FormatBaseline(baseline Baseline) []byte {
	return []byte(fmt.Sprintf("count=%d\ndate=%s\n", baseline.Count, baseline.Date.Format("2006-01-02")))
}
