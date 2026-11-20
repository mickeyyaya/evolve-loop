package cmdutil

import (
	"slices"
	"testing"
)

func TestFlags_Parse(t *testing.T) {
	t.Parallel()
	var asJSON bool
	var root string
	var items []string
	operands, err := Flags().
		Bools(map[string]*bool{"--json": &asJSON}).
		Values(map[string]*string{"--project-root": &root}).
		Lists(map[string]*[]string{"--item": &items}).
		Parse([]string{"op1", "--json", "--project-root=/r", "--item", "a", "--item=b", "op2"})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if !asJSON || root != "/r" || !slices.Equal(items, []string{"a", "b"}) || !slices.Equal(operands, []string{"op1", "op2"}) {
		t.Fatalf("got json=%v root=%q items=%v operands=%v", asJSON, root, items, operands)
	}
	for _, bad := range [][]string{{"--project-root"}, {"--bogus"}} {
		if _, err := Flags().Values(map[string]*string{"--project-root": &root}).Parse(bad); err == nil {
			t.Errorf("Parse(%q) expected error", bad)
		}
	}
}
