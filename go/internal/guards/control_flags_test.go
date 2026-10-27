package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

const controlFlagModulePath = "github.com/mickeyyaya/evolve-loop/go"

type envSourceFile struct {
	rel     string
	pkg     string
	file    *ast.File
	imports map[string]string
}

type envReadScan struct {
	fset  *token.FileSet
	files []envSourceFile
	pkgs  map[string][]envSourceFile
}

type envRead struct {
	at   string
	name string
}

func scanEnvModule(t *testing.T, module string) envReadScan {
	t.Helper()
	scan := envReadScan{fset: token.NewFileSet(), pkgs: map[string][]envSourceFile{}}
	err := filepath.WalkDir(module, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "testdata" || d.Name() == "vendor" || d.Name() == "acs") {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(scan.fset, p, nil, 0)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(module, p)
		rel = filepath.ToSlash(rel)
		src := envSourceFile{rel: rel, pkg: path.Join(controlFlagModulePath, path.Dir(rel)), file: file, imports: importNames(file)}
		scan.files = append(scan.files, src)
		scan.pkgs[src.pkg] = append(scan.pkgs[src.pkg], src)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return scan
}

func importNames(file *ast.File) map[string]string {
	out := map[string]string{}
	for _, imp := range file.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		name := path.Base(p)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		out[name] = p
	}
	return out
}

func (s envReadScan) reads() []envRead {
	return s.readsOf(func(name string) bool {
		return strings.HasPrefix(name, "EVOLVE_") || strings.HasPrefix(name, "BRIDGE_")
	})
}

func (s envReadScan) readsOf(keep func(string) bool) []envRead {
	readers := s.readerFuncs()
	var out []envRead
	for _, src := range s.files {
		writes := assignedIndexes(src.file)
		ast.Inspect(src.file, func(n ast.Node) bool {
			for _, name := range s.readNames(src, n, writes, readers) {
				if keep(name) {
					out = append(out, envRead{at: src.rel + ":" + strconv.Itoa(s.fset.Position(n.Pos()).Line), name: name})
				}
			}
			return true
		})
	}
	return out
}

func (s envReadScan) readerFuncs() map[string]bool {
	readers := map[string]bool{}
	for grew := true; grew; {
		grew = false
		for _, src := range s.files {
			for _, fn := range funcBodies(src.file) {
				key := src.pkg + "." + fn.name
				if !readers[key] && s.readsAParam(src, fn, readers) {
					readers[key] = true
					grew = true
				}
			}
		}
	}
	return readers
}

type namedFunc struct {
	name   string
	params []string
	body   *ast.BlockStmt
}

func funcBodies(file *ast.File) []namedFunc {
	var out []namedFunc
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncDecl:
			if n.Body != nil {
				out = append(out, namedFunc{name: n.Name.Name, params: paramNames(n.Type), body: n.Body})
			}
		case *ast.AssignStmt:
			if len(n.Lhs) == 1 && len(n.Rhs) == 1 {
				id, isIdent := n.Lhs[0].(*ast.Ident)
				lit, isLit := n.Rhs[0].(*ast.FuncLit)
				if isIdent && isLit {
					out = append(out, namedFunc{name: id.Name, params: paramNames(lit.Type), body: lit.Body})
				}
			}
		}
		return true
	})
	return out
}

func paramNames(ft *ast.FuncType) []string {
	var out []string
	for _, field := range ft.Params.List {
		for _, n := range field.Names {
			out = append(out, n.Name)
		}
	}
	return out
}

func (s envReadScan) readsAParam(src envSourceFile, fn namedFunc, readers map[string]bool) bool {
	found := false
	ast.Inspect(fn.body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			if s.isEnvReader(src, n.Fun, readers) && slices.ContainsFunc(n.Args, func(a ast.Expr) bool { return isParam(a, fn.params) }) {
				found = true
			}
		case *ast.IndexExpr:
			if isEnvMap(n.X) && isParam(n.Index, fn.params) {
				found = true
			}
		}
		return !found
	})
	return found
}

func isEnvMap(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.Ident:
		return strings.Contains(strings.ToLower(x.Name), "env")
	case *ast.SelectorExpr:
		return strings.Contains(strings.ToLower(x.Sel.Name), "env")
	}
	return false
}

func isParam(e ast.Expr, params []string) bool {
	id, ok := e.(*ast.Ident)
	return ok && slices.Contains(params, id.Name)
}

func (s envReadScan) readNames(src envSourceFile, n ast.Node, writes map[ast.Expr]bool, readers map[string]bool) []string {
	switch n := n.(type) {
	case *ast.CallExpr:
		if !s.isEnvReader(src, n.Fun, readers) {
			return nil
		}
		var names []string
		for _, arg := range n.Args {
			names = append(names, s.fold(src, arg, 0))
		}
		return names
	case *ast.IndexExpr:
		if writes[n] || !isEnvMap(n.X) {
			return nil
		}
		return []string{s.fold(src, n.Index, 0)}
	case *ast.KeyValueExpr:
		if id, ok := n.Key.(*ast.Ident); ok && id.Name == "key" {
			return []string{s.fold(src, n.Value, 0)}
		}
	}
	return nil
}

