package capabilities

import "testing"

func TestKeyBinding_FieldsNamed(t *testing.T) {
	kb := KeyBinding{Keys: "ctrl+c", Action: "interrupt"}
	if kb.Keys != "ctrl+c" || kb.Action != "interrupt" {
		t.Errorf("KeyBinding fields not addressable as set: %+v", kb)
	}
}

func TestHeadless_FieldsNamed(t *testing.T) {
	h := Headless{Entrypoint: "claude -p", Available: []string{"-p", "--print"}}
	if h.Entrypoint != "claude -p" {
		t.Errorf("Entrypoint=%q, want %q", h.Entrypoint, "claude -p")
	}
	if len(h.Available) != 2 || h.Available[0] != "-p" {
		t.Errorf("Available=%v, want [-p --print]", h.Available)
	}
}

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
