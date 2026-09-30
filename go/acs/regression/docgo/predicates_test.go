//go:build acs

package docgo

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const minDocWords = commentaudit.MinPackageDocWords

func TestDocGo_EveryInternalPackageDocumented(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	internalDir := filepath.Join(goDir, "internal")

	allow := readAllowMissing(t, goDir)

	documented := map[string]bool{}
	undocumented := map[string]bool{}
	for dir := range packageDirs(t, internalDir) {
		rel, err := filepath.Rel(goDir, dir)
		if err != nil {
			t.Fatalf("rel %s: %v", dir, err)
		}
		key := "./" + filepath.ToSlash(rel)
		if packageDocumented(dir) {
			documented[key] = true
		} else {
			undocumented[key] = true
		}
	}

	var regress []string
	for p := range undocumented {
		if !allow[p] {
			regress = append(regress, p)
		}
	}
	if len(regress) > 0 {
		sort.Strings(regress)
		t.Errorf("docgo regression: %d internal package(s) lack a substantive package doc comment (>= %d words) and are not in .docgo-allow-missing — add a doc.go stating what/how/why:\n  %s",
			len(regress), minDocWords, strings.Join(regress, "\n  "))
	}

	var stale []string
	for p := range allow {
		switch {
		case documented[p]:
			stale = append(stale, p+"  (now documented — remove from .docgo-allow-missing)")
		case !undocumented[p]:
			stale = append(stale, p+"  (not a real internal package)")
		}
	}
	if len(stale) > 0 {
		sort.Strings(stale)
		t.Errorf("docgo: %d stale .docgo-allow-missing entr(ies):\n  %s", len(stale), strings.Join(stale, "\n  "))
	}
}

func packageDirs(t *testing.T, internalDir string) map[string]bool {
	t.Helper()
	dirs := map[string]bool{}
	err := filepath.WalkDir(internalDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		dirs[filepath.Dir(path)] = true
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", internalDir, err)
	}
	return dirs
}

func packageDocumented(dir string) bool {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return false
	}
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			if f.Doc != nil && len(strings.Fields(f.Doc.Text())) >= minDocWords {
				return true
			}
		}
	}
	return false
}

func readAllowMissing(t *testing.T, goDir string) map[string]bool {
	t.Helper()
	allow := map[string]bool{}
	data, err := os.ReadFile(filepath.Join(goDir, ".docgo-allow-missing"))
	if err != nil {
		if os.IsNotExist(err) {
			return allow
		}
		t.Fatalf("read .docgo-allow-missing: %v", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		allow[line] = true
	}
	return allow
}