func assignedIndexes(file *ast.File) map[ast.Expr]bool {
	out := map[ast.Expr]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		if as, ok := n.(*ast.AssignStmt); ok {
			for _, lhs := range as.Lhs {
				out[lhs] = true
			}
		}
		return true
	})
	return out
}

var envchainReaders = []string{"Resolve", "Int", "IntMin", "Bool"}

func (s envReadScan) isEnvReader(src envSourceFile, fun ast.Expr, readers map[string]bool) bool {
	switch f := fun.(type) {
	case *ast.Ident:
		return envReaderName(f.Name) || readers[src.pkg+"."+f.Name]
	case *ast.SelectorExpr:
		if pkg, ok := f.X.(*ast.Ident); ok && src.imports[pkg.Name] != "" {
			imported := src.imports[pkg.Name]
			return (imported == controlFlagModulePath+"/internal/envchain" && slices.Contains(envchainReaders, f.Sel.Name)) || envReaderName(f.Sel.Name) || readers[imported+"."+f.Sel.Name]
		}
		return envReaderName(f.Sel.Name) || readers[src.pkg+"."+f.Sel.Name]
	}
	return false
}

func envReaderName(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "getenv") || strings.Contains(lower, "lookupenv") || strings.Contains(lower, "envvalue")
}

func (s envReadScan) fold(src envSourceFile, e ast.Expr, depth int) string {
	if depth > 16 {
		return "*"
	}
	switch e := e.(type) {
	case *ast.BasicLit:
		if v, err := strconv.Unquote(e.Value); err == nil && e.Kind == token.STRING {
			return v
		}
	case *ast.ParenExpr:
		return s.fold(src, e.X, depth+1)
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			return joinWildcards(s.fold(src, e.X, depth+1) + s.fold(src, e.Y, depth+1))
		}
	case *ast.Ident:
		return s.foldIdent(src, e, depth)
	case *ast.SelectorExpr:
		if pkg, ok := e.X.(*ast.Ident); ok && src.imports[pkg.Name] != "" {
			return s.foldTopLevel(src.imports[pkg.Name], e.Sel.Name, depth)
		}
	case *ast.CallExpr:
		return s.foldCall(src, e, depth)
	}
	return "*"
}

func (s envReadScan) foldIdent(src envSourceFile, id *ast.Ident, depth int) string {
	if id.Obj == nil {
		return s.foldTopLevel(src.pkg, id.Name, depth)
	}
	switch decl := id.Obj.Decl.(type) {
	case *ast.ValueSpec:
		for i, name := range decl.Names {
			if name.Name == id.Name && i < len(decl.Values) {
				return s.fold(src, decl.Values[i], depth+1)
			}
		}
	case *ast.AssignStmt:
		if len(decl.Lhs) == len(decl.Rhs) {
			for i, lhs := range decl.Lhs {
				if l, ok := lhs.(*ast.Ident); ok && l.Name == id.Name {
					return s.fold(src, decl.Rhs[i], depth+1)
				}
			}
		}
	}
	return "*"
}

func (s envReadScan) foldTopLevel(pkg, name string, depth int) string {
	for _, src := range s.pkgs[pkg] {
		for _, decl := range src.file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || (gen.Tok != token.CONST && gen.Tok != token.VAR) {
				continue
			}
			for _, spec := range gen.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, n := range vs.Names {
					if n.Name == name && i < len(vs.Values) {
						return s.fold(src, vs.Values[i], depth+1)
					}
				}
			}
		}
	}
	return "*"
}

func (s envReadScan) foldCall(src envSourceFile, call *ast.CallExpr, depth int) string {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) != 2 {
		return "*"
	}
	if pkg, ok := sel.X.(*ast.Ident); ok && src.imports[pkg.Name] == controlFlagModulePath+"/internal/envchain" && sel.Sel.Name == "PhaseEnvKey" {
		return "EVOLVE_*_" + s.fold(src, call.Args[1], depth+1)
	}
	return "*"
}

func joinWildcards(name string) string {
	for strings.Contains(name, "**") {
		name = strings.ReplaceAll(name, "**", "*")
	}
	return name
}

const unmovedControlCeiling = 10

func declaredControlNames() []string {
	return slices.Concat(ipcenv.Markers(), ipcenv.UnmovedControls())
}

func undeclaredReads(reads []envRead, declared []string) []envRead {
	var out []envRead
	for _, r := range reads {
		if !slices.ContainsFunc(declared, func(pattern string) bool { return controlNameMatches(pattern, r.name) }) {
			out = append(out, r)
		}
	}
	return out
}

func controlNameMatches(pattern, name string) bool {
	matched, _ := path.Match(pattern, name)
	return matched
}

