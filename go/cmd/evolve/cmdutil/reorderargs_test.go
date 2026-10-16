package cmdutil

import (
	"reflect"
	"testing"
)

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
