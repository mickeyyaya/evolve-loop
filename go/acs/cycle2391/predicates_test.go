//go:build acs

package cycle2391

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/covchanged"
	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	shipPkgRel   = "go/internal/phases/ship"
	shipPkgArg   = "./internal/phases/ship"
	frozenTest   = "go/internal/phases/ship/ship_manual_branch_item_test.go"
	namedTest    = "TestShipManual_WritesTheTrailerOfTheBranchItem"
	specDoc      = "docs/architecture/inbox-curator.md"
	packageNote  = "docs/architecture/packages/internal-phases-ship.md"
	goTestBudget = 4 * time.Minute
)

func repoAndModule(t *testing.T) (string, string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	module, err := acsassert.ModuleDir(filepath.Join(root, "go"))
	if err != nil {
		t.Fatal(err)
	}
	return root, module
}

func runIn(t *testing.T, dir string, name string, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), goTestBudget)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func gitIn(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()
	return runIn(t, root, "git", append([]string{"-C", root}, args...)...)
}

func baseRef(t *testing.T, root string) string {
	t.Helper()
	for _, ref := range []string{"main", "origin/main"} {
		if out, err := gitIn(t, root, "merge-base", "HEAD", ref); err == nil {
			return strings.TrimSpace(out)
		}
	}
	t.Fatal("no merge base of HEAD with main or origin/main")
	return ""
}

func baseSource(t *testing.T, root, base, rel string) []byte {
	t.Helper()
	out, err := gitIn(t, root, "show", base+":"+rel)
	if err != nil {
		return nil
	}
	return []byte(out)
}

func goFilesChangedSince(t *testing.T, root, base string) []string {
	t.Helper()
	seen := map[string]bool{}
	for _, args := range [][]string{
		{"diff", "--name-only", "--no-renames", "--diff-filter=AM", base, "--", "go"},
		{"ls-files", "--others", "--exclude-standard", "--", "go"},
	} {
		out, err := gitIn(t, root, args...)
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			if strings.HasSuffix(line, ".go") && !strings.HasPrefix(line, "go/acs/") {
				seen[line] = true
			}
		}
	}
	files := make([]string, 0, len(seen))
	for f := range seen {
		files = append(files, f)
	}
	sort.Strings(files)
	return files
}

func commentTexts(t *testing.T, name string, src []byte) map[string]int {
	t.Helper()
	counts := map[string]int{}
	if len(src) == 0 {
		return counts
	}
	f, err := parser.ParseFile(token.NewFileSet(), name, src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	for _, group := range f.Comments {
		for _, c := range group.List {
			if !strings.HasPrefix(c.Text, "//go:") && !strings.HasPrefix(c.Text, "//line ") {
				counts[c.Text]++
			}
		}
	}
	return counts
}

func funcKeys(t *testing.T, name string, src []byte) map[string]bool {
	t.Helper()
	keys := map[string]bool{}
	if len(src) == 0 {
		return keys
	}
	f, err := parser.ParseFile(token.NewFileSet(), name, src, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok {
			keys[fn.Name.Name] = true
		}
	}
	return keys
}

func consultsClosesInbox(fn *ast.FuncDecl) bool {
	found := false
	ast.Inspect(fn, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "closesinbox" {
				found = true
			}
		}
		return !found
	})
	return found
}

func isProductionGoFile(e os.DirEntry) bool {
	return !e.IsDir() && strings.HasSuffix(e.Name(), ".go") && !strings.HasSuffix(e.Name(), "_test.go")
}

func changedShipFuncs(t *testing.T, root, base string) (all, consulting []string) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(shipPkgRel))
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	baseNames := map[string]bool{}
	for _, e := range entries {
		if !isProductionGoFile(e) {
			continue
		}
		rel := shipPkgRel + "/" + e.Name()
		for k := range funcKeys(t, rel, baseSource(t, root, base, rel)) {
			baseNames[k] = true
		}
	}
	for _, e := range entries {
		if !isProductionGoFile(e) {
			continue
		}
		path := filepath.Join(dir, e.Name())
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Name.Name == "init" {
				continue
			}
			switch {
			case consultsClosesInbox(fn):
				consulting = append(consulting, fn.Name.Name)
				all = append(all, fn.Name.Name)
			case !baseNames[fn.Name.Name]:
				all = append(all, fn.Name.Name)
			}
		}
	}
	return all, consulting
}

func TestC2391_001_TheNamedRedTestPassesThroughShipRun(t *testing.T) {
	root, module := repoAndModule(t)
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(frozenTest))); err != nil {
		t.Fatalf("the frozen test file is missing: %v", err)
	}
	if out, err := gitIn(t, root, "check-ignore", "-q", frozenTest); err == nil {
		t.Errorf("the frozen test file is gitignored and would be dropped at ship: %s", out)
	}
	out, err := runIn(t, module, "go", "test", "-count=1", "-v", "-run", "^"+namedTest+"$", shipPkgArg)
	if err != nil {
		t.Fatalf("%s did not pass: %v\n%s", namedTest, err, acsassert.OutputTail(out, 40))
	}
	if !strings.Contains(out, "--- PASS: "+namedTest) {
		t.Errorf("%s did not run:\n%s", namedTest, acsassert.OutputTail(out, 20))
	}
}

