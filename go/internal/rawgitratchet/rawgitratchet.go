// Package rawgitratchet keeps new raw git fixtures out of the module's tests.
// A test outside internal/gittest that builds its own git repository fails the
// ratchet unless its file is listed in baseline.json, and a listed file's
// count of init sites may only shrink until its entry disappears. The fixture
// home is internal/gittest, whose repos keep git's detached maintenance child
// from racing t.TempDir() removal.
//
// The scan binds the module's git-tracked test files, index-staged included,
// so an untracked file never reds it (ADR-0084 I1). When git cannot answer —
// the module is not in a work tree — every on-disk test file is bound instead:
// the strict direction, never bind-none.
package rawgitratchet

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/repostate"
)

// OwnerDir is the module-relative directory whose tests may build git
// repositories: the sanctioned fixture package and everything under it.
const OwnerDir = "internal/gittest"

// BoundTestFiles returns the sorted, module-relative slash paths of the
// _test.go files under root that the ratchet binds: the git-tracked ones, or
// every on-disk one when git cannot list them, in which case note says why.
// Directories named vendor or testdata, or starting with "." or "_", are
// never bound; an empty binding set is an error.
func BoundTestFiles(root string) (files []string, note string, err error) {
	tracked, gitErr := repostate.TrackedTree(root, ".")
	if gitErr == nil {
		for _, rel := range tracked {
			rel = filepath.ToSlash(rel)
			if isBound(rel) {
				files = append(files, rel)
			}
		}
	} else {
		note = fmt.Sprintf("git cannot list tracked files, binding every on-disk test file: %v", gitErr)
		if files, err = onDiskTestFiles(root); err != nil {
			return nil, note, err
		}
	}
	if len(files) == 0 {
		return nil, note, fmt.Errorf("rawgitratchet: no test files bound under %s", root)
	}
	sort.Strings(files)
	return files, note, nil
}

func isBound(rel string) bool {
	if !strings.HasSuffix(rel, "_test.go") {
		return false
	}
	for _, seg := range strings.Split(path.Dir(rel), "/") {
		if skipDir(seg) {
			return false
		}
	}
	return true
}

func skipDir(name string) bool {
	return name == "vendor" || name == "testdata" || (name != "." && strings.HasPrefix(name, ".")) || strings.HasPrefix(name, "_")
}

func onDiskTestFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		if rel = filepath.ToSlash(rel); isBound(rel) {
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("rawgitratchet: walk %s: %w", root, err)
	}
	return files, nil
}

// Sites counts the raw git init sites in files (module-relative slash paths
// under root) outside OwnerDir, keyed by file; files without a site are
// absent. A site is an "init" string literal passed to a call or listed in a
// composite literal, in a file whose directory holds a bound test file with a
// "git" string literal — so a helper in a sibling file that spawns git counts
// as much as an inline exec.Command. A listed file missing from disk is
// skipped; one that does not parse is an error.
func Sites(root string, files []string) (map[string]int, error) {
	inits := map[string]int{}
	gitDirs := map[string]bool{}
	for _, rel := range files {
		if rel == OwnerDir || strings.HasPrefix(rel, OwnerDir+"/") {
			continue
		}
		hasGit, n, err := scanFile(filepath.Join(root, filepath.FromSlash(rel)))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("rawgitratchet: %s: %w", rel, err)
		}
		dir := path.Dir(rel)
		gitDirs[dir] = gitDirs[dir] || hasGit
		if n > 0 {
			inits[rel] = n
		}
	}
	for rel := range inits {
		if !gitDirs[path.Dir(rel)] {
			delete(inits, rel)
		}
	}
	return inits, nil
}

func scanFile(p string) (hasGit bool, inits int, err error) {
	src, err := os.ReadFile(p)
	if err != nil {
		return false, 0, err
	}
	f, err := parser.ParseFile(token.NewFileSet(), p, src, parser.SkipObjectResolution)
	if err != nil {
		return false, 0, err
	}
	ast.Inspect(f, func(n ast.Node) bool {
		var args []ast.Expr
		switch n := n.(type) {
		case *ast.BasicLit:
			hasGit = hasGit || isString(n, "git")
		case *ast.CallExpr:
			args = n.Args
		case *ast.CompositeLit:
			args = n.Elts
		}
		for _, arg := range args {
			if lit, ok := arg.(*ast.BasicLit); ok && isString(lit, "init") {
				inits++
			}
		}
		return true
	})
	return hasGit, inits, nil
}

func isString(lit *ast.BasicLit, want string) bool {
	if lit.Kind != token.STRING {
		return false
	}
	s, err := strconv.Unquote(lit.Value)
	return err == nil && s == want
}

// LoadBaseline reads the list of existing call sites: a flat JSON object of
// module-relative test file to its allowed init-site count, each at least 1.
func LoadBaseline(p string) (map[string]int, error) {
	body, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("rawgitratchet: read baseline: %w", err)
	}
	var baseline map[string]int
	if err := json.Unmarshal(body, &baseline); err != nil {
		return nil, fmt.Errorf("rawgitratchet: parse baseline %s: %w", p, err)
	}
	if baseline == nil {
		return nil, fmt.Errorf("rawgitratchet: baseline %s is not a JSON object", p)
	}
	for file, n := range baseline {
		if n < 1 {
			return nil, fmt.Errorf("rawgitratchet: baseline %s: %s allows %d sites; remove the entry", p, file, n)
		}
	}
	return baseline, nil
}

// Check returns nil when sites matches baseline exactly, else one error
// naming every file that differs: an unlisted file with a site, a listed file
// with more sites than allowed, or fewer (lower or remove its entry, so a
// migration frees no slot).
func Check(sites, baseline map[string]int) error {
	var problems []string
	for file, n := range sites {
		if _, listed := baseline[file]; !listed {
			problems = append(problems, fmt.Sprintf("%s builds a raw git repo (%d init sites) outside %s: use gittest.Fixture(t) / gittest.Bare(t) instead", file, n, OwnerDir))
		}
	}
	for file, allowed := range baseline {
		switch n := sites[file]; {
		case n > allowed:
			problems = append(problems, fmt.Sprintf("%s grew to %d raw git init sites > %d listed: use gittest.Fixture(t) / gittest.Bare(t) for the new one", file, n, allowed))
		case n == 0:
			problems = append(problems, fmt.Sprintf("%s is listed with %d raw git init sites but has none: remove its baseline.json entry", file, allowed))
		case n < allowed:
			problems = append(problems, fmt.Sprintf("%s shrank to %d raw git init sites < %d listed: lower its baseline.json entry to %d", file, n, allowed, n))
		}
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return errors.New("rawgitratchet: raw git fixture ratchet violated (the list lives in go/internal/rawgitratchet/baseline.json and may only shrink):\n  " +
		strings.Join(problems, "\n  "))
}
