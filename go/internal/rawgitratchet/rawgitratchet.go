// Package rawgitratchet keeps new raw git fixtures out of the module's
// tests.
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
	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
)

const OwnerDir = "internal/gittest"

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

var onDiskRel = filepath.Rel

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
		rel, err := onDiskRel(root, p)
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

func isCommitMessageFlag(e ast.Expr) bool {
	lit, ok := e.(*ast.BasicLit)
	return ok && isString(lit, "-m")
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
		for i, arg := range args {
			lit, ok := arg.(*ast.BasicLit)
			if !ok || !isString(lit, "init") {
				continue
			}
			if i > 0 && isCommitMessageFlag(args[i-1]) {
				continue
			}
			inits++
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

func Check(sites, baseline map[string]int) error {
	var problems []string
	for file, n := range sites {
		if _, listed := baseline[file]; !listed {
			problems = append(problems, fmt.Sprintf("%s builds a raw git repo (%d init sites) outside %s: use gittest.Fixture(t) / gittest.Bare(t) instead", file, n, OwnerDir))
		}
	}
	for file, allowed := range baseline {
		if n := sites[file]; n > allowed {
			problems = append(problems, fmt.Sprintf("%s grew to %d raw git init sites > %d listed: use gittest.Fixture(t) / gittest.Bare(t) for the new one", file, n, allowed))
		}
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return errors.New("rawgitratchet: raw git fixture ratchet violated (the list lives in go/internal/rawgitratchet/baseline.json and may only shrink):\n  " +
		strings.Join(problems, "\n  "))
}

const BaselineRelPath = "internal/rawgitratchet/baseline.json"

func Scan(root string) (note string, err error) {
	sites, baseline, note, err := load(root)
	if err != nil {
		return note, err
	}
	return note, Check(sites, baseline)
}

func TightenModule(root string) (changes []string, encoded []byte, err error) {
	sites, baseline, _, err := load(root)
	if err != nil {
		return nil, nil, err
	}
	return sizeratchet.TightenEncoded(sites, baseline, 0)
}

func load(root string) (sites, baseline map[string]int, note string, err error) {
	files, note, err := BoundTestFiles(root)
	if err != nil {
		return nil, nil, note, err
	}
	if sites, err = Sites(root, files); err != nil {
		return nil, nil, note, err
	}
	baseline, err = LoadBaseline(filepath.Join(root, filepath.FromSlash(BaselineRelPath)))
	return sites, baseline, note, err
}
