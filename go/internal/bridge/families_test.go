package bridge

import (
	"reflect"
	"strings"
	"testing"
)

func TestInteractiveFamiliesFrom(t *testing.T) {
	names := []string{"claude-tmux", "codex-tmux", "claude-p", "agy-tmux", "ollama-tmux"}
	manifest := func(name string) (Manifest, error) {
		return Manifest{CLI: name, Binary: strings.TrimSuffix(name, "-tmux")}, nil
	}
	installed := map[string]bool{"claude": true, "codex": true, "agy": false, "ollama": true}
	got := interactiveFamiliesFrom(names, manifest, func(bin string) bool { return installed[bin] })
	want := []string{"claude", "codex", "ollama"} // agy uninstalled; claude-p not tmux
	if !reflect.DeepEqual(got, want) {
		t.Errorf("interactiveFamiliesFrom = %v, want %v", got, want)
	}
}

// The exact set is host-dependent (only installed CLIs), so this asserts only
// the stable invariants: sorted and deduped.
func TestInteractiveFamilies_Invariants(t *testing.T) {
	got := InteractiveFamilies()
	seen := map[string]bool{}
	for i, fam := range got {
		if seen[fam] {
			t.Errorf("duplicate family %q in %v", fam, got)
		}
		seen[fam] = true
		if i > 0 && got[i-1] > fam {
			t.Errorf("families not sorted: %v", got)
		}
	}
}
