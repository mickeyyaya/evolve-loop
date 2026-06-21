package phasecmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPhasesList_MultiRootShowsProvenance is the WP5 integration check: a
// phase shipped by a plugin root (EVOLVE_PHASE_ROOTS) must appear in `phases
// list` with its discovery root, alongside the project-local one.
func TestPhasesList_MultiRootShowsProvenance(t *testing.T) {
	root := createFixtureProject(t)
	writeUserPhase(t, root, "local-check", `{"name":"local-check","optional":true}`)

	pluginRoot := t.TempDir()
	dir := filepath.Join(pluginRoot, "plugin-check")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "phase.json"),
		[]byte(`{"name":"plugin-check","optional":true,"categories":["security"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_PHASE_ROOTS", ".evolve/phases:"+pluginRoot)

	var out, errb bytes.Buffer
	if code := RunPhases([]string{"list"}, nil, &out, &errb); code != 0 {
		t.Fatalf("list exit = %d (stderr=%q)", code, errb.String())
	}
	listing := out.String()
	for _, want := range []string{"local-check", "plugin-check", ".evolve/phases", pluginRoot} {
		if !strings.Contains(listing, want) {
			t.Errorf("listing missing %q:\n%s", want, listing)
		}
	}

	// validate must see the plugin phase too.
	out.Reset()
	if code := RunPhases([]string{"validate", "plugin-check"}, nil, &out, &errb); code != 0 {
		t.Fatalf("validate exit = %d (stdout=%q)", code, out.String())
	}
	if !strings.Contains(out.String(), "OK") {
		t.Errorf("plugin phase should validate OK; got %q", out.String())
	}
}
