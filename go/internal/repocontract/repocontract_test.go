package repocontract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	root := treeDeclaring(t, "// a header comment\nmodule \"github.com/mickeyyaya/evolve-loop/go\" // quoted, with a trailing comment\n\ngo 1.23\n")
	if runs, note := PackRuns("enforce", root); !runs || note != "" {
		t.Fatalf("a quoted module path with a trailing comment is still evolve-loop's module; got (%v, %q)", runs, note)
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
