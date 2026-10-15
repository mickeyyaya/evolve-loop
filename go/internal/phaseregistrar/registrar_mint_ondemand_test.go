package phaseregistrar

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

func mintCfg(name string) phaseconfig.PhaseConfig {
	cfg := validCfg()
	cfg.Name = name
	cfg.Catalog = ""
	return cfg
}

func TestRegister_PersonalessMintStampedOnDemand(t *testing.T) {
	dir := t.TempDir()
	r := Registrar{Bridge: fakeBridge{}, Prompts: prompts.NewFromFS(fstest.MapFS{}),
		PhasesDir: dir, ProfilesDir: t.TempDir()}
	res, err := r.Register(mintCfg("ghost-wiring-proof"))
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if res.Spec.Catalog != phasespec.CatalogOnDemand {
		t.Fatalf("spec.Catalog = %q, want on-demand — the persisted stub reds lane trees and dies at next-cycle load-agent (cycle-1567/1568)", res.Spec.Catalog)
	}
	raw, rerr := os.ReadFile(filepath.Join(dir, "ghost-wiring-proof", "phase.json"))
	if rerr != nil {
		t.Fatalf("persisted phase.json missing: %v", rerr)
	}
	var doc map[string]any
	if json.Unmarshal(raw, &doc) != nil {
		t.Fatal("unparseable phase.json")
	}
	if doc["catalog"] != "on-demand" {
		t.Fatalf("persisted catalog = %v, want on-demand ON DISK (the file outlives the process)", doc["catalog"])
	}
}

func TestRegister_ResolvablePersonaMintStaysOnMenu(t *testing.T) {
	r := Registrar{Bridge: fakeBridge{},
		Prompts: prompts.NewFromFS(fstest.MapFS{
			"agents/evolve-real-check.md": &fstest.MapFile{Data: []byte("---\nname: real-check\n---\npersona")},
		}),
		PhasesDir: t.TempDir(), ProfilesDir: t.TempDir()}
	res, err := r.Register(mintCfg("real-check"))
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if res.Spec.Catalog == phasespec.CatalogOnDemand {
		t.Fatal("a mint with a resolvable persona was demoted off the menu — the stamp over-fires")
	}
}

func TestRegister_ExplicitCatalogWordKeptVerbatim(t *testing.T) {
	cfg := mintCfg("explicit-word")
	cfg.Catalog = phasespec.CatalogOnDemand
	r := Registrar{Bridge: fakeBridge{}, Prompts: prompts.NewFromFS(fstest.MapFS{}),
		PhasesDir: t.TempDir(), ProfilesDir: t.TempDir()}
	res, err := r.Register(cfg)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if res.Spec.Catalog != phasespec.CatalogOnDemand {
		t.Fatalf("explicit catalog word clobbered: %q", res.Spec.Catalog)
	}
}