func TestNoControlFlagReadOutsideTheMarkerList(t *testing.T) {
	module, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	scan := scanEnvModule(t, module)
	if len(scan.files) < 500 {
		t.Fatalf("scanned only %d files under %s; the guard lost its corpus", len(scan.files), module)
	}
	reads := scan.reads()
	for _, r := range undeclaredReads(reads, declaredControlNames()) {
		t.Errorf("%s reads %s from the environment, and the name is not a pipeline marker (ipcenv.Markers), a control not yet moved (ipcenv.UnmovedControls). A test seam is an injected field, never an environment read. Read no control value from the environment: put a setting in policy.json and a bypass behind a CLI parameter (I-16, ADR-0156)", r.at, r.name)
	}
	for _, name := range ipcenv.UnmovedControls() {
		if !slices.ContainsFunc(reads, func(r envRead) bool { return controlNameMatches(name, r.name) }) {
			t.Errorf("%s is declared as a control not yet moved, but no production code reads it: remove it from the list", name)
		}
	}
}

func TestControlFlagLists_OnlyShrink(t *testing.T) {
	if n := len(ipcenv.UnmovedControls()); n > unmovedControlCeiling {
		t.Errorf("ipcenv.UnmovedControls has %d names, ceiling %d: a new control goes into policy.json or a CLI parameter, never into the environment", n, unmovedControlCeiling)
	}
}

func TestUndeclaredReads_CatchAConstructedNameThroughAHelperAndAnotherPackage(t *testing.T) {
	module := t.TempDir()
	writeGoFile(t, module, filepath.Join("internal", "names", "names.go"), `package names

const Prefix = "EVOLVE_"
`)
	writeGoFile(t, module, filepath.Join("internal", "core", "seat.go"), `package core

import (
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/names"
)

func value(key string) string { return os.Getenv(key) }

func knob() string {
	key := names.Prefix + "X"
	return value(key) + os.Getenv("EVOLVE_"+"Y") + os.Getenv("EVOLVE_PROJECT_ROOT")
}
`)
	writeGoFile(t, module, filepath.Join("internal", "core", "seat_test.go"), `package core

import "os"

var _ = os.Getenv("EVOLVE_" + "Z")
`)
	got := undeclaredReads(scanEnvModule(t, module).reads(), declaredControlNames())
	var names []string
	for _, r := range got {
		names = append(names, r.name)
	}
	if !slices.Equal(names, []string{"EVOLVE_X", "EVOLVE_Y"}) {
		t.Fatalf("undeclared reads = %v, want [EVOLVE_X EVOLVE_Y]: a name built from a constant of another package and read through a helper, and a concatenated literal, both count; a marker and a test file do not", got)
	}
}

func TestFlagRegistryRows_EachHasAProductionReader(t *testing.T) {
	module, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	reads := scanEnvModule(t, module).reads()
	for _, row := range flagregistry.All {
		if !slices.ContainsFunc(reads, func(r envRead) bool { return controlNameMatches(r.name, row.Name) }) {
			t.Errorf("flag registry row %s has no production reader: delete the row and each place that names the flag (I-16, X18e)", row.Name)
		}
	}
}

var removedControlNames = []string{
	"EVOLVE_REFLECTION_JOURNAL", "EVOLVE_RESUME", "EVOLVE_RESUME_MODE", "EVOLVE_SANDBOX_FALLBACK_ON_EPERM", "EVOLVE_RELEASE_STRICT_PASS",
	"EVOLVE_PROFILES_DIR_OVERRIDE", "EVOLVE_ADAPTERS_DIR_OVERRIDE", "EVOLVE_LEDGER_OVERRIDE", "EVOLVE_PROMPTS_DIR",
	"BRIDGE_TESTING", "BRIDGE_*_BINARY", "CG_TEST_INSTALL", "CG_TEST_FORCE_MISSING",
}

func TestRemovedControlNames_HaveNoProductionRead(t *testing.T) {
	if len(removedControlNames) < 13 {
		t.Fatalf("removedControlNames has %d names, want at least 13: a deleted name stays on the list so no reader comes back", len(removedControlNames))
	}
	module, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	reads := scanEnvModule(t, module).readsOf(func(name string) bool {
		return slices.ContainsFunc(removedControlNames, func(removed string) bool { return controlNameMatches(removed, name) })
	})
	for _, r := range reads {
		t.Errorf("%s reads %s, a dead name or a test seam of control-flags.md classes (d) and (e): a test sets an injected field instead (X18e)", r.at, r.name)
	}
}

func TestRetiredControls_HaveNoProductionRead(t *testing.T) {
	module, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	reads := scanEnvModule(t, module).readsOf(func(name string) bool {
		return slices.ContainsFunc(ipcenv.RetiredControls(), func(r ipcenv.RetiredControl) bool { return controlNameMatches(r.Name, name) })
	})
	for _, r := range reads {
		t.Errorf("%s reads %s, a control that moved to policy.json or a CLI parameter: read the replacement that ipcenv.RetiredControls names (X18c)", r.at, r.name)
	}
}
