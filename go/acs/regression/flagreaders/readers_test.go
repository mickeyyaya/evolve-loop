//go:build acs

// Package flagreaders is a durable ACS regression guard for the flag-reduction
// campaign: every well-formed EVOLVE_* string literal used in production Go
// (non-_test.go, outside go/acs) MUST have a flagregistry row.
//
// It catches the silent-orphan class — removing a registry entry while a code
// reader still exists, or adding a reader without documenting the flag — which
// the registry has no other guard for (the read path does not funnel through
// the registry). It replaces the blunt `len(All) >= 250` count-floor that used
// to (accidentally) block intentional reduction.
//
// Discrimination: it inspects STRING-LITERAL AST nodes only (not comments, not
// substrings), and the anchored name regex rejects mid-sentence mentions
// ("EVOLVE_FOO is deprecated" — the literal is the whole sentence) and dynamic
// prefixes ("EVOLVE_E2E_MODEL_" ends in '_'). So a standalone read/const/
// map-key "EVOLVE_FOO" is caught; a WARN string or a built-up key is not.
package flagreaders

import (
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

// flagNameRE matches a complete, well-formed flag name. Anchored, so it never
// matches a flag mentioned inside a longer string, and the (_[A-Z0-9]+)+ tail
// rejects dynamic prefixes that end in '_'.
var flagNameRE = regexp.MustCompile(`^EVOLVE(_[A-Z0-9]+)+$`)

// skipDirs are subtrees whose EVOLVE_* literals are not production reads:
// vendor/testdata are fixtures, matched by basename (convention guarantees no
// production readers). The ACS predicate tree (go/acs, which references flag
// names in assertions) is skipped by PATH in the walk so a future production
// dir merely named "acs" is not pruned. (_test.go files are skipped per-file.)
var skipDirs = map[string]bool{"vendor": true, "testdata": true}

// TestEveryProductionReaderHasRegistryRow walks go/ and fails if any standalone
// EVOLVE_* string literal in non-test production code lacks a flagregistry row.
func TestEveryProductionReaderHasRegistryRow(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	acsDir := filepath.Join(goDir, "acs")
	fset := token.NewFileSet()
	orphans := map[string][]string{} // flag name -> file:line locations
	var skipped []string             // unparseable files (logged, not failed)

	walkErr := filepath.Walk(goDir, func(path string, info os.FileInfo, err error) error {
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
		// Parse syntactically (mode 0): no comment scanning, build tags ignored
		// so tagged production files are still covered. Unparseable files are
		// recorded and logged (the compiler catches syntax errors separately) so
		// the skip is never silent.
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
				// Raw (backtick) string literals don't Unquote — strip delimiters
				// so a `EVOLVE_FOO`-style const/map-key read is still seen.
				if len(lit.Value) >= 2 && lit.Value[0] == '`' && lit.Value[len(lit.Value)-1] == '`' {
					val = lit.Value[1 : len(lit.Value)-1]
				} else {
					return true
				}
			}
			if !flagNameRE.MatchString(val) {
				return true
			}
			if _, found := flagregistry.Lookup(val); !found {
				orphans[val] = append(orphans[val], fset.Position(lit.Pos()).String())
			}
			return true
		})
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk %s: %v", goDir, walkErr)
	}
	if len(skipped) > 0 {
		t.Logf("flagreaders: %d unparseable file(s) skipped: %s", len(skipped), strings.Join(skipped, ", "))
	}

	for name, locs := range orphans {
		t.Errorf("orphan flag %q is read in production Go but has no flagregistry row — "+
			"add it to go/internal/flagregistry/registry_table.go (sorted) or remove the reader.\n  read at: %s",
			name, strings.Join(locs, ", "))
	}
}
