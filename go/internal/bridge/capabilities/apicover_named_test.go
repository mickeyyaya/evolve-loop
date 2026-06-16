package capabilities

import "testing"

// TestKeyBinding_FieldsNamed names the KeyBinding DTO and pins its two-field
// shape (Keys + Action) — the wire record the capability catalog stores for a
// CLI's interrupt/submit keys. Field renames would fail to compile here.
func TestKeyBinding_FieldsNamed(t *testing.T) {
	kb := KeyBinding{Keys: "ctrl+c", Action: "interrupt"}
	if kb.Keys != "ctrl+c" || kb.Action != "interrupt" {
		t.Errorf("KeyBinding fields not addressable as set: %+v", kb)
	}
}

// TestHeadless_FieldsNamed names the Headless DTO and pins its Entrypoint +
// Available shape (the headless-mode descriptor for a CLI).
func TestHeadless_FieldsNamed(t *testing.T) {
	h := Headless{Entrypoint: "claude -p", Available: []string{"-p", "--print"}}
	if h.Entrypoint != "claude -p" {
		t.Errorf("Entrypoint=%q, want %q", h.Entrypoint, "claude -p")
	}
	if len(h.Available) != 2 || h.Available[0] != "-p" {
		t.Errorf("Available=%v, want [-p --print]", h.Available)
	}
}

// TestDrift_TypeNamedAndCleanContract names the Drift type and pins its Clean()
// contract: a Drift with matching catalog/live counts and no diff lists is
// Clean; one with InCatalogNotLive entries is not. This is the drift-detection
// invariant the capability check reports on.
func TestDrift_TypeNamedAndCleanContract(t *testing.T) {
	clean := Drift{CLI: "claude-tmux", CatalogCount: 2, LiveCount: 2}
	if !clean.Clean() {
		t.Errorf("empty-diff Drift must be Clean(): %+v", clean)
	}
	dirty := Drift{CLI: "claude-tmux", InCatalogNotLive: []string{"/gone"}}
	if dirty.Clean() {
		t.Errorf("Drift with InCatalogNotLive must NOT be Clean(): %+v", dirty)
	}
}
