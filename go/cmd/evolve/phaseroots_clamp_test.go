package main

// phaseroots_clamp_test.go — wiring proof for the load-time registrar-parity
// clamp (inbox loadtime-userspec-registrar-clamp; ADR-0073 Finding 1): the
// clamp must run in the COMPOSED cmd_cycle discovery path, not just as a
// phasespec unit. A smuggled .evolve/phases/*/phase.json claiming
// writes_source:true must come out of the composed path stripped unless its
// dispatch profile is registrar-minted with sandbox enabled.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

func writeUserPhase(t *testing.T, root, name string, spec map[string]any) {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "phases", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(spec)
	if err := os.WriteFile(filepath.Join(dir, "phase.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverUserSpecsClamped_ComposedPathStripsSmuggledWriter(t *testing.T) {
	root := t.TempDir()
	writeUserPhase(t, root, "smuggled-writer", map[string]any{
		"name": "smuggled-writer", "optional": true, "writes_source": true})
	writeUserPhase(t, root, "minted-writer", map[string]any{
		"name": "minted-writer", "optional": true, "writes_source": true})
	profDir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(profDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profDir, "minted-writer.json"),
		[]byte(`{"name":"minted-writer","sandbox":{"enabled":true}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	specs, warns := discoverUserSpecsClamped(root, prompts.NewFromFS(fstest.MapFS{}))
	byName := map[string]bool{}
	for _, s := range specs {
		byName[s.Name] = s.WritesSource
	}
	if byName["smuggled-writer"] {
		t.Fatal("composed discovery path admitted a smuggled writes_source spec with no sandboxed profile — the ADR-0073 hole is open")
	}
	if !byName["minted-writer"] {
		t.Fatal("composed path stripped a VERIFIED sandboxed writer — over-clamped")
	}
	if !strings.Contains(strings.Join(warns, "\n"), "smuggled-writer") {
		t.Errorf("the strip must surface in the composed path's warnings: %v", warns)
	}
}
