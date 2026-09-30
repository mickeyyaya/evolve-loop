// Package commentaudit proves a Go edit changed only comments, and measures
// how much of a package is comment.
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
	if !slices.Equal(anchoredDirectives(before), anchoredDirectives(after)) {
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

// cgoPreamble returns the comment above `import "C"`, which cgo compiles as C.
func cgoPreamble(file *ast.File) string {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			continue
		}
		for _, spec := range gen.Specs {
			if imp := spec.(*ast.ImportSpec); imp.Path.Value == `"C"` {
				if imp.Doc != nil {
					return imp.Doc.Text()
				}
				if len(gen.Specs) == 1 {
					return gen.Doc.Text()
				}
				return ""
			}
		}
	}
	return ""
}

var directive = regexp.MustCompile(`^(//go:\S|//line |//export |//extern |//nolint(:|\s|$)|// \+build |// Code generated .* DO NOT EDIT\.$|// acs-predicate:|// minimal:|// Deprecated:|// (Unordered )?[Oo]utput:|//\s?apicover:ignore|.*IPC-protocol-allowed)`)

// Directives returns the comments tools or conventions act on: toolchain and
// linter directives, generated headers, deprecation notes, example output,
// apicover ignores, ACS waivers and minimal: markers.
func Directives(src []byte) []string {
	var found []string
	for _, d := range scanDirectives(src) {
		found = append(found, d.text)
	}
	return found
}

// placedDirective is a directive and its place in the code's token stream,
// which a comment-only edit cannot move.
type placedDirective struct {
	text        string
	line        int
	nextToken   int  // index of the next code token; -1 at end of file
	trailing    bool // code precedes it on its own line
	blankBefore bool // a blank line detaches a leading directive from its code
	paragraph   bool // a Deprecated: note opens a doc paragraph, as tools require
}

func anchoredDirectives(src []byte) []string {
	var found []string
	for _, d := range scanDirectives(src) {
		found = append(found, fmt.Sprintf("%s @%d trailing=%t blank=%t paragraph=%t", d.text, d.nextToken, d.trailing, d.blankBefore, d.paragraph))
	}
	return found
}

func scanDirectives(src []byte) []placedDirective {
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
			if text := strings.TrimSpace(lit); directive.MatchString(text) {
				waiting = append(waiting, placedDirective{
					text: text, line: line, trailing: line == lastCodeLine,
					paragraph: strings.HasPrefix(text, "// Deprecated:") && opensParagraph(lines, line),
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

// opensParagraph reports whether the comment at line starts its group or
// follows an empty `//` line.
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
