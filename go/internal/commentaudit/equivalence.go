// Package commentaudit measures, removes and records a package's comments, and
// proves that an edit changed only comments.
package commentaudit

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/envscope"
)

func Equivalent(name string, before, after []byte) (bool, string, error) {
	b, err := parseShape(name, before)
	if err != nil {
		return false, "", fmt.Errorf("before: %w", err)
	}
	a, err := parseShape(name, after)
	if err != nil {
		return false, "", fmt.Errorf("after: %w", err)
	}
	if a != b {
		return false, "code changed", nil
	}
	if !slices.Equal(anchoredDirectives(name, before), anchoredDirectives(name, after)) {
		return false, "directive changed", nil
	}
	return true, "", nil
}

func parseShape(name string, src []byte) (string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), name, src, parser.SkipObjectResolution|parser.ParseComments)
	if err != nil {
		return "", err
	}
	return codeShape(file)
}

func codeShape(file *ast.File) (string, error) {
	var out strings.Builder
	if err := ast.Fprint(&out, nil, file, codeOnly); err != nil {
		return "", err
	}
	out.WriteString(cgoPreamble(file))
	return out.String(), nil
}

var (
	posType           = reflect.TypeOf(token.NoPos)
	commentGroupType  = reflect.TypeOf((*ast.CommentGroup)(nil))
	commentGroupsType = reflect.TypeOf([]*ast.CommentGroup(nil))
)

func codeOnly(name string, v reflect.Value) bool {
	t := v.Type()
	return ast.NotNilFilter(name, v) && t != posType && t != commentGroupType && t != commentGroupsType
}

func cgoPreamble(file *ast.File) string {
	return cgoPreambleGroup(file).Text()
}

func cgoPreambleGroup(file *ast.File) *ast.CommentGroup {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			continue
		}
		for _, spec := range gen.Specs {
			imp := spec.(*ast.ImportSpec)
			switch {
			case imp.Path.Value != `"C"`:
			case imp.Doc != nil:
				return imp.Doc
			case len(gen.Specs) == 1:
				return gen.Doc
			default:
				return nil
			}
		}
	}
	return nil
}

type markerExtent int

const (
	markerLine markerExtent = iota
	markerParagraph
	markerRestOfGroup
	markerGroup
	markerFile
)

type commentMarker struct {
	pattern          *regexp.Regexp
	extent           markerExtent
	anchorsParagraph bool
}

type commentMarkerRule struct {
	pattern          string
	extent           markerExtent
	anchorsParagraph bool
	scope            markerScope
}

type markerScope int

const (
	scopeAnywhere markerScope = iota
	scopeEnvtaintFiles
	scopeExampleFuncs
)

var commentMarkerRules = []commentMarkerRule{
	{`//go:\S`, markerLine, false, scopeAnywhere},
	{`//line `, markerLine, false, scopeAnywhere},
	{`//export `, markerLine, false, scopeAnywhere},
	{`//extern `, markerLine, false, scopeAnywhere},
	{`//nolint(:|\s|$)`, markerLine, false, scopeAnywhere},
	{`// \+build `, markerLine, false, scopeAnywhere},
	{`// Code generated .* DO NOT EDIT\.$`, markerFile, false, scopeAnywhere},
	{`// acs-predicate:`, markerLine, false, scopeAnywhere},
	{`// minimal:`, markerParagraph, false, scopeAnywhere},
	{`// Deprecated:`, markerParagraph, true, scopeAnywhere},
	{`// (Unordered output|Output):`, markerRestOfGroup, false, scopeExampleFuncs},
	{`//\s?apicover:ignore`, markerLine, false, scopeAnywhere},
	{`.*IPC-protocol-allowed`, markerGroup, false, scopeEnvtaintFiles},
}

var (
	directive       = anyMarker(func(markerScope) bool { return true })
	directiveAlways = anyMarker(func(sc markerScope) bool { return sc == scopeAnywhere })
	directiveEnv    = anyMarker(func(sc markerScope) bool { return sc == scopeEnvtaintFiles })
	directiveInEx   = anyMarker(func(sc markerScope) bool { return sc == scopeExampleFuncs })
	commentMarkers  = compiledMarkers()
)

