package repocontract_test

import (
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
	"testing"
)

const treestateDir = "go/internal/treestate"

var outputShapeFlags = map[string]bool{
	"--name-only": true, "--name-status": true, "--stat": true, "--shortstat": true,
	"--numstat": true, "--quiet": true, "--check": true, "--raw": true, "--exit-code": true,
}

type diffHomeAllowance struct {
	Reason string
}

var diffHomeAllowlist = map[string]diffHomeAllowance{
	"go/internal/core/phase_bindings.go":            {Reason: "digest home: moves with pair (a), the ledger writer and the ship checks"},
	"go/internal/subagent/subagent.go":              {Reason: "digest home: moves with pair (a)"},
	"go/internal/cyclesimulator/cyclesimulator.go":  {Reason: "digest home: moves with pair (a)"},
	"go/internal/phases/ship/shiptest/fixture.go":   {Reason: "digest home: moves with pair (a)"},
	"go/internal/integration/train.go":              {Reason: "digest home: moves with pair (b), the commit gate attestation and the train"},
	"go/cmd/evolve/cmd_composition_wiring.go":       {Reason: "composition evidence: frozen until the composition records carry a scheme field (change-digest-treedelta-scheme-join)"},
	"go/internal/core/composition_carryforward.go":  {Reason: "composition evidence: frozen until the composition records carry a scheme field (change-digest-treedelta-scheme-join)"},
	"go/internal/gc/salvage.go":                     {Reason: "patch for git apply: the output is applied, not hashed, and keeps --binary"},
	"go/cmd/evolve/cmd_worktree_dev_landed.go":      {Reason: "dev tree text compare: --binary, no stored digest"},
	"go/internal/treedelta/treedelta.go":            {Reason: "dev tree text compare: --binary, frozen scheme with its own test"},
	"go/cmd/evolve/cmd_docs_added.go":               {Reason: "line parser: reads -U0 hunks"},
	"go/internal/gateclass/gateclass.go":            {Reason: "line parser: reads -U0 hunks"},
	"go/internal/core/ste_floor.go":                 {Reason: "line parser: reads -U0 hunks"},
	"go/internal/covchanged/workspace.go":           {Reason: "line parser: reads -U0 hunks"},
	"go/internal/phases/ship/verify.go":             {Reason: "display or scan: no binding holds the hash"},
	"go/cmd/evolve/cmd_scan.go":                     {Reason: "display or scan: no binding holds the hash"},
	"go/internal/integration/compose.go":            {Reason: "display or scan: no binding holds the hash"},
	"go/internal/auditfixports/worktree.go":         {Reason: "display or scan: no binding holds the hash"},
	"go/internal/phases/audit/facts_seed.go":        {Reason: "line parser: the -U0 diff feeds covchanged.ParseDiff and is never hashed"},
	"go/internal/phases/ship/held_proposal_gate.go": {Reason: "audit file: the raw diff bytes are written for the auditor and hashed as that file, which a digest-only call cannot supply"},
}

type diffHomeScan struct {
	DiffSites   map[string]int
	ArgsCallers map[string]int
}

func stringLit(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok {
		return "", false
	}
	text, err := strconv.Unquote(lit.Value)
	return text, err == nil && lit.Kind == token.STRING
}

func isDiffArgsCall(n *ast.CallExpr) bool {
	sel, ok := n.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "DiffArgs" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "treestate"
}

func countDiffSites(args []ast.Expr) int {
	diffs := 0
	for _, arg := range args {
		s, ok := stringLit(arg)
		if !ok {
			continue
		}
		if outputShapeFlags[s] {
			return 0
		}
		if s == "diff" {
			diffs++
		}
	}
	return diffs
}

func scanDiffFile(p string) (sites, callers int, err error) {
	src, err := os.ReadFile(p)
	if err != nil {
		return 0, 0, err
	}
	f, err := parser.ParseFile(token.NewFileSet(), p, src, parser.SkipObjectResolution)
	if err != nil {
		return 0, 0, err
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			if isDiffArgsCall(n) {
				callers++
			}
			sites += countDiffSites(n.Args)
		case *ast.CompositeLit:
			sites += countDiffSites(n.Elts)
		}
		return true
	})
	return sites, callers, nil
}

func scanDiffHomes(root string) (diffHomeScan, error) {
	out := diffHomeScan{DiffSites: map[string]int{}, ArgsCallers: map[string]int{}}
	err := filepath.WalkDir(filepath.Join(root, "go"), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if name := d.Name(); name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".") || rel == treestateDir {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		sites, callers, scanErr := scanDiffFile(p)
		if scanErr != nil {
			return fmt.Errorf("%s: %w", rel, scanErr)
		}
		if sites > 0 {
			out.DiffSites[rel] = sites
		}
		if callers > 0 {
			out.ArgsCallers[rel] = callers
		}
		return nil
	})
	return out, err
}

