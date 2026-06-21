package cmdutil

import (
	"reflect"
	"testing"
)

// TestReorderArgs verifies flags are moved ahead of positionals (order within
// each class preserved), so flag.Parse accepts flag-after-positional input.
func TestReorderArgs(t *testing.T) {
	cases := []struct {
		in   []string
		want []string
	}{
		{[]string{"probe", "foo", "--json"}, []string{"--json", "probe", "foo"}},
		{[]string{"--a", "x", "-b", "y"}, []string{"--a", "-b", "x", "y"}},
		{[]string{"only", "positionals"}, []string{"only", "positionals"}},
		{nil, []string{}},
	}
	for _, c := range cases {
		if got := ReorderArgs(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("ReorderArgs(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}
