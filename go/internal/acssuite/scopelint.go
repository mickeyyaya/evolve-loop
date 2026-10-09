package acssuite

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	"github.com/mickeyyaya/evolve-loop/go/internal/gopkgpattern"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type ScopeFinding struct {
	Test    string
	File    string
	Pattern string
}

func LintPredicateScope(dir string, touched []string) ([]ScopeFinding, error) {
	if len(touched) == 0 {
		return nil, nil
	}
	inScope := map[string]bool{}
	for _, p := range touched {
		if key := patternKey(p); key != "" {
			inScope[key] = true
		}
	}

	files, names, err := parseGoDir(dir)
	if err != nil {
		return nil, err
	}
	consts := packageStringConsts(files)
	var findings []ScopeFinding
	for i, file := range files {
		findings = append(findings, fileScopeFindings(file, names[i], consts, inScope)...)
	}
	return findings, nil
}

func parseGoDir(dir string) ([]*ast.File, []string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	fset := token.NewFileSet()
	var files []*ast.File
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		f, perr := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		if perr != nil {
			return nil, nil, perr
		}
		files = append(files, f)
		names = append(names, e.Name())
	}
	return files, names, nil
}

func fileScopeFindings(file *ast.File, name string, consts map[string]string, inScope map[string]bool) []ScopeFinding {
	var findings []ScopeFinding
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || !strings.HasPrefix(fn.Name.Name, "Test") {
			continue
		}
		for _, pat := range functionPackagePatterns(fn, consts) {
			key := patternKey(pat)
			if key == "" || strings.HasPrefix(key, "acs/") || inScope[key] {
				continue
			}
			findings = append(findings, ScopeFinding{Test: fn.Name.Name, File: name, Pattern: pat})
		}
	}
	return findings
}

func packageStringConsts(files []*ast.File) map[string]string {
	out := map[string]string{}
	for _, file := range files {
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || (gd.Tok != token.CONST && gd.Tok != token.VAR) {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, ident := range vs.Names {
					if i < len(vs.Values) {
						if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
							if v, err := strconv.Unquote(lit.Value); err == nil {
								out[ident.Name] = v
							}
						}
					}
				}
			}
		}
	}
	return out
}

func functionPackagePatterns(fn *ast.FuncDecl, consts map[string]string) []string {
	var out []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.BasicLit:
			if v.Kind == token.STRING {
				if s, err := strconv.Unquote(v.Value); err == nil && isPackagePattern(s) {
					out = append(out, s)
				}
			}
		case *ast.Ident:
			if s, ok := consts[v.Name]; ok && isPackagePattern(s) {
				out = append(out, s)
			}
		}
		return true
	})
	return out
}

func isPackagePattern(s string) bool {
	return gopkgpattern.IsPackagePattern(s)
}

const wholeModuleKey = "(whole-module)"

func patternKey(p string) string {
	if strings.TrimSpace(p) == gopkgpattern.WholeModule {
		return wholeModuleKey
	}
	return gopkgpattern.Key(p)
}

var scopeLintChangedPackages = func(worktreeRoot string) []string {
	pkgs, ok := changedpkgs.FromGitChecked(worktreeRoot, "HEAD")
	if !ok {
		return nil
	}
	return pkgs
}

func demoteOutOfScope(results []Result, opts Options) []string {
	touched := scopeLintChangedPackages(opts.Root)
	if len(touched) == 0 {
		return nil
	}
	findings, err := LintPredicateScope(currentCycleGoPkgDir(moduleRoot(opts), opts.Cycle), touched)
	if err != nil {
		return []string{fmt.Sprintf("scope-lint: disabled (predicate sources unparseable: %v)", err)}
	}
	if len(findings) == 0 {
		return nil
	}
	flagged := map[string][]string{}
	for _, f := range findings {
		flagged[f.Test] = append(flagged[f.Test], f.Pattern)
	}

	cyclePrefix := fmt.Sprintf("cycle%d/", opts.Cycle)
	var demote []int
	liveOwn := 0
	for i, r := range results {
		name, ok := strings.CutPrefix(r.ACID, cyclePrefix)
		if !ok {
			continue
		}
		if r.ResultStr != "skip" {
			liveOwn++
		}
		if _, hit := flagged[testRootName(name)]; hit && r.ResultStr != "skip" {
			demote = append(demote, i)
		}
	}
	if len(demote) > 0 && len(demote) == liveOwn {
		return []string{fmt.Sprintf("scope-lint: %d out-of-scope predicate(s) found but demotion CANCELLED — it would leave zero live own predicates (gate-weakening floor); scope the predicates to the touched packages %v", len(demote), touched)}
	}

	var warnings []string
	for _, i := range demote {
		name := strings.TrimPrefix(results[i].ACID, cyclePrefix)
		pats := flagged[testRootName(name)]
		results[i].ResultStr = "skip"
		results[i].ExitCode = SkipExitCode
		if results[i].EvidenceNote != "" {
			results[i].EvidenceNote += " | "
		}
		results[i].EvidenceNote += fmt.Sprintf(
			"out-of-scope meta-predicate demoted to SKIP: shells go test over %v beyond this cycle's touched packages %v — "+
				"whole-repo staleness is the regression suite's job; scope the predicate to the touched packages (whole-suite false-red class, e.g. cycles 1115/1117 auditor probes, 1116 shared-fixture contention)",
			pats, touched)
		warnings = append(warnings, fmt.Sprintf("scope-lint: %s demoted to SKIP (references %v; touched %v)", results[i].ACID, pats, touched))
	}
	return warnings
}

func testRootName(name string) string {
	if i := strings.IndexByte(name, '/'); i >= 0 {
		return name[:i]
	}
	return name
}

func moduleRoot(opts Options) string {
	if opts.GoModuleDir != "" {
		return opts.GoModuleDir
	}
	return filepath.Join(opts.Root, "go")
}
