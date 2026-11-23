package docstests

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func seedGoRepo(t *testing.T, module string, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	goDir := filepath.Join(root, "go")
	if err := os.MkdirAll(goDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if module != "" {
		if err := os.WriteFile(filepath.Join(goDir, "go.mod"), []byte("module "+module+"\n\ngo 1.23\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for rel, body := range files {
		full := filepath.Join(goDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestSelect_MatchesLiteralsConstsHelpersAndSkipsTaggedAndProd(t *testing.T) {
	t.Parallel()
	root := seedGoRepo(t, "example.test/fx", map[string]string{
		"internal/x/consts_test.go": `
package x

type docBox struct{ X string }

const (
	constDoc   = "docs/" + "const.md"
	aliasDoc   = constDoc
	spaced     = "docs/with space.md"
	parenDoc   = ("docs/paren.md")
	numConst   = 1
	cmpConst   = 1 == 2
	badAdd     = "a" + 1
	loopA      = loopB
	loopB      = loopA
	refUnknown = missingIdent
)

var callVar = wrapDoc("x")

func wrapDoc(s string) docBox { return docBox{X: s} }
func helperReads(t interface{}) { _ = "docs/helper.md" }
func cycleA() { cycleB() }
func cycleB() { cycleA() }
`,
		"internal/x/x_test.go": `package x

import "testing"

func Test(t *testing.T) { _ = "docs/lit.md" }
func TestLiteral(t *testing.T) { _ = "docs/lit.md" }
func TestConst(t *testing.T) { _ = aliasDoc }
func TestSpaced(t *testing.T) { _ = spaced }
func TestParen(t *testing.T) { _ = parenDoc }
func TestHelper(t *testing.T) { helperReads(t) }
func TestSelector(t *testing.T) { _ = wrapDoc("docs/sel.md").X; _ = wrapDoc("docs/other.md").X }
func TestCycle(t *testing.T) { cycleA() }
func TestNeighbour(t *testing.T) { _ = "docs/lit.md.bak" }
func TesthelperLower(t *testing.T) { _ = "docs/lit.md" }
`,
		"internal/x/prod.go": "package x\n\nconst ProdOnly = \"docs/prod.md\"\n",
		"internal/y/y_test.go": `//go:build acs

package y

import "testing"

func TestTagged(t *testing.T) { _ = "docs/tagged.md" }
`,
		"internal/z/z_test.go": `// +build integration

package z

import "testing"

func TestPlusTagged(t *testing.T) { _ = "docs/plus.md" }
`,
	})

	got, err := Select(root, []string{"docs/lit.md", "./docs/const.md", "docs/with space.md", "docs/helper.md", "docs/paren.md", "docs/sel.md"})
	if err != nil {
		t.Fatal(err)
	}
	want := []Selection{{
		Package: "./internal/x",
		Tests:   []string{"Test", "TestConst", "TestHelper", "TestLiteral", "TestParen", "TestSelector", "TestSpaced"},
	}}
	if !slices.EqualFunc(got, want, func(a, b Selection) bool {
		return a.Package == b.Package && slices.Equal(a.Tests, b.Tests)
	}) {
		t.Fatalf("Select = %+v, want %+v", got, want)
	}
	for _, unread := range []string{"docs/prod.md", "docs/tagged.md", "docs/plus.md", "docs/missing.md"} {
		none, err := Select(root, []string{unread})
		if err != nil || len(none) != 0 {
			t.Errorf("Select(%q) = %+v, %v; want empty", unread, none, err)
		}
	}
}

func TestSelect_HandlesRootTestsAndErrors(t *testing.T) {
	t.Parallel()
	root := seedGoRepo(t, evolveLoopModule, map[string]string{
		"root_test.go": `package root

import "testing"

func TestRootDoc(t *testing.T) { _ = "README.md" }
`,
		"testdata/ignored_test.go": `package td

import "testing"

func TestIgnored(t *testing.T) { _ = "README.md" }
`,
	})
	got, err := Select(root, []string{"README.md"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Package != "." || !slices.Equal(got[0].Tests, []string{"TestRootDoc"}) {
		t.Fatalf("Select(README.md) = %+v", got)
	}

	gotRegistry, err := Select(root, []string{"docs/operations/runtime-reference.md"})
	if err != nil {
		t.Fatal(err)
	}
	if Format(gotRegistry) == "" {
		t.Fatalf("expected repo-contract rows for runtime-reference.md in evolve-loop module")
	}

	noGoMod := seedGoRepo(t, "", map[string]string{"x_test.go": "package x\n"})
	if got, err := Select(noGoMod, []string{"docs/operations/runtime-reference.md"}); err != nil || len(got) != 0 {
		t.Fatalf("missing go.mod: %+v, %v", got, err)
	}
	emptyGoMod := seedGoRepo(t, "", map[string]string{"go.mod": "go 1.23\n", "x_test.go": "package x\n"})
	if got, err := Select(emptyGoMod, []string{"docs/operations/runtime-reference.md"}); err != nil || len(got) != 0 {
		t.Fatalf("go.mod without module: %+v, %v", got, err)
	}

	if _, err := Select(t.TempDir(), []string{"README.md"}); err == nil {
		t.Fatal("missing go directory must error")
	}
	badRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(badRoot, "go"), []byte("not-a-dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Select(badRoot, []string{"README.md"}); err == nil {
		t.Fatal("non-directory go path must error")
	}

	badSyntax := seedGoRepo(t, "example.test/fx", map[string]string{"bad_test.go": "// comment only\n"})
	if _, err := Select(badSyntax, []string{"README.md"}); err == nil {
		t.Fatal("unparseable test file must error")
	}

	unreadableFile := seedGoRepo(t, "example.test/fx", map[string]string{"locked_test.go": "package x\n"})
	lockedPath := filepath.Join(unreadableFile, "go", "locked_test.go")
	if err := os.Chmod(lockedPath, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(lockedPath, 0o644) })
	if _, err := Select(unreadableFile, []string{"README.md"}); err == nil {
		t.Fatal("unreadable test file must error")
	}

	unreadableDir := seedGoRepo(t, "example.test/fx", map[string]string{"sub/a_test.go": "package sub\n"})
	lockedDir := filepath.Join(unreadableDir, "go", "sub")
	if err := os.Chmod(lockedDir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(lockedDir, 0o755) })
	if _, err := Select(unreadableDir, []string{"README.md"}); err == nil {
		t.Fatal("unreadable directory must error")
	}
}

func TestFormatAndRegexp_EmptyCases(t *testing.T) {
	t.Parallel()
	if got := (Selection{Package: "./x"}).Regexp(); got != "" {
		t.Errorf("empty Regexp = %q", got)
	}
	if got := (Selection{Package: "./x", Tests: []string{"TestOne"}}).Regexp(); got != "^TestOne$" {
		t.Errorf("single Regexp = %q", got)
	}
	if got := Format([]Selection{{Package: "./x"}}); got != "" {
		t.Errorf("empty tests Format = %q", got)
	}
}
