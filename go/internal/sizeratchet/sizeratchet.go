// Package sizeratchet enforces the 50-line function limit across a whole Go
// module: a function past MaxLines fails unless it is a listed offender, and a
// listed offender's allowance may only shrink until the entry disappears.
package sizeratchet

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MaxLines is the largest function size, in lines, that needs no allowance.
const MaxLines = 50

// FuncSpan is one function declaration's key and size. Key is
// "<slash dir>.<Name>" or "<slash dir>.<Receiver>.<Name>", the dir relative to
// the walked root; Lines runs from the func keyword to the closing brace.
type FuncSpan struct {
	Key   string
	Lines int
}

// Walk measures every function declaration with a body in the non-test .go
// files under root, whatever their build constraints. Directories named vendor
// or testdata, or starting with "." or "_", are skipped; a file that does not
// parse is an error.
func Walk(root string) ([]FuncSpan, error) {
	fset := token.NewFileSet()
	var spans []FuncSpan
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && skipDir(name) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		fileSpans, err := walkFile(fset, root, path)
		spans = append(spans, fileSpans...)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("sizeratchet: walk %s: %w", root, err)
	}
	return spans, nil
}

func skipDir(name string) bool {
	return name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

func walkFile(fset *token.FileSet, root, path string) ([]FuncSpan, error) {
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(root, filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	dir := filepath.ToSlash(rel)
	var spans []FuncSpan
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		lines := fset.Position(fn.End()).Line - fset.Position(fn.Pos()).Line + 1
		spans = append(spans, FuncSpan{Key: funcKey(dir, fn), Lines: lines})
	}
	return spans, nil
}

// funcKey names a method by its receiver's base type so same-named methods on
// different types keep separate allowances.
func funcKey(dir string, fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return dir + "." + fn.Name.Name
	}
	expr := fn.Recv.List[0].Type
	for {
		switch e := expr.(type) {
		case *ast.StarExpr:
			expr = e.X
		case *ast.ParenExpr:
			expr = e.X
		case *ast.IndexExpr:
			expr = e.X
		case *ast.IndexListExpr:
			expr = e.X
		case *ast.Ident:
			return dir + "." + e.Name + "." + fn.Name.Name
		default:
			return fmt.Sprintf("%s.%T.%s", dir, e, fn.Name.Name)
		}
	}
}

// LoadOffenders reads a flat JSON object of key to allowance. Every allowance
// must exceed MaxLines: a function within the limit needs no entry.
func LoadOffenders(path string) (map[string]int, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("sizeratchet: read offender list: %w", err)
	}
	var offenders map[string]int
	if err := json.Unmarshal(body, &offenders); err != nil {
		return nil, fmt.Errorf("sizeratchet: parse offender list %s: %w", path, err)
	}
	if offenders == nil {
		return nil, fmt.Errorf("sizeratchet: offender list %s is not a JSON object", path)
	}
	for key, allowance := range offenders {
		if allowance <= MaxLines {
			return nil, fmt.Errorf("sizeratchet: offender list %s: %s allowance %d is not over %d; remove the entry", path, key, allowance, MaxLines)
		}
	}
	return offenders, nil
}

// Check returns nil when every span fits the ratchet, else one error naming
// every violating key. A key's size is its largest span, so same-named
// declarations in build-tagged files share one allowance. An allowance is a
// ceiling: the violations are an unlisted function past MaxLines and a listed
// one past its allowance. A listed function under its allowance, within
// MaxLines or gone is slack a boundary tighten removes, never a lane's failure.
func Check(spans []FuncSpan, offenders map[string]int) error {
	sizes := make(map[string]int, len(spans))
	for _, s := range spans {
		if s.Lines > sizes[s.Key] {
			sizes[s.Key] = s.Lines
		}
	}
	var problems []string
	for key, n := range sizes {
		if _, listed := offenders[key]; !listed && n > MaxLines {
			problems = append(problems, fmt.Sprintf("%s is %d lines > %d: shrink it to %d or fewer", key, n, MaxLines, MaxLines))
		}
	}
	for key, allowance := range offenders {
		if problem := listedProblem(key, sizes[key], allowance); problem != "" {
			problems = append(problems, problem)
		}
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return errors.New("sizeratchet: function-size ratchet violated (allowances live in go/internal/sizeratchet/offenders.json):\n  " +
		strings.Join(problems, "\n  "))
}

func listedProblem(key string, n, allowance int) string {
	if n > allowance {
		return fmt.Sprintf("%s grew to %d lines > allowance %d: shrink it back; allowances may only shrink", key, n, allowance)
	}
	return ""
}

const OffendersRelPath = "internal/sizeratchet/offenders.json"

func Scan(root string) error {
	spans, err := Walk(root)
	if err != nil {
		return err
	}
	offenders, err := LoadOffenders(filepath.Join(root, filepath.FromSlash(OffendersRelPath)))
	if err != nil {
		return err
	}
	return Check(spans, offenders)
}