func anyMarker(include func(markerScope) bool) *regexp.Regexp {
	var alternatives []string
	for _, rule := range commentMarkerRules {
		if include(rule.scope) {
			alternatives = append(alternatives, rule.pattern)
		}
	}
	return regexp.MustCompile(`^(` + strings.Join(alternatives, "|") + `)`)
}

func readsDirectives(name string, src []byte) func(text string, line int) bool {
	envFile := envscope.Reads(name)
	spans := exampleSpans(name, src)
	return func(text string, line int) bool {
		return directiveAlways.MatchString(text) ||
			envFile && directiveEnv.MatchString(text) ||
			directiveInEx.MatchString(text) && slices.ContainsFunc(spans, func(sp [2]int) bool { return line >= sp[0] && line <= sp[1] })
	}
}

func exampleSpans(name string, src []byte) [][2]int {
	if !strings.HasSuffix(name, "_test.go") {
		return nil
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
	if err != nil {
		return nil
	}
	var spans [][2]int
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && strings.HasPrefix(fn.Name.Name, "Example") {
			spans = append(spans, [2]int{fset.Position(fn.Pos()).Line, fset.Position(fn.End()).Line})
		}
	}
	return spans
}

func compiledMarkers() []commentMarker {
	markers := make([]commentMarker, len(commentMarkerRules))
	for i, rule := range commentMarkerRules {
		markers[i] = commentMarker{pattern: regexp.MustCompile(`^(` + rule.pattern + `)`), extent: rule.extent, anchorsParagraph: rule.anchorsParagraph}
	}
	return markers
}

func anchorsItsParagraph(text string) bool {
	return slices.ContainsFunc(commentMarkers, func(m commentMarker) bool {
		return m.anchorsParagraph && m.pattern.MatchString(text)
	})
}

func markerExtentOf(text string) (markerExtent, bool) {
	for _, m := range commentMarkers {
		if m.pattern.MatchString(text) {
			return m.extent, true
		}
	}
	return markerLine, false
}

func Directives(src []byte) []string {
	var found []string
	for _, d := range scanDirectives(func(text string, _ int) bool { return directive.MatchString(text) }, src) {
		found = append(found, d.text)
	}
	return found
}

type placedDirective struct {
	text        string
	line        int
	nextToken   int
	trailing    bool
	blankBefore bool
	paragraph   bool
}

func anchoredDirectives(name string, src []byte) []string {
	var found []string
	for _, d := range scanDirectives(readsDirectives(name, src), src) {
		found = append(found, fmt.Sprintf("%s @%d trailing=%t blank=%t paragraph=%t", d.text, d.nextToken, d.trailing, d.blankBefore, d.paragraph))
	}
	return found
}

func scanDirectives(accept func(text string, line int) bool, src []byte) []placedDirective {
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))
	var s scanner.Scanner
	s.Init(file, src, nil, scanner.ScanComments)
	lines := strings.Split(string(src), "\n")
	var found, waiting []placedDirective
	tokens, lastCodeLine := 0, 0
	place := func(next, nextLine int) {
		for _, d := range waiting {
			d.nextToken = next
			d.blankBefore = !d.trailing && blankBetween(lines, d.line, nextLine)
			found = append(found, d)
		}
		waiting = nil
	}
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		line := file.PositionFor(pos, false).Line
		switch tok {
		case token.COMMENT:
			if text := strings.TrimSpace(lit); accept(text, line) {
				waiting = append(waiting, placedDirective{
					text: text, line: line, trailing: line == lastCodeLine,
					paragraph: anchorsItsParagraph(text) && opensParagraph(lines, line),
				})
			}
		case token.SEMICOLON:
		default:
			place(tokens, line)
			tokens++
			lastCodeLine = line
		}
	}
	place(-1, len(lines)+1)
	return found
}

func opensParagraph(lines []string, line int) bool {
	if line < 2 {
		return true
	}
	prev := strings.TrimSpace(lines[line-2])
	return !strings.HasPrefix(prev, "//") || prev == "//"
}

func blankBetween(lines []string, from, to int) bool {
	for i := from; i < to-1 && i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			return true
		}
	}
	return false
}
