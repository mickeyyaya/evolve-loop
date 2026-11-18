package cmdutil

import (
	"slices"
	"testing"
)

func TestTakeFlagValue(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		wantValue string
		wantRest  []string
		wantFound bool
		wantOK    bool
	}{
		{"absent", []string{"a", "b"}, "", []string{"a", "b"}, false, true},
		{"present", []string{"a", "--x", "v", "b"}, "v", []string{"a", "b"}, true, true},
		{"valueless", []string{"a", "--x"}, "", nil, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			value, rest, found, ok := TakeFlagValue(c.args, "--x")
			if value != c.wantValue || !slices.Equal(rest, c.wantRest) || found != c.wantFound || ok != c.wantOK {
				t.Fatalf("TakeFlagValue = %q %v %v %v, want %q %v %v %v", value, rest, found, ok, c.wantValue, c.wantRest, c.wantFound, c.wantOK)
			}
		})
	}
}

func TestTakeProjectRoot(t *testing.T) {
	dir := t.TempDir()
	root, rest, ok := TakeProjectRoot([]string{"list", "--project-root", dir, "--json"})
	if root != dir || !slices.Equal(rest, []string{"list", "--json"}) || !ok {
		t.Fatalf("TakeProjectRoot = %q %v %v", root, rest, ok)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", dir)
	root, rest, ok = TakeProjectRoot([]string{"list"})
	if root != dir || !slices.Equal(rest, []string{"list"}) || !ok {
		t.Fatalf("TakeProjectRoot without the flag = %q %v %v", root, rest, ok)
	}
	if _, _, ok := TakeProjectRoot([]string{"--project-root"}); ok {
		t.Fatal("a --project-root with no value must not be ok")
	}
}
