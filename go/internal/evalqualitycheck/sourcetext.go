package evalqualitycheck

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

const (
	SourceTextWaiver  = "acs-predicate: config-check"
	sourceTextReason  = "source-text predicate: only matches source text (acsassert.FileContains/FileMatchesRegex) and never runs the code — exercise the behaviour (exec.Command, acsassert.SubprocessOutput or a call into the package), or mark a config-presence check with `// " + SourceTextWaiver + "`"
	repoModulePrefix  = "github.com/mickeyyaya/evolve-loop/go/"
	acsassertImport   = repoModulePrefix + "pkg/acsassert"
	osExecImport      = "os/exec"
	acsassertPkgIdent = "acsassert"
)

var sourceTextCalls = map[string]bool{"FileContains": true, "FileMatchesRegex": true}

type SourceTextFinding struct {
	Func   string
	File   string
	Reason string
}

type SourceTextLintReport struct {
	Path     string
	Files    []string
	Findings []SourceTextFinding
}

func (r SourceTextLintReport) Linted() int { return len(r.Files) }

func LintSourceTextPredicates(path string) (SourceTextLintReport, error) {
	report := SourceTextLintReport{Path: path}
	paths, err := predicateSourcePaths(path)
	if err != nil {
		return report, err
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, p := range paths {
		f, perr := parser.ParseFile(fset, p, nil, parser.ParseComments)
		if perr != nil {
			return report, perr
		}
		files = append(files, f)
		report.Files = append(report.Files, filepath.Base(p))
	}
	helpers := flakyPackageFuncs(files)
	for i, f := range files {
		runners := executingImports(f)
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Recv != nil || !strings.HasPrefix(fn.Name.Name, "Test") {
				continue
			}
			if fn.Doc != nil && strings.Contains(fn.Doc.Text(), SourceTextWaiver) {
				continue
			}
			if usesSourceText(fn.Body) && !runsCode(fn.Body, runners, helpers, map[string]bool{fn.Name.Name: true}) {
				report.Findings = append(report.Findings, SourceTextFinding{Func: fn.Name.Name, File: report.Files[i], Reason: sourceTextReason})
			}
		}
	}
	return report, nil
}

func executingImports(f *ast.File) map[string]bool {
	out := map[string]bool{}
	for _, imp := range f.Imports {
		p := strings.Trim(imp.Path.Value, "\"`")
		name := filepath.Base(p)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		if p == osExecImport || (strings.HasPrefix(p, repoModulePrefix) && p != acsassertImport) {
			out[name] = true
		}
	}
	return out
}

func usesSourceText(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			for name := range sourceTextCalls {
				if isPkgCall(call, acsassertPkgIdent, name) {
					found = true
				}
			}
		}
		return !found
	})
	return found
}

func runsCode(body *ast.BlockStmt, runners map[string]bool, helpers map[string]*ast.FuncDecl, seen map[string]bool) bool {
	runs := false
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || runs {
			return !runs
		}
		switch fun := call.Fun.(type) {
		case *ast.SelectorExpr:
			if x, ok := fun.X.(*ast.Ident); ok {
				runs = runners[x.Name] || (x.Name == acsassertPkgIdent && (strings.HasPrefix(fun.Sel.Name, "Subprocess") || strings.HasSuffix(fun.Sel.Name, "Output")))
			}
		case *ast.Ident:
			if h, ok := helpers[fun.Name]; ok && !seen[fun.Name] {
				seen[fun.Name] = true
				runs = runsCode(h.Body, runners, helpers, seen)
			}
		}
		return !runs
	})
	return runs
}
