package bridge

import (
	"reflect"
	"testing"
)

func TestUpdateArgv_ClaudeAndAgyDeclareTheirOwnUpdater(t *testing.T) {
	t.Parallel()
	want := map[string][]string{
		"claude-tmux": {"claude", "update"},
		"agy-tmux":    {"agy", "update"},
	}
	for name, argv := range want {
		m, err := LoadManifest(name)
		if err != nil {
			t.Fatalf("LoadManifest(%s): %v", name, err)
		}
		if !reflect.DeepEqual(m.UpdateArgv, argv) {
			t.Errorf("%s update_argv = %q, want %q", name, m.UpdateArgv, argv)
			continue
		}
		if m.UpdateArgv[0] != m.Binary {
			t.Errorf("%s updater runs %q, not the family binary %q", name, m.UpdateArgv[0], m.Binary)
		}
	}
}

func TestUpdateArgv_EveryOtherManifestDeclaresNoUpdater(t *testing.T) {
	t.Parallel()
	for _, name := range ManifestNames() {
		if name == "claude-tmux" || name == "agy-tmux" {
			continue
		}
		m, err := LoadManifest(name)
		if err != nil {
			t.Fatalf("LoadManifest(%s): %v", name, err)
		}
		if len(m.UpdateArgv) != 0 {
			t.Errorf("%s declares update_argv %q; only the family manifests the boundary updater reads declare one", name, m.UpdateArgv)
		}
	}
}

func TestUpdateArgv_ParsesFromManifestJSON(t *testing.T) {
	t.Parallel()
	m, err := parseManifest("fake-tmux", []byte(`{"cli":"fake-tmux","binary":"fake","update_argv":["fake","self-update","--yes"]}`))
	if err != nil {
		t.Fatalf("parseManifest: %v", err)
	}
	if want := []string{"fake", "self-update", "--yes"}; !reflect.DeepEqual(m.UpdateArgv, want) {
		t.Errorf("UpdateArgv = %q, want %q", m.UpdateArgv, want)
	}
}
