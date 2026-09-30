//go:build acs

package flagreaders

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var flagNameRE = regexp.MustCompile(`^EVOLVE(_[A-Z0-9]+)+$`)

var textFlagRE = regexp.MustCompile(`\bEVOLVE(_[A-Z0-9]+)+\b`)

var skipDirs = map[string]bool{
	"vendor": true, "testdata": true, ".git": true, "node_modules": true, ".evolve": true,
	"ipcenv": true, "dist": true,
}

var textExts = map[string]bool{
	".md": true, ".markdown": true, ".yml": true, ".yaml": true,
	".json": true, ".txt": true, ".toml": true,
}

var shellExts = map[string]bool{".sh": true}

var textSurfaceRoots = []string{"skills", "agents", ".github"}

var rootProseExclusions = map[string]bool{
	"CHANGELOG.md": true, "SECURITY.md": true, "CODE_OF_CONDUCT.md": true,
	"PRIVACY.md": true, "CONTRIBUTING.md": true,
}

var retiredFlagsByFile = map[string]map[string]bool{
	"agents/evolve-tester.md":             {"EVOLVE_WORKTREE_PATH": true},
	"skills/adversarial-testing/SKILL.md": {"EVOLVE_WORKTREE_BASE": true},
}

func TestEveryProductionReaderHasRegistryRow(t *testing.T) {
	repoRoot := acsassert.RepoRoot(t)
	hasRow := func(name string) bool { _, ok := flagregistry.Lookup(name); return ok }
	orphans := map[string][]string{}

	skipped, err := collectGoOrphans(filepath.Join(repoRoot, "go"), hasRow, orphans)
	if err != nil {
		t.Fatalf("scan go/: %v", err)
	}
	if len(skipped) > 0 {
		t.Logf("flagreaders: %d unparseable file(s) skipped: %s", len(skipped), strings.Join(skipped, ", "))
	}

	for _, sub := range textSurfaceRoots {
		if err := scanTextTree(filepath.Join(repoRoot, sub), textExts, hasRow, orphans); err != nil {
			t.Fatalf("scan %s/: %v", sub, err)
		}
	}
	if err := scanTextTree(repoRoot, shellExts, hasRow, orphans); err != nil {
		t.Fatalf("scan *.sh: %v", err)
	}
	if err := scanRootMarkdown(repoRoot, hasRow, orphans); err != nil {
		t.Fatalf("scan root *.md: %v", err)
	}
	if err := scanTextFile(filepath.Join(repoRoot, "go", "Makefile"), hasRow, orphans); err != nil {
		t.Fatalf("scan go/Makefile: %v", err)
	}

	for name, locs := range orphans {
		t.Errorf("orphan flag %q is referenced in a production surface but has no flagregistry row — "+
			"add it to go/internal/flagregistry/registry_table.go (sorted) or remove the reference.\n  referenced at: %s",
			name, strings.Join(locs, ", "))
	}
}

func collectGoOrphans(goDir string, hasRow func(string) bool, orphans map[string][]string) ([]string, error) {
	acsDir := filepath.Join(goDir, "acs")
	fset := token.NewFileSet()
	var skipped []string
	err := filepath.Walk(goDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if path == acsDir || skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			skipped = append(skipped, path)
			return nil
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			val, uerr := strconv.Unquote(lit.Value)
			if uerr != nil {
				if len(lit.Value) >= 2 && lit.Value[0] == '`' && lit.Value[len(lit.Value)-1] == '`' {
					val = lit.Value[1 : len(lit.Value)-1]
				} else {
					return true
				}
			}
			if !flagNameRE.MatchString(val) {
				return true
			}
			if !hasRow(val) {
				orphans[val] = append(orphans[val], fset.Position(lit.Pos()).String())
			}
			return true
		})
		return nil
	})
	return skipped, err
}

func scanTextTree(root string, exts map[string]bool, hasRow func(string) bool, orphans map[string][]string) error {
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			if os.IsPermission(err) && info != nil && info.IsDir() {
				fmt.Fprintf(os.Stderr, "[flagreaders] NOTE: %s denied by the sandbox — skipped\n", path)
				return filepath.SkipDir
			}
			return err
		}
		if info.IsDir() {
			if skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !exts[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		effectiveHasRow := hasRow
		slash := filepath.ToSlash(path)
		for relPath, retired := range retiredFlagsByFile {
			if strings.HasSuffix(slash, "/"+relPath) {
				r := retired
				orig := hasRow
				effectiveHasRow = func(name string) bool { return orig(name) || r[name] }
				break
			}
		}
		return scanTextFile(path, effectiveHasRow, orphans)
	})
	if err != nil {
		return fmt.Errorf("walk %s: %w", root, err)
	}
	return nil
}

func scanTextFile(path string, hasRow func(string) bool, orphans map[string][]string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read %s: %w", path, err)
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return nil
	}
	for i, line := range strings.Split(string(data), "\n") {
		for _, tok := range textFlagRE.FindAllString(line, -1) {
			if !flagNameRE.MatchString(tok) || hasRow(tok) {
				continue
			}
			orphans[tok] = append(orphans[tok], fmt.Sprintf("%s:%d", path, i+1))
		}
	}
	return nil
}

func scanRootMarkdown(repoRoot string, hasRow func(string) bool, orphans map[string][]string) error {
	entries, err := os.ReadDir(repoRoot)
	if err != nil {
		return fmt.Errorf("readdir %s: %w", repoRoot, err)
	}
	for _, e := range entries {
		if e.IsDir() || strings.ToLower(filepath.Ext(e.Name())) != ".md" || rootProseExclusions[e.Name()] {
			continue
		}
		if err := scanTextFile(filepath.Join(repoRoot, e.Name()), hasRow, orphans); err != nil {
			return err
		}
	}
	return nil
}
