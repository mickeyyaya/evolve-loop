package apicover

import (
	"strings"
	"testing"
)

func FuzzModuleLine(f *testing.F) {
	for _, seed := range []string{"module example.com/m\n", "go 1.23\n", "", "module\n", "  module  spaced  \n\nmodule second\n", "modulex y\n", "module \"q\"\n"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, gomod string) {
		got := moduleLine([]byte(gomod))
		if got != strings.TrimSpace(got) {
			t.Fatalf("moduleLine(%q) = %q carries surrounding space", gomod, got)
		}
		if got != "" && !strings.Contains(gomod, got) {
			t.Fatalf("moduleLine(%q) = %q is not a substring of the input", gomod, got)
		}
	})
}
