//go:build acs

package acsnames

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var (
	definedRE = regexp.MustCompile(`func ((?:Test|Fuzz)\w*)\(`)
	nameRE    = regexp.MustCompile(`^(?:Test|Fuzz)\w+$`)
	groupRE   = regexp.MustCompile(`^(\w*)\((\w+(?:\|\w+)*)\)(\w*)$`)
)

type alternative struct {
	name       string
	head, tail bool
}

type reference struct {
	alternative
	pkg  string
	file string
	line int
}

type predicatePackage struct {
	fset  *token.FileSet
	files []*ast.File
}

func findViolations(goDir string, absent map[string]string) ([]string, error) {
	declared, err := declaredTests(goDir)
	if err != nil {
		return nil, err
	}
	refs, err := predicateReferences(goDir)
	if err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return nil, errors.New("no predicate package under go/acs names a test to run: the scan reads nothing, so it checks nothing")
	}
	var violations []string
	used := map[string]bool{}
	for _, ref := range refs {
		if resolves(declared, ref.alternative) {
			continue
		}
		key := ref.pkg + "/" + ref.name
		if _, listed := absent[key]; listed {
			used[key] = true
			continue
		}
		violations = append(violations, describe(goDir, ref))
	}
	for _, key := range slices.Sorted(maps.Keys(absent)) {
		if !used[key] {
			violations = append(violations, fmt.Sprintf("absent list entry %q matches no unresolved name: remove it, because the predicate no longer names it or a test now declares it", key))
		}
	}
	return violations, nil
}

func predicateReferences(goDir string) ([]reference, error) {
	acsDir := filepath.Join(goDir, "acs")
	filesByDir, err := goFilesByDirectory(acsDir, filepath.Join(acsDir, "regression", "acsnames"))
	if err != nil {
		return nil, err
	}
	var refs []reference
	for _, dir := range slices.Sorted(maps.Keys(filesByDir)) {
		found, err := referencesIn(acsDir, dir, filesByDir[dir])
		if err != nil {
			return nil, err
		}
		refs = append(refs, found...)
	}
	return refs, nil
}

func declaredTests(goDir string) (map[string]bool, error) {
	declared := map[string]bool{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(goDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && skippedDirectory(d.Name()) {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		for _, decl := range file.Decls {
			if fn, isFunc := decl.(*ast.FuncDecl); isFunc && fn.Recv == nil && nameRE.MatchString(fn.Name.Name) {
				declared[fn.Name.Name] = true
			}
		}
		return nil
	})
	return declared, err
}

func skippedDirectory(name string) bool {
	return name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".")
}

func goFilesByDirectory(acsDir, skip string) (map[string][]string, error) {
	byDir := map[string][]string{}
	err := filepath.WalkDir(acsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" || path == skip {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			byDir[filepath.Dir(path)] = append(byDir[filepath.Dir(path)], path)
		}
		return nil
	})
	return byDir, err
}

func referencesIn(acsDir, dir string, files []string) ([]reference, error) {
	pkg, err := loadPackage(files)
	if err != nil {
		return nil, err
	}
	if !pkg.runsByName() {
		return nil, nil
	}
	fixtures, err := definedFixtures(dir)
	if err != nil {
		return nil, err
	}
	name := filepath.ToSlash(strings.TrimPrefix(dir, acsDir+string(os.PathSeparator)))
	var refs []reference
	for _, ref := range pkg.references() {
		if !fixtures[ref.name] {
			ref.pkg = name
			refs = append(refs, ref)
		}
	}
	return refs, nil
}

func loadPackage(files []string) (predicatePackage, error) {
	pkg := predicatePackage{fset: token.NewFileSet()}
	for _, path := range files {
		file, err := parser.ParseFile(pkg.fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return predicatePackage{}, err
		}
		pkg.files = append(pkg.files, file)
	}
	return pkg, nil
}

