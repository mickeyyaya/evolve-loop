package docstests

import (
	"testing"
)

func TestApicover_NamesEveryExport(t *testing.T) {
	t.Parallel()
	sel := Selection{
		Package: "./internal/x",
		Tests:   []string{"TestA", "TestB"},
	}
	if got := sel.Regexp(); got != "^(TestA|TestB)$" {
		t.Fatalf("Selection.Regexp() = %q, want ^(TestA|TestB)$", got)
	}
	if got := Format([]Selection{sel}); got != "./internal/x\t^(TestA|TestB)$\n" {
		t.Fatalf("Format() = %q", got)
	}
	got, err := Select(t.TempDir(), nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("Select(nil) = %v, %v", got, err)
	}
}
