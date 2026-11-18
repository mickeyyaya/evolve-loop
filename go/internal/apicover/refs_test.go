package apicover

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNamesReferencedInTests_CollectsIdentsAndSelectors(t *testing.T) {
	named, err := NamesReferencedInTests(context.Background(), "testdata/sample")
	if err != nil {
		t.Fatalf("NamesReferencedInTests: %v", err)
	}
	for _, want := range []string{"ExportedFunc", "ExportedVar", "ExportedType", "ExportedMethod"} {
		if !named[want] {
			t.Errorf("expected %q to be referenced by the _test fixture; got %v", want, named)
		}
	}
	if named["NeverReferencedAnywhere"] {
		t.Error("did not expect an unreferenced name to appear")
	}
}

func namesFor(t *testing.T, src string) map[string]bool {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x_test.go"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	named, err := NamesReferencedInTests(context.Background(), dir)
	if err != nil {
		t.Fatalf("NamesReferencedInTests: %v", err)
	}
	return named
}

func TestNamesReferencedInTests_BlankBindings(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want map[string]bool
	}{
		{"blank var bare", "package p\nvar _ = A\n", map[string]bool{"A": false}},
		{"blank var multi", "package p\nvar _, _ = A, pkg.B\n", map[string]bool{"A": false, "B": false}},
		{"blank assign paren", "package p\nfunc f() { _ = (A); _, _ = B, C }\n", map[string]bool{"A": false, "B": false, "C": false}},
		{"blank var call names", "package p\nvar _ = D()\n", map[string]bool{"D": true}},
		{"blank assign mixed", "package p\nfunc f() { _, _ = A, E() }\n", map[string]bool{"A": false, "E": true}},
		{"blank typed assertion names type", "package p\nvar _ T = v\n", map[string]bool{"T": true, "v": false}},
		{"blank var no value", "package p\nvar _ int\n", map[string]bool{"int": true}},
		{"named var", "package p\nvar v = A\n", map[string]bool{"A": true}},
		{"partial blank", "package p\nfunc f() { var x int; x, _ = A, B }\n", map[string]bool{"A": true, "B": true}},
		{"define", "package p\nfunc f() { _ := A }\n", map[string]bool{"A": true}},
		{"non ident lhs", "package p\nfunc f() { s[0] = A }\n", map[string]bool{"A": true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := namesFor(t, c.src)
			for name, want := range c.want {
				if got[name] != want {
					t.Errorf("%q: got %v want %v (names %v)", name, got[name], want, got)
				}
			}
		})
	}
}

func TestNamesReferencedInTests_Errors(t *testing.T) {
	if _, err := NamesReferencedInTests(context.Background(), filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("expected error for missing dir")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad_test.go"), []byte("not go"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NamesReferencedInTests(context.Background(), dir); err == nil {
		t.Error("expected parse error")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NamesReferencedInTests(ctx, dir); err == nil {
		t.Error("expected context error")
	}
}
