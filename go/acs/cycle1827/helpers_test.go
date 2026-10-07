//go:build acs

package cycle1827

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const atomicwriteImportPath = "github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"

func seedLedger(t *testing.T, n int) (*ledger.FileLedger, string) {
	t.Helper()
	dir := t.TempDir()
	l := ledger.New(dir)
	for i := 0; i < n; i++ {
		if err := l.Append(context.Background(), core.LedgerEntry{
			Cycle: i, Role: "builder", Kind: "phase_complete", Message: fmt.Sprintf("entry-%d", i),
		}); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}
	return l, dir
}

func appendEntries(t *testing.T, l *ledger.FileLedger, from, n int) {
	t.Helper()
	for i := from; i < from+n; i++ {
		if err := l.Append(context.Background(), core.LedgerEntry{
			Cycle: i, Role: "auditor", Kind: "phase_complete", Message: fmt.Sprintf("entry-%d", i),
		}); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}
}

func sealOrFail(t *testing.T, l *ledger.FileLedger, keepTail int) {
	t.Helper()
	if err := l.Seal(context.Background(), keepTail); err != nil {
		t.Fatalf("Seal(%d): %v", keepTail, err)
	}
}

func liveLines(t *testing.T, dir string) []string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(body), "\n"), "\n")
}

func writeLiveLines(t *testing.T, dir string, lines []string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func pointTipAt(t *testing.T, dir, line string) {
	t.Helper()
	var e core.LedgerEntry
	if err := json.Unmarshal([]byte(line), &e); err != nil {
		t.Fatalf("fixture: the new last live line does not decode: %v", err)
	}
	sum := sha256.Sum256([]byte(line))
	tip := fmt.Sprintf("%d:%s", e.EntrySeq, hex.EncodeToString(sum[:]))
	if err := os.WriteFile(filepath.Join(dir, "ledger.tip"), []byte(tip), 0o644); err != nil {
		t.Fatal(err)
	}
}

func segmentCount(t *testing.T, dir string) int {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "ledger-segments", "seg-*.jsonl.gz"))
	if err != nil {
		t.Fatal(err)
	}
	return len(matches)
}

func waitUntilLockHeldElsewhere(t *testing.T, path string) {
	t.Helper()
	for poll := 0; poll < 400; poll++ {
		release, held, err := flock.TryLock(path)
		if err != nil {
			t.Fatalf("probing %s: %v", path, err)
		}
		if held {
			return
		}
		release()
		time.Sleep(5 * time.Millisecond)
	}
}

type sealRace struct {
	sealErr, verifyErr error
}

func verifyDeepDuringSeal(t *testing.T, l *ledger.FileLedger, dir string, keepTail int) sealRace {
	t.Helper()
	const settle = 40 * time.Millisecond
	ctx := context.Background()
	releaseChainLock, err := flock.Lock(filepath.Join(dir, "ledger.lock"))
	if err != nil {
		t.Fatal(err)
	}
	sealDone := make(chan error, 1)
	go func() { sealDone <- l.Seal(ctx, keepTail) }()
	waitUntilLockHeldElsewhere(t, filepath.Join(dir, "ledger.lock.seal"))
	time.Sleep(settle)
	verifyDone := make(chan error, 1)
	go func() { verifyDone <- l.VerifyDeep(ctx) }()
	time.Sleep(settle)
	releaseChainLock()
	deadlockGuard := time.After(2 * time.Minute)
	var race sealRace
	for pending := 2; pending > 0; pending-- {
		select {
		case race.sealErr = <-sealDone:
			sealDone = nil
		case race.verifyErr = <-verifyDone:
			verifyDone = nil
		case <-deadlockGuard:
			t.Fatal("Seal and a concurrent VerifyDeep deadlocked")
		}
	}
	return race
}

func goTestPackage(t *testing.T, pkg string, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", append(append([]string{"test", "-count=1"}, args...), pkg)...)
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func requirePassed(t *testing.T, out string, names ...string) {
	t.Helper()
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("%s did not run and pass", name)
		}
	}
}

type packageSource struct {
	funcs    map[string]*ast.FuncDecl
	vars     map[string]*ast.ValueSpec
	fileOf   map[ast.Node]*ast.File
	awLocals map[*ast.File]string
}

func parsePackageSource(t *testing.T, pkgRel string) packageSource {
	t.Helper()
	dir := filepath.Join(acsassert.RepoRoot(t), "go", filepath.FromSlash(pkgRel))
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi fs.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", dir, err)
	}
	src := packageSource{funcs: map[string]*ast.FuncDecl{}, vars: map[string]*ast.ValueSpec{}, fileOf: map[ast.Node]*ast.File{}, awLocals: map[*ast.File]string{}}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			src.indexFile(file)
		}
	}
	return src
}