func TestC2391_002_TheDeclaredTrailerAndNoItemCasesStayGreen(t *testing.T) {
	_, module := repoAndModule(t)
	out, err := runIn(t, module, "go", "test", "-count=1", "-v", "-run", "^TestShipManual_", shipPkgArg)
	if err != nil {
		t.Fatalf("a TestShipManual_ test failed: %v\n%s", err, acsassert.OutputTail(out, 60))
	}
	for _, want := range []string{
		"--- PASS: " + namedTest,
		"--- PASS: TestShipManual_KeepsATrailerTheMessageAlreadyDeclares/closes_another_item",
		"--- PASS: TestShipManual_KeepsATrailerTheMessageAlreadyDeclares/refs_another_item",
		"--- PASS: TestShipManual_KeepsATrailerTheMessageAlreadyDeclares/closes_none",
		"--- PASS: TestShipManual_WritesNoTrailerWhenNoOpenItemNamesTheBranch/no_inbox_directory",
		"--- PASS: TestShipManual_WritesNoTrailerWhenNoOpenItemNamesTheBranch/only_another_item",
		"--- PASS: TestShipManual_WritesNoTrailerWhenNoOpenItemNamesTheBranch/an_item_id_that_only_starts_the_branch_name",
		"--- PASS: TestShipManual_WritesNoTrailerWhenNoOpenItemNamesTheBranch/an_item_already_consumed",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in the test output:\n%s", want, acsassert.OutputTail(out, 30))
		}
	}
}

func TestC2391_003_EveryNewOrClosesInboxFunctionIsFullyCovered(t *testing.T) {
	root, module := repoAndModule(t)
	base := baseRef(t, root)
	all, consulting := changedShipFuncs(t, root, base)
	if len(consulting) == 0 {
		t.Fatal("no function of package ship consults closesinbox: the pre-fill guard is absent")
	}
	profile := filepath.Join(t.TempDir(), "cover.out")
	out, err := runIn(t, module, "go", "test", "-count=1", "-run", "^TestShipManual_", "-coverprofile="+profile, shipPkgArg)
	if err != nil {
		t.Fatalf("the coverage run failed: %v\n%s", err, acsassert.OutputTail(out, 40))
	}
	report, err := runIn(t, module, "go", "tool", "cover", "-func="+profile)
	if err != nil {
		t.Fatalf("go tool cover: %v\n%s", err, report)
	}
	for _, name := range all {
		pct, err := acsassert.FuncCoverage(report, name)
		if err != nil {
			t.Errorf("%v", err)
			continue
		}
		if pct < 100 {
			t.Errorf("function %s is %.1f%% covered by the TestShipManual_ tests, want 100%%", name, pct)
		}
	}
}

func TestC2391_004_NoGoFileOfTheChangeGainsAComment(t *testing.T) {
	root, _ := repoAndModule(t)
	base := baseRef(t, root)
	files := goFilesChangedSince(t, root, base)
	if len(files) == 0 {
		t.Fatal("the change touches no Go file under go/")
	}
	sawShip := false
	for _, rel := range files {
		sawShip = sawShip || (strings.HasPrefix(rel, shipPkgRel+"/") && !strings.HasSuffix(rel, "_test.go"))
		now, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		before := commentTexts(t, rel, baseSource(t, root, base, rel))
		for text, n := range commentTexts(t, rel, now) {
			if n > before[text] {
				t.Errorf("%s gains the comment %q", rel, text)
			}
		}
	}
	if !sawShip {
		t.Error("the change touches no production file of package ship")
	}
}

func addedLinesOf(t *testing.T, root, base, rel string) map[int]bool {
	t.Helper()
	out, err := gitIn(t, root, "-c", "core.quotePath=false", "diff", "-U0", "--no-color", "--no-ext-diff", "--no-renames", base, "--", rel)
	if err != nil {
		t.Fatalf("git diff %s: %v\n%s", rel, err, out)
	}
	diff, err := covchanged.ParseDiff([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	added := map[int]bool{}
	for _, n := range diff.Added[rel] {
		added[n] = true
	}
	return added
}

func addedText(t *testing.T, root, rel string, added map[int]bool) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for i, line := range strings.Split(string(raw), "\n") {
		if added[i+1] {
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}

func TestC2391_005_TheSpecAndThePackageNoteStateTheChangeInSTE(t *testing.T) {
	root, _ := repoAndModule(t)
	base := baseRef(t, root)
	words, err := stelint.LoadStandard(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{specDoc, packageNote} {
		added := addedLinesOf(t, root, base, rel)
		if len(added) == 0 {
			t.Errorf("%s has no added line: the change is not stated", rel)
			continue
		}
		text := addedText(t, root, rel, added)
		for _, want := range []string{"Closes-Inbox", "--class manual", "branch"} {
			if !strings.Contains(text, want) {
				t.Errorf("%s: the added lines never mention %q", rel, want)
			}
		}
		abs := filepath.Join(root, filepath.FromSlash(rel))
		findings, err := stelint.LintFile(abs, stelint.Options{Words: words})
		if err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(abs)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range stelint.KeepAdded(raw, false, added, findings) {
			if !f.IsSkipNotice() {
				t.Errorf("%s:%d STE finding %s: %s", rel, f.Line, f.Rule, f.Message)
			}
		}
	}
}
