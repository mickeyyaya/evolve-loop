package bridge

import (
	"reflect"
	"strings"
	"testing"
)

// TestInteractiveFamiliesFrom verifies the family enumeration the usage probe
// targets: only interactive (*-tmux) drivers whose binary is actually INSTALLED,
// mapped to their family (binary) name, deduped, and sorted. Filtering by
// installation is what stops the probe from wasting a boot timeout every cycle
// on a CLI the operator does not have.
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

// TestInteractiveFamilies_Invariants exercises the production enumeration over
// the real registry + host PATH. The exact set is host-dependent (only installed
// CLIs), so we assert only the stable invariants: sorted and deduped.
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
