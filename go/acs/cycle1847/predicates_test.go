//go:build acs

package cycle1847

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func tokenusageTestFiles(t *testing.T) (*token.FileSet, map[string]*ast.File) {
	t.Helper()
	dir := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "tokenusage")
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool { return strings.HasSuffix(fi.Name(), "_test.go") }, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", dir, err)
	}
	files := map[string]*ast.File{}
	for _, pkg := range pkgs {
		for name, f := range pkg.Files {
			files[filepath.Base(name)] = f
		}
	}
	if len(files) == 0 {
		t.Fatalf("no test files parsed in %s", dir)
	}
	return fset, files
}

func transcriptWithLineOfBytes(t *testing.T, worktree string, lineBytes int) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "projects", "-wt")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"type":"user","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:01Z","message":{"id":"u1","content":"go"}}` + "\n" +
		`{"type":"assistant","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:05Z","message":{"id":"m1","usage":{"input_tokens":100,"output_tokens":20}}}` + "\n" +
		`{"type":"assistant","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:06Z","message":{"id":"big","content":"` + strings.Repeat("x", lineBytes) + `"}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "s.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func scanWindow(worktree string) tokenusage.Window {
	start, _ := time.Parse(time.RFC3339, "2026-07-07T10:00:00Z")
	end, _ := time.Parse(time.RFC3339, "2026-07-07T10:10:00Z")
	return tokenusage.Window{Worktree: worktree, Start: start, End: end}
}

func TestC1847_001_OverlongTranscriptLineIsReportedNotTruncatedSilently(t *testing.T) {
	root := transcriptWithLineOfBytes(t, "/wt", 9*1024*1024)
	res, err := tokenusage.ScanConfigRoot(root, scanWindow("/wt"))
	if err == nil && res.Warn == "" {
		t.Errorf("9 MiB line dropped silently: Usage=%+v Warn=%q", res.Usage, res.Warn)
	}
}

func TestC1847_002_TranscriptWithinBufferStaysQuietAndCounted(t *testing.T) {
	root := transcriptWithLineOfBytes(t, "/wt", 1024)
	res, err := tokenusage.ScanConfigRoot(root, scanWindow("/wt"))
	if err != nil || res.Warn != "" {
		t.Fatalf("normal transcript must not warn: err=%v warn=%q", err, res.Warn)
	}
	if res.Source != tokenusage.SourceTranscript || res.Usage.Input != 100 || res.Usage.Output != 20 {
		t.Errorf("normal transcript miscounted: %+v source=%q", res.Usage, res.Source)
	}
}

func TestC1847_003_NoTokenusageTestComparesAValueWithItself(t *testing.T) {
	fset, files := tokenusageTestFiles(t)
	for name, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			be, ok := n.(*ast.BinaryExpr)
			if !ok || (be.Op != token.EQL && be.Op != token.NEQ) {
				return true
			}
			if exprText(fset, be.X) == exprText(fset, be.Y) {
				t.Errorf("%s:%d compares %s with itself", name, fset.Position(be.Pos()).Line, exprText(fset, be.X))
			}
			return true
		})
	}
}

func exprText(fset *token.FileSet, e ast.Expr) string {
	for p, ok := e.(*ast.ParenExpr); ok; p, ok = e.(*ast.ParenExpr) {
		e = p.X
	}
	pos, end := fset.Position(e.Pos()), fset.Position(e.End())
	src, err := os.ReadFile(pos.Filename)
	if err != nil {
		return ""
	}
	return string(src[pos.Offset:end.Offset])
}

func TestC1847_004_NoHandRolledItoaOrJSONQuoteInTokenusageTests(t *testing.T) {
	_, files := tokenusageTestFiles(t)
	for name, f := range files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && (fd.Name.Name == "itoa" || fd.Name.Name == "jsonQuote") {
				t.Errorf("%s declares hand-rolled %s; use strconv.Itoa / encoding/json", name, fd.Name.Name)
			}
		}
	}
}

func TestC1847_005_MarkerTestWhyStringsCarryNoLineNumbers(t *testing.T) {
	path := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "tokenusage", "scanner_marker_test.go")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if hits := regexp.MustCompile(`\w+\.go:\d+`).FindAllString(string(src), -1); len(hits) > 0 {
		t.Errorf("stale file:line citations remain: %v", hits)
	}
}

func TestC1847_006_FillWarnKeepsItsContractAfterSimplification(t *testing.T) {
	for _, pct := range []float64{tokenusage.FillPctUnmeasured, -0.5, -100, 0, 80} {
		if got := tokenusage.FillWarn("build", pct, 80); got != "" {
			t.Errorf("FillWarn(%v, 80) = %q, want empty", pct, got)
		}
	}
	if got := tokenusage.FillWarn("build", 80.1, 80); !strings.Contains(got, "build") {
		t.Errorf("FillWarn above threshold = %q, want phase-named warning", got)
	}
}
