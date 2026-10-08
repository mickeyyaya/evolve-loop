package stelint

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"strconv"
)

const maxConstDepth = 8

var (
	fprintNames    = setOf([]string{"Fprint", "Fprintf", "Fprintln"})
	logMethodNames = setOf([]string{"Infof", "Warnf", "Errorf", "Printf", "Logf"})
	streamNames    = setOf([]string{"stderr", "Stderr", "stdout", "Stdout"})
)

type goMessages struct {
	imports map[string]string
	consts  map[string]ast.Expr
}

func CheckGo(src []byte, opt Options) ([]Finding, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("parse Go source: %w", err)
	}
	if ast.IsGenerated(file) {
		return []Finding{generatedNotice(1, "this file")}, nil
	}
	g := goMessages{imports: importNames(file), consts: constValues(file)}
	m := newMatcher(opt.Words)
	var out []Finding
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if arg := g.messageArg(call); arg != nil {
			if text, ok := g.resolve(arg, 0); ok {
				out = append(out, m.checkText(text, fset.Position(arg.Pos()).Line)...)
			}
		}
		return true
	})
	sortFindings(out)
	return out, nil
}

func (m matcher) checkText(text string, line int) []Finding {
	runes := []rune(text)
	lines := make([]int, len(runes))
	for i := range lines {
		lines[i] = line
	}
	sents := m.sentences(scanText(runes, lines, goTextSyntax))
	return append(m.wordFindingsOf(sents), lengthFindings(sents, maxSentenceWords)...)
}

func importNames(file *ast.File) map[string]string {
	names := map[string]string{}
	for _, spec := range file.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := path.Base(p)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		names[name] = p
	}
	return names
}

func constValues(file *ast.File) map[string]ast.Expr {
	counts := map[string]int{}
	values := map[string]ast.Expr{}
	ast.Inspect(file, func(n ast.Node) bool {
		decl, ok := n.(*ast.GenDecl)
		if !ok || decl.Tok != token.CONST {
			return true
		}
		for _, spec := range decl.Specs {
			vs := spec.(*ast.ValueSpec)
			for i, name := range vs.Names {
				counts[name.Name]++
				if i < len(vs.Values) {
					values[name.Name] = vs.Values[i]
				}
			}
		}
		return true
	})
	for name, n := range counts {
		if n != 1 {
			delete(values, name)
		}
	}
	return values
}

func (g goMessages) importPath(x ast.Expr) string {
	if id, ok := x.(*ast.Ident); ok {
		return g.imports[id.Name]
	}
	return ""
}

func (g goMessages) messageArg(call *ast.CallExpr) ast.Expr {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) == 0 {
		return nil
	}
	pkg, name := g.importPath(sel.X), sel.Sel.Name
	switch {
	case pkg == "fmt" && fprintNames[name]:
		if len(call.Args) > 1 && isStream(call.Args[0]) {
			return call.Args[1]
		}
		return nil
	case pkg == "errors" && name == "New", pkg == "log", logMethodNames[name]:
		return call.Args[0]
	}
	return nil
}

func isStream(e ast.Expr) bool {
	switch v := e.(type) {
	case *ast.Ident:
		return streamNames[v.Name]
	case *ast.SelectorExpr:
		return streamNames[v.Sel.Name]
	}
	return false
}

func (g goMessages) resolve(e ast.Expr, depth int) (string, bool) {
	if depth > maxConstDepth {
		return "", false
	}
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(v.Value)
		return s, err == nil
	case *ast.ParenExpr:
		return g.resolve(v.X, depth+1)
	case *ast.BinaryExpr:
		if v.Op != token.ADD {
			return "", false
		}
		left, okLeft := g.resolve(v.X, depth+1)
		right, okRight := g.resolve(v.Y, depth+1)
		return left + right, okLeft && okRight
	case *ast.Ident:
		if c, ok := g.consts[v.Name]; ok {
			return g.resolve(c, depth+1)
		}
	}
	return "", false
}
