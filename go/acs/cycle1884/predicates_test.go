//go:build acs

package cycle1884

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/treefence"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const gitexecImport = "github.com/mickeyyaya/evolve-loop/go/internal/gitexec"

var ambientNames = []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR"}

func committedWorktree(t *testing.T) *gittest.Repo {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write("src/keep.go", "package src\n")
	repo.Write("src/mat.go", "package src\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	repo.Write("src/mat.go", "package src\n\nvar changed = true\n")
	repo.Write("src/untracked.go", "package src\n")
	return repo
}

func parseProductionFiles(t *testing.T, dir string) (*token.FileSet, []*ast.File) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		files = append(files, f)
	}
	return fset, files
}

func importsPath(files []*ast.File, path string) bool {
	for _, f := range files {
		for _, imp := range f.Imports {
			if p, _ := strconv.Unquote(imp.Path.Value); p == path {
				return true
			}
		}
	}
	return false
}

func literalListsAllAmbientNames(lit *ast.CompositeLit) bool {
	seen := map[string]bool{}
	ast.Inspect(lit, func(n ast.Node) bool {
		if b, ok := n.(*ast.BasicLit); ok && b.Kind == token.STRING {
			if s, err := strconv.Unquote(b.Value); err == nil {
				seen[s] = true
			}
		}
		return true
	})
	for _, name := range ambientNames {
		if !seen[name] {
			return false
		}
	}
	return true
}

func ambientListSites(t *testing.T, internalDir string) []string {
	t.Helper()
	var sites []string
	err := filepath.WalkDir(internalDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		fset, files := parseProductionFiles(t, path)
		for _, f := range files {
			ast.Inspect(f, func(n ast.Node) bool {
				if lit, ok := n.(*ast.CompositeLit); ok && literalListsAllAmbientNames(lit) {
					rel, _ := filepath.Rel(internalDir, fset.Position(lit.Pos()).Filename)
					sites = append(sites, filepath.ToSlash(rel))
				}
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", internalDir, err)
	}
	return sites
}

func selectorCalls(files []*ast.File, pkg, name string) int {
	count := 0
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != name {
				return true
			}
			if pkg == "" {
				count++
				return true
			}
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == pkg {
				count++
			}
			return true
		})
	}
	return count
}

func TestC1884_001_AmbientGitVarsAreListedOnlyInGitexecAndTreefenceStillIgnoresThem(t *testing.T) {
	root := acsassert.RepoRoot(t)
	internalDir := filepath.Join(root, "go", "internal")

	sites := ambientListSites(t, internalDir)
	for _, site := range sites {
		if !strings.HasPrefix(site, "gitexec/") {
			t.Errorf("the four ambient git variable names are listed outside internal/gitexec at %s (all sites: %v)", site, sites)
		}
	}
	if !slices.ContainsFunc(sites, func(s string) bool { return strings.HasPrefix(s, "gitexec/") }) {
		t.Errorf("internal/gitexec must own the one list of ambient git variable names; sites=%v", sites)
	}
	_, treefenceFiles := parseProductionFiles(t, filepath.Join(internalDir, "treefence"))
	if !importsPath(treefenceFiles, gitexecImport) {
		t.Errorf("treefence must import %s to reach the ambient-variable isolation", gitexecImport)
	}

	repo := committedWorktree(t)
	decoy := gittest.Fixture(t)
	decoy.Write("decoy.txt", "decoy\n")
	decoy.Git("add", "-A")
	decoyIndex := filepath.Join(decoy.Dir, ".git", "index")
	decoyBefore, err := os.ReadFile(decoyIndex)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_DIR", filepath.Join(decoy.Dir, ".git"))
	t.Setenv("GIT_WORK_TREE", decoy.Dir)
	t.Setenv("GIT_INDEX_FILE", decoyIndex)
	t.Setenv("GIT_COMMON_DIR", filepath.Join(decoy.Dir, ".git"))
	snap, err := treefence.Take(context.Background(), repo.Dir)
	if err != nil {
		t.Fatalf("Take under ambient git variables: %v", err)
	}
	for _, name := range ambientNames {
		os.Unsetenv(name)
	}
	if listing := repo.Git("ls-tree", "-r", "--name-only", snap.Tree); !strings.Contains(listing, "src/untracked.go") || strings.Contains(listing, "decoy.txt") {
		t.Errorf("Take must hash the given worktree, not the ambient GIT_DIR/GIT_WORK_TREE target; tree lists:\n%s", listing)
	}
	if after, err := os.ReadFile(decoyIndex); err != nil || string(after) != string(decoyBefore) {
		t.Errorf("Take must never write the ambient GIT_INDEX_FILE (%v)", err)
	}
}