func diffHomeProblems(scan diffHomeScan, allow map[string]diffHomeAllowance) []string {
	var problems []string
	for file := range scan.DiffSites {
		if _, ok := allow[file]; !ok {
			problems = append(problems, file+" builds a git diff argv outside internal/treestate: call treestate.Pinned or treestate.PinnedBetween, or list the file with a reason if its output is not hashed into a binding")
		}
	}
	for file := range scan.ArgsCallers {
		problems = append(problems, file+" calls treestate.DiffArgs outside internal/treestate: it would skip the hunk-header strip and the env scrub, so call treestate.Pinned or treestate.PinnedBetween")
	}
	for file := range allow {
		if scan.DiffSites[file] == 0 {
			problems = append(problems, file+" is in the change-digest allowlist but builds no git diff argv: remove the stale entry")
		}
	}
	sort.Strings(problems)
	return problems
}

func TestChangeDigestHasOneHome(t *testing.T) {
	scan, err := scanDiffHomes(repositoryRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range diffHomeProblems(scan, diffHomeAllowlist) {
		t.Error(p)
	}
}

func plantTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		full := filepath.Join(root, "go", filepath.FromSlash(rel))
		if err := os.MkdirAll(path.Dir(filepath.ToSlash(full)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestChangeDigestHomeScan_CatchesASecondHome(t *testing.T) {
	root := plantTree(t, map[string]string{
		"internal/second/second.go": "package second\n\nfunc digest(run func(args ...string)) {\n\trun(\"git\", \"diff\", \"HEAD\")\n}\n",
	})
	scan, err := scanDiffHomes(root)
	if err != nil {
		t.Fatal(err)
	}
	problems := diffHomeProblems(scan, nil)
	if len(problems) != 1 || !strings.Contains(problems[0], "internal/second/second.go builds a git diff argv") {
		t.Errorf("problems = %v, want one naming the planted second home", problems)
	}
}

func TestChangeDigestHomeScan_CatchesAHashThroughAHelper(t *testing.T) {
	root := plantTree(t, map[string]string{
		"internal/helper/helper.go": "package helper\n\nvar gitBin = \"git\"\n\nfunc argv() []string {\n\treturn []string{\"diff\", \"--no-ext-diff\", \"HEAD\"}\n}\n",
		"internal/helper/hash.go":   "package helper\n\nfunc hash(run func([]string) string) string {\n\treturn run(argv())\n}\n",
	})
	scan, err := scanDiffHomes(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := scan.DiffSites["go/internal/helper/helper.go"]; got != 1 {
		t.Errorf("helper argv sites = %d, want 1: a hash in another function must not hide the argv", got)
	}
}

func TestChangeDigestHomeScan_CatchesACallerOfDiffArgs(t *testing.T) {
	root := plantTree(t, map[string]string{
		"internal/caller/caller.go": "package caller\n\nimport \"x/treestate\"\n\nfunc a() []string {\n\treturn treestate.DiffArgs(\"HEAD\")\n}\n",
	})
	scan, err := scanDiffHomes(root)
	if err != nil {
		t.Fatal(err)
	}
	problems := diffHomeProblems(scan, nil)
	if len(problems) != 1 || !strings.Contains(problems[0], "calls treestate.DiffArgs outside internal/treestate") {
		t.Errorf("problems = %v, want one naming the DiffArgs caller", problems)
	}
}

func TestChangeDigestHomeScan_SkipsOutputShapeCallsTestsAndTreestate(t *testing.T) {
	root := plantTree(t, map[string]string{
		"internal/shape/shape.go":      "package shape\n\nfunc a(run func(...string)) {\n\trun(\"git\", \"diff\", \"--name-only\", \"HEAD\")\n\trun(\"git\", \"diff\", \"--shortstat\")\n}\n",
		"internal/shape/shape_test.go": "package shape\n\nfunc b(run func(...string)) {\n\trun(\"git\", \"diff\", \"HEAD\")\n}\n",
		"internal/treestate/home.go":   "package treestate\n\nfunc c(run func(...string)) {\n\trun(\"git\", \"diff\", \"HEAD\")\n}\n",
	})
	scan, err := scanDiffHomes(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(scan.DiffSites) != 0 || len(scan.ArgsCallers) != 0 {
		t.Errorf("scan = %+v, want no site", scan)
	}
}

func TestChangeDigestAllowlist_FailsOnAStaleEntry(t *testing.T) {
	scan := diffHomeScan{DiffSites: map[string]int{}, ArgsCallers: map[string]int{}}
	problems := diffHomeProblems(scan, map[string]diffHomeAllowance{"go/internal/gone/gone.go": {Reason: "moved"}})
	if len(problems) != 1 || !strings.Contains(problems[0], "remove the stale entry") {
		t.Errorf("problems = %v, want one stale-entry problem", problems)
	}
}

func TestChangeDigestAllowlist_EachEntryNamesAReason(t *testing.T) {
	for file, a := range diffHomeAllowlist {
		if strings.TrimSpace(a.Reason) == "" {
			t.Errorf("%s has no reason", file)
		}
	}
}