func (src packageSource) indexFile(file *ast.File) {
	for _, imp := range file.Imports {
		if strings.Trim(imp.Path.Value, `"`) == atomicwriteImportPath {
			local := "atomicwrite"
			if imp.Name != nil {
				local = imp.Name.Name
			}
			src.awLocals[file] = local
		}
	}
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil {
				src.funcs[d.Name.Name] = d
				src.fileOf[d] = file
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok {
					for _, name := range vs.Names {
						src.vars[name.Name] = vs
					}
					src.fileOf[vs] = file
				}
			}
		}
	}
}

func (src packageSource) packageLevel(id *ast.Ident) (*ast.FuncDecl, *ast.ValueSpec) {
	if id.Obj != nil && id.Obj.Kind != ast.Fun && id.Obj.Kind != ast.Var {
		return nil, nil
	}
	if fn, ok := src.funcs[id.Name]; ok && (id.Obj == nil || id.Obj.Decl == fn) {
		return fn, nil
	}
	if vs, ok := src.vars[id.Name]; ok && (id.Obj == nil || id.Obj.Decl == vs) {
		return nil, vs
	}
	return nil, nil
}

var ownDurableWriteCalls = map[string]bool{"createtemp": true, "create": true, "openfile": true, "writefile": true, "rename": true, "sync": true}

func (src packageSource) atomicwriteRef(file *ast.File, sel *ast.SelectorExpr) (string, bool) {
	x, ok := sel.X.(*ast.Ident)
	if !ok || src.awLocals[file] == "" || x.Name != src.awLocals[file] {
		return "", false
	}
	return sel.Sel.Name, true
}

type delegationWalk struct {
	src            packageSource
	problems       []string
	reachesDurable bool
	seenFuncs      map[*ast.FuncDecl]bool
	seenVars       map[*ast.ValueSpec]bool
	queue          []*ast.FuncDecl
}

func durableDelegationProblems(t *testing.T, pkgRel, funcName string) []string {
	t.Helper()
	src := parsePackageSource(t, pkgRel)
	root, ok := src.funcs[funcName]
	if !ok {
		return []string{fmt.Sprintf("%s.%s is gone; the acceptance names it as a writer that delegates", pkgRel, funcName)}
	}
	w := &delegationWalk{src: src, seenFuncs: map[*ast.FuncDecl]bool{}, seenVars: map[*ast.ValueSpec]bool{}, queue: []*ast.FuncDecl{root}}
	for len(w.queue) > 0 {
		fn := w.queue[0]
		w.queue = w.queue[1:]
		if w.seenFuncs[fn] || fn.Body == nil {
			continue
		}
		w.seenFuncs[fn] = true
		w.visit(fn, src.fileOf[fn], fn.Body)
	}
	if !w.reachesDurable {
		w.problems = append(w.problems, fmt.Sprintf("%s.%s never reaches atomicwrite.Durable", pkgRel, funcName))
	}
	return w.problems
}

func (w *delegationWalk) visit(fn *ast.FuncDecl, file *ast.File, node ast.Node) {
	ast.Inspect(node, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.SelectorExpr:
			if name, ok := w.src.atomicwriteRef(file, v); ok {
				w.noteAtomicwrite(fn.Name.Name, name)
				return false
			}
			w.visit(fn, file, v.X)
			return false
		case *ast.CallExpr:
			if sel, ok := v.Fun.(*ast.SelectorExpr); ok && ownDurableWriteCalls[strings.ToLower(sel.Sel.Name)] {
				if _, isAW := w.src.atomicwriteRef(file, sel); !isAW {
					w.problems = append(w.problems, fmt.Sprintf("%s still performs its own %s step (%s)", fn.Name.Name, sel.Sel.Name, exprText(sel)))
				}
			}
		case *ast.Ident:
			callee, spec := w.src.packageLevel(v)
			if callee != nil && callee != fn {
				w.queue = append(w.queue, callee)
			}
			if spec != nil && !w.seenVars[spec] {
				w.seenVars[spec] = true
				if w.src.specReferencesDurable(spec) {
					w.reachesDurable = true
				}
			}
		}
		return true
	})
}

func (w *delegationWalk) noteAtomicwrite(caller, name string) {
	if name == "Durable" {
		w.reachesDurable = true
		return
	}
	w.problems = append(w.problems, fmt.Sprintf("%s reaches atomicwrite.%s, which is not the durable variant", caller, name))
}

func (src packageSource) specReferencesDurable(spec *ast.ValueSpec) bool {
	file := src.fileOf[spec]
	found := false
	for _, value := range spec.Values {
		ast.Inspect(value, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				if name, ok := src.atomicwriteRef(file, sel); ok && name == "Durable" {
					found = true
				}
			}
			return !found
		})
	}
	return found
}

func exprText(sel *ast.SelectorExpr) string {
	if x, ok := sel.X.(*ast.Ident); ok {
		return x.Name + "." + sel.Sel.Name
	}
	return "." + sel.Sel.Name
}

func entryNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("listing %s: %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func permOf(t *testing.T, path string) fs.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}
