package apicover

import (
	"bytes"
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_JoinsMethodCoverAndEnforcesUncovered(t *testing.T) {
	imp, err := packageImportPath("testdata/sample")
	if err != nil {
		t.Fatal(err)
	}
	syms, err := Enumerate(context.Background(), "testdata/sample")
	if err != nil {
		t.Fatal(err)
	}
	var cover strings.Builder
	for _, s := range syms {
		if s.Kind == KindFunc || s.Kind == KindMethod {
			fmt.Fprintf(&cover, "%s/%s:%d:\t%s\t100.0%%\n", imp, s.File, s.Line, s.Name)
		}
	}
	path := filepath.Join(t.TempDir(), "cover.func")
	if err := os.WriteFile(path, []byte(cover.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	code, err := Run(context.Background(), Config{Dirs: []string{"testdata/sample"}, CoverPath: path, Enforce: true}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 || strings.Contains(buf.String(), "ExportedMethod") || !strings.Contains(buf.String(), "UNCOVERED (no test names it): 1") {
		t.Errorf("code=%d out:\n%s", code, buf.String())
	}
}

func TestRun_PreExistingDebtHeader(t *testing.T) {
	dir, cover := writeDiffScopeFixture(t)
	var buf strings.Builder
	if _, err := Run(context.Background(), Config{Enforce: true, Dirs: []string{dir}, CoverPath: cover,
		ChangedFilesByDir: map[string]map[string]bool{dir: {"other.go": true}}}, &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "PRE-EXISTING DEBT (files untouched by this change — WARN only, pay down separately):: 2\n") {
		t.Errorf("out:\n%s", buf.String())
	}
}

const enumerateFixture = "package p\n" +
	"\n" +
	"// F is documented.\n" +
	"func F() {}\n" +
	"\n" +
	"type hidden struct{}\n" +
	"\n" +
	"// M sits on an unexported type.\n" +
	"func (hidden) M() {}\n" +
	"\n" +
	"type unexportedType int\n" +
	"\n" +
	"// T is documented.\n" +
	"type T struct{}\n" +
	"\n" +
	"// TM is a documented method.\n" +
	"func (T) TM() {}\n" +
	"\n" +
	"// Group doc.\n" +
	"const (\n" +
	"\tGroupConst = 1\n" +
	")\n" +
	"\n" +
	"// Types doc.\n" +
	"type (\n" +
	"\tGroupedType int\n" +
	")\n" +
	"\n" +
	"// Values doc.\n" +
	"var (\n" +
	"\t//apicover:ignore reason=spec level\n" +
	"\tSpecIgnored = 1\n" +
	")\n" +
	"\n" +
	"func Undocumented() {}\n"

func TestEnumerate_SymbolFieldsAcrossDeclShapes(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "p.go"), []byte(enumerateFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	syms, err := Enumerate(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range syms {
		got = append(got, fmt.Sprintf("%s|%s|%s|%d|%v|%v|%s", s.Pkg, s.Name, s.Kind, s.Line, s.HasDoc, s.Ignored, s.IgnoreReason))
	}
	want := []string{
		"p|F|func|4|true|false|",
		"p|T|type|14|true|false|",
		"p|T.TM|method|17|true|false|",
		"p|GroupConst|const|21|true|false|",
		"p|GroupedType|type|26|true|false|",
		"p|SpecIgnored|var|32|true|true|spec level",
		"p|Undocumented|func|35|false|false|",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	var buf bytes.Buffer
	if _, err := Run(context.Background(), Config{Dirs: []string{root}, RequireDoc: true}, &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "MISSING-DOC (exported, no godoc): 1\n") {
		t.Errorf("out:\n%s", buf.String())
	}
}

func TestExportedSymbols_MalformedDirectivesKeepFirstError(t *testing.T) {
	src := "package q\n\n//apicover:ignore\nfunc First() {}\n\n//apicover:ignore\nfunc Second() {}\n"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "q.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	syms, err := exportedSymbols(fset, f, "q.go")
	if err == nil || !strings.HasPrefix(err.Error(), "q.go:4 First: //apicover:ignore requires") {
		t.Errorf("err=%v", err)
	}
	if len(syms) != 2 || !syms[0].Ignored || !syms[1].Ignored {
		t.Errorf("syms=%+v", syms)
	}
}
