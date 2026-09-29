package repocontract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func treeDeclaring(t *testing.T, goMod string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go", "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestPackRuns_OnlyInEvolveLoopsOwnModuleWhileTheGateIsOn(t *testing.T) {
	own := treeDeclaring(t, "module github.com/mickeyyaya/evolve-loop/go\n\ngo 1.23\n")
	foreign := treeDeclaring(t, "module e2e.local/fixture\n\ngo 1.23\n")
	bare := t.TempDir()
	for _, tc := range []struct {
		name, gate, root, note string
		runs                   bool
	}{
		{"enforce in evolve-loop's module", "enforce", own, "", true},
		{"off", "off", own, "", false},
		{"unset is off, as ship has always read it", "", own, "", false},
		{"a typo runs as enforce and says so", "shadwo", own, `unknown stage "shadwo" — treating as enforce`, true},
		{"a foreign module", "enforce", foreign, "does not declare github.com/mickeyyaya/evolve-loop/go", false},
		{"a tree without go/", "enforce", bare, "does not declare github.com/mickeyyaya/evolve-loop/go", false},
		{"off in a foreign module stays silent", "off", foreign, "", false},
	} {
		runs, note := PackRuns(tc.gate, tc.root)
		if runs != tc.runs || (tc.note == "") != (note == "") || !strings.Contains(note, tc.note) {
			t.Errorf("%s: PackRuns(%q) = (%v, %q), want (%v, containing %q)", tc.name, tc.gate, runs, note, tc.runs, tc.note)
		}
	}
}

func TestPackRuns_ReadsTheModuleLineAsGoWritesIt(t *testing.T) {
	for name, goMod := range map[string]string{
		"space":                       "module github.com/mickeyyaya/evolve-loop/go\n",
		"tab":                         "module\tgithub.com/mickeyyaya/evolve-loop/go\n",
		"runs of blanks":              "module \t  github.com/mickeyyaya/evolve-loop/go  \n",
		"quoted, with a comment":      "// a header comment\nmodule \"github.com/mickeyyaya/evolve-loop/go\" // trailing\n",
		"raw string":                  "module `github.com/mickeyyaya/evolve-loop/go`\n",
		"indented after a blank line": "\n  module github.com/mickeyyaya/evolve-loop/go\n",
	} {
		if runs, note := PackRuns("enforce", treeDeclaring(t, goMod+"\ngo 1.23\n")); !runs || note != "" {
			t.Errorf("%s: %q is evolve-loop's module line; got (%v, %q)", name, goMod, runs, note)
		}
	}
}

func TestPackRuns_RunsInThisRepositorysOwnTree(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if runs, note := PackRuns("enforce", root); !runs || note != "" {
		t.Fatalf("the real tree %s must run the pack, or the fixed pack goes quiet in the live loop; got (%v, %q)", root, runs, note)
	}
}

func TestGateOn_OnlyUnsetAndOffAreOff(t *testing.T) {
	for gate, want := range map[string]bool{"": false, "off": false, "enforce": true, "shadow": true, "shadwo": true} {
		if got := GateOn(gate); got != want {
			t.Errorf("GateOn(%q) = %v, want %v", gate, got, want)
		}
	}
}

func TestModuleDir_IsTheTreesGoDir(t *testing.T) {
	if got := ModuleDir("/lane"); got != filepath.Join("/lane", "go") {
		t.Fatalf("ModuleDir(/lane) = %q", got)
	}
}
