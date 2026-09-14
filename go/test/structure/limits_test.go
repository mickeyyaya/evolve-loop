package structure

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckLimits_SourceBoundaries(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"function 49 lines", "package fixture\nfunc Example() {\n" + strings.Repeat("// body\n", 47) + "}\n", ""},
		{"function 50 lines", "package fixture\nfunc Example() {\n" + strings.Repeat("// body\n", 48) + "}\n", "fixture.go: Example is 50 lines ≥ 50"},
		{"file 799 newlines", "package fixture\n" + strings.Repeat("\n", 798), ""},
		{"file 800 newlines", "package fixture\n" + strings.Repeat("\n", 799), "fixture.go: 800 lines ≥ 800"},
		{"final line without newline", "package fixture\n" + strings.Repeat("\n", 798) + "// final line", ""},
		{"doc comments excluded from function", "package fixture\n" + strings.Repeat("// docs\n", 55) + "func Example() {}\n", ""},
		{"depth four", "package fixture\nfunc Example() { for { for { for { for {} } } } }", ""},
		{"depth five", "package fixture\nfunc Example() { for { for { for { for { for {} } } } } }", "fixture.go: Example nests 5 deep > 4"},
		{"else depth six", "package fixture\nfunc Example() { if true {} else { for { for { for { for { for {} } } } } } }", "fixture.go: Example nests 6 deep > 4"},
		{"bodyless declaration", "package fixture\nfunc Example()", ""},
		{"nonfunction declaration", "package fixture\ntype Example struct{}", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "fixture.go"), []byte(tc.src), 0o644); err != nil {
				t.Fatal(err)
			}
			err := CheckLimits(dir)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("CheckLimits: %v", err)
				}
			} else if err == nil || err.Error() != tc.want {
				t.Fatalf("CheckLimits = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestCheckLimits_PreservesDirectoryScope(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, name := range []string{"ignored_test.go", "notes.txt", "child/invalid.go"} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("invalid Go"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := CheckLimits(dir); err != nil {
		t.Fatalf("excluded files affected current-directory check: %v", err)
	}
	// The existing scanner checks source regardless of filename build selection.
	if err := os.WriteFile(filepath.Join(dir, "source_windows.go"), []byte("invalid Go"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CheckLimits(dir); err == nil || !strings.Contains(err.Error(), "source_windows.go") {
		t.Fatalf("platform source was not checked: %v", err)
	}
}

func TestCheckLimits_CollectsEveryViolation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := "package fixture\nfunc First() { for { for { for { for { for {} } } } } }\n" +
		"func Second() { for { for { for { for { for {} } } } } }\n"
	for _, name := range []string{"a.go", "b.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want := "a.go: First nests 5 deep > 4\na.go: Second nests 5 deep > 4\n" +
		"b.go: First nests 5 deep > 4\nb.go: Second nests 5 deep > 4"
	if err := CheckLimits(dir); err == nil || err.Error() != want {
		t.Fatalf("CheckLimits = %v, want %q", err, want)
	}
}

func TestCheckLimits_ReadAndParseErrors(t *testing.T) {
	t.Parallel()
	t.Run("missing directory", func(t *testing.T) {
		if err := CheckLimits(filepath.Join(t.TempDir(), "missing")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("CheckLimits = %v, want missing-directory error", err)
		}
	})
	t.Run("unreadable source", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "directory.go"), 0o755); err != nil {
			t.Fatal(err)
		}
		var pathErr *os.PathError
		if err := CheckLimits(dir); !errors.As(err, &pathErr) {
			t.Fatalf("CheckLimits = %v, want source read error", err)
		}
	})
	t.Run("malformed source", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "broken.go"), []byte("invalid Go"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := CheckLimits(dir); err == nil || !strings.Contains(err.Error(), "broken.go:1:1:") {
			t.Fatalf("CheckLimits = %v, want parser error with location", err)
		}
	})
}

func TestNesting_ControlFlowDepth(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		want int
	}{
		{"no control flow", "return", 0},
		{"if body", "if true { if true {} }", 2},
		{"depth four allowed", "for { for { for { for {} } } }", 4},
		{"depth five rejected", "for { for { for { for { for {} } } } }", 5},
		{"empty else", "if true {} else {}", 1},
		{"else body", "if true {} else { for { for { for { for { for {} } } } } }", 6},
		{"else if", "if true {} else if false {} else if true {}", 3},
		{"deepest sibling", "if true { for {} } else { for { for {} } }", 3},
		{"range", "for range []int{} { if true {} }", 2},
		{"switch", "switch { case true: for {} }", 2},
		{"type switch", "switch x.(type) { case int: for {} }", 2},
		{"select", "select { default: for {} }", 2},
		{"anonymous body", "func() { if true {} }()", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", "package fixture\nfunc Example() { "+tc.body+" }", 0)
			if err != nil {
				t.Fatal(err)
			}
			fn := file.Decls[0].(*ast.FuncDecl)
			if got := nesting(fn.Body, 0); got != tc.want {
				t.Errorf("nesting(%q) = %d, want %d", tc.body, got, tc.want)
			}
		})
	}
}