func TestC1884_002_TakeWritesItsTreeThroughGitexecTreeOfIndexCopy(t *testing.T) {
	root := acsassert.RepoRoot(t)
	_, treefenceFiles := parseProductionFiles(t, filepath.Join(root, "go", "internal", "treefence"))
	if n := selectorCalls(treefenceFiles, "", "TreeOfIndexCopy"); n == 0 {
		t.Errorf("treefence must write its snapshot trees through gitexec.Git.TreeOfIndexCopy; found %d call sites", n)
	}
	for _, f := range treefenceFiles {
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && (fn.Name.Name == "writeTreeMode" || fn.Name.Name == "copyIndex") {
				t.Errorf("treefence still defines its private scratch-index writer %s", fn.Name.Name)
			}
		}
	}

	repo := committedWorktree(t)
	ctx := context.Background()
	indexPath := filepath.Join(repo.Dir, ".git", "index")
	before, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := treefence.Take(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("Take: %v", err)
	}
	want, err := gitexec.Isolated(repo.Dir).TreeOfIndexCopy(ctx, []string{"add", "-A", "--", "."})
	if err != nil {
		t.Fatalf("TreeOfIndexCopy: %v", err)
	}
	if snap.Tree != want {
		t.Errorf("Take tree %s differs from gitexec.TreeOfIndexCopy tree %s", snap.Tree, want)
	}
	if after, err := os.ReadFile(indexPath); err != nil || string(after) != string(before) {
		t.Errorf("Take must leave the real index untouched (%v)", err)
	}
}

func TestC1884_003_TakeOnAWorktreeWithoutAnIndexIsRefused(t *testing.T) {
	repo := gittest.Fixture(t)
	repo.Write("src/only.go", "package src\n")
	if _, err := os.Stat(filepath.Join(repo.Dir, ".git", "index")); !os.IsNotExist(err) {
		t.Fatalf("fixture precondition: a fresh repository with nothing staged has no index (%v)", err)
	}

	snap, err := treefence.Take(context.Background(), repo.Dir)

	if err == nil {
		t.Fatalf("Take on a worktree whose index cannot be read must be refused like gitexec.TreeOfIndexCopy, got tree %q", snap.Tree)
	}
	if snap.Tree != "" {
		t.Errorf("a refused Take carries no tree, got %q", snap.Tree)
	}
	fence := treefence.Begin(context.Background(), repo.Dir, true)
	if fence.TakeErr() == nil {
		t.Errorf("Begin on a read-only dispatch without an index must surface the snapshot refusal through TakeErr")
	}
}

func TestC1884_004_TakeStagedWithoutAnIndexStaysRefused(t *testing.T) {
	repo := gittest.Fixture(t)
	repo.Write("src/only.go", "package src\n")

	snap, err := treefence.TakeStaged(context.Background(), repo.Dir, []string{"src/only.go"})

	if err == nil || snap.Tree != "" {
		t.Fatalf("TakeStaged without a readable real index must be refused, got tree=%q err=%v", snap.Tree, err)
	}
}

func TestC1884_005_NoIndexCopyRoutineOutsideGitexec(t *testing.T) {
	root := acsassert.RepoRoot(t)
	internalDir := filepath.Join(root, "go", "internal")
	var offenders []string
	err := filepath.WalkDir(internalDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(internalDir, path)
		if filepath.ToSlash(rel) == "gitexec" {
			return nil
		}
		fset, files := parseProductionFiles(t, path)
		for _, f := range files {
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				if selectorCalls([]*ast.File{{Decls: []ast.Decl{fn}}}, "os", "Chtimes") == 0 {
					continue
				}
				if strings.Contains(strings.ToLower(fn.Name.Name), "index") {
					offenders = append(offenders, fset.Position(fn.Pos()).String()+" "+fn.Name.Name)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Errorf("an index-copy routine calling os.Chtimes remains outside internal/gitexec: %v", offenders)
	}
	_, gitexecFiles := parseProductionFiles(t, filepath.Join(internalDir, "gitexec"))
	if selectorCalls(gitexecFiles, "os", "Chtimes") == 0 {
		t.Errorf("internal/gitexec must keep the one index copy that preserves the index mtime")
	}
}