func (p predicatePackage) runsByName() bool {
	found := false
	for _, file := range p.files {
		ast.Inspect(file, func(n ast.Node) bool {
			if value, isString := stringValue(n); isString {
				found = found || isRunFlag(value)
			}
			if selector, isSelector := n.(*ast.SelectorExpr); isSelector {
				found = found || selector.Sel.Name == "GoTests"
			}
			return !found
		})
	}
	return found
}

func isRunFlag(value string) bool {
	return value == "-run" || value == "-test.run" || strings.HasPrefix(value, "-run=") || strings.HasPrefix(value, "-test.run=")
}

func (p predicatePackage) references() []reference {
	skipped := p.stringsCallArguments()
	var refs []reference
	for _, file := range p.files {
		ast.Inspect(file, func(n ast.Node) bool {
			value, isString := stringValue(n)
			if !isString || skipped[n.Pos()] {
				return true
			}
			position := p.fset.Position(n.Pos())
			for _, alt := range parseAlternatives(value) {
				refs = append(refs, reference{alternative: alt, file: position.Filename, line: position.Line})
			}
			return true
		})
	}
	return refs
}

func (p predicatePackage) stringsCallArguments() map[token.Pos]bool {
	skipped := map[token.Pos]bool{}
	for _, file := range p.files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, isCall := n.(*ast.CallExpr)
			if !isCall || !inspectsText(call) {
				return true
			}
			for _, arg := range call.Args {
				if _, isString := stringValue(arg); isString {
					skipped[arg.Pos()] = true
				}
			}
			return true
		})
	}
	return skipped
}

func inspectsText(call *ast.CallExpr) bool {
	selector, isSelector := call.Fun.(*ast.SelectorExpr)
	if !isSelector {
		return false
	}
	pkg, isIdent := selector.X.(*ast.Ident)
	return isIdent && pkg.Name == "strings"
}

func stringValue(node ast.Node) (string, bool) {
	lit, isLit := node.(*ast.BasicLit)
	if !isLit || lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	return value, err == nil
}

func definedFixtures(dir string) (map[string]bool, error) {
	defined := map[string]bool{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range definedRE.FindAllSubmatch(source, -1) {
			defined[string(match[1])] = true
		}
		return nil
	})
	return defined, err
}

func parseAlternatives(value string) []alternative {
	text, _, _ := strings.Cut(strings.TrimSpace(value), "/")
	text, atHead := strings.CutPrefix(text, "^")
	text, atTail := strings.CutSuffix(text, "$")
	parts := strings.Split(strings.Trim(text, "|"), "|")
	grouped := false
	if group := groupRE.FindStringSubmatch(text); group != nil {
		grouped = true
		parts = nil
		for _, member := range strings.Split(group[2], "|") {
			parts = append(parts, group[1]+member+group[3])
		}
	}
	alternatives := make([]alternative, 0, len(parts))
	for i, part := range parts {
		part, partHead := strings.CutPrefix(part, "^")
		part, partTail := strings.CutSuffix(part, "$")
		if !nameRE.MatchString(part) {
			return nil
		}
		alternatives = append(alternatives, alternative{
			name: part,
			head: partHead || (atHead && (grouped || i == 0)),
			tail: partTail || (atTail && (grouped || i == len(parts)-1)),
		})
	}
	return alternatives
}

func (a alternative) pattern() string {
	pattern := regexp.QuoteMeta(a.name)
	if a.head {
		pattern = "^" + pattern
	}
	if a.tail {
		pattern += "$"
	}
	return pattern
}

func resolves(declared map[string]bool, alt alternative) bool {
	if declared[alt.name] {
		return true
	}
	matcher := regexp.MustCompile(alt.pattern())
	for name := range declared {
		if matcher.MatchString(name) {
			return true
		}
	}
	return false
}

func describe(goDir string, ref reference) string {
	file := filepath.ToSlash(strings.TrimPrefix(ref.file, goDir+string(os.PathSeparator)))
	return fmt.Sprintf("go/%s:%d: %q is named by a predicate package that runs tests by name, but no test function matches %s: repoint the predicate to the renamed test, or archive it under the A2 retention rule", file, ref.line, ref.name, ref.pattern())
}
