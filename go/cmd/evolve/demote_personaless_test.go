package main

// demote_personaless_test.go — the registration-seam prevention layer for
// cycle-1551: a discovered spec whose persona doc does not load is demoted to
// catalog:"on-demand" IN MEMORY before the catalog merge, so the SELECT menu
// never offers an undispatchable phase — on any host, tracked or not.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

func TestDemotePersonalessSpecs(t *testing.T) {
	prm := prompts.NewFromFS(fstest.MapFS{
		"agents/evolve-has-doc.md": &fstest.MapFile{Data: []byte("---\nname: has-doc\n---\npersona body")},
	})
	specs := []phasespec.PhaseSpec{
		{Name: "has-doc", Optional: true},                              // resolvable: untouched
		{Name: "no-doc", Optional: true},                               // cycle-1551 shape: demoted
		{Name: "already-hidden", Optional: true, Catalog: "on-demand"}, // already off the menu: untouched
		{Name: "ship", Optional: true},                                 // control role (name-inferred): exempt
		{Name: "script-phase", Optional: true, Kind: "script"},         // non-llm: exempt
	}
	out, warns := demotePersonalessSpecs(specs, prm)
	if specs[1].Catalog == phasespec.CatalogOnDemand {
		t.Fatalf("input slice mutated — the helper must copy")
	}
	if out[0].Catalog != "" {
		t.Errorf("resolvable spec must keep its catalog word, got %q", out[0].Catalog)
	}
	if out[1].Catalog != phasespec.CatalogOnDemand {
		t.Errorf("persona-less spec must be demoted to on-demand, got %q", out[1].Catalog)
	}
	for _, i := range []int{2, 3, 4} {
		if out[i].Catalog != specs[i].Catalog {
			t.Errorf("exempt spec %q changed catalog %q -> %q", out[i].Name, specs[i].Catalog, out[i].Catalog)
		}
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "no-doc") || !strings.Contains(warns[0], "evolve-no-doc.md") {
		t.Errorf("warns = %v, want exactly one naming the demoted phase and its persona path", warns)
	}
}

// The COMPOSED path (discovery + clamp + demotion) is what cmd_cycle actually
// calls — this pins the wiring, not just the helper: a persona-less spec
// discovered from disk must come back demoted from discoverUserSpecsClamped
// itself, so no future caller can forget the demotion step.
func TestDiscoverUserSpecsClamped_DemotesPersonalessFromDisk(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "phases", "ghost-check")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "phase.json"),
		[]byte(`{"name":"ghost-check","optional":true,"archetype":"evaluate"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	specs, warns := discoverUserSpecsClamped(root, prompts.NewFromFS(fstest.MapFS{}))
	var got *phasespec.PhaseSpec
	for i := range specs {
		if specs[i].Name == "ghost-check" {
			got = &specs[i]
		}
	}
	if got == nil {
		t.Fatalf("ghost-check not discovered; specs=%v warns=%v", specs, warns)
	}
	if got.Catalog != phasespec.CatalogOnDemand {
		t.Errorf("composed path returned catalog=%q, want on-demand — the demotion is not wired into discovery", got.Catalog)
	}
}
