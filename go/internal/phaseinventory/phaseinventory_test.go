package phaseinventory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fixtureProject lays out a minimal project: a built-in registry with two
// phases (one Control, which must still be inventoried — the inventory is a
// catalog, not an advisor projection) and one user phase under .evolve/phases.
func fixtureProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	regDir := filepath.Join(root, "docs", "architecture")
	if err := os.MkdirAll(regDir, 0o755); err != nil {
		t.Fatal(err)
	}
	registry := `{
  "schema_version": 4,
  "phases": [
    { "name": "scout", "kind": "llm", "agent": "evolve-scout",
      "outputs": { "files": ["scout-report.md"] },
      "classify": { "require_sections": ["Proposed Tasks"] } },
    { "name": "ship", "archetype": "control" }
  ]
}`
	if err := os.WriteFile(filepath.Join(regDir, "phase-registry.json"), []byte(registry), 0o644); err != nil {
		t.Fatal(err)
	}

	phaseDir := filepath.Join(root, ".evolve", "phases", "reproduce-bug")
	if err := os.MkdirAll(phaseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	userPhase := `{
  "name": "reproduce-bug", "kind": "llm", "optional": true, "archetype": "evaluate",
  "writes_source": true,
  "description": "Failing reproduction test before any patch.",
  "when_to_use": "bugfix cycles, before tdd/build.",
  "categories": ["bugfix"],
  "outputs": { "files": [".evolve/runs/cycle-{cycle}/reproduce-bug-report.md"] },
  "classify": { "require_sections": ["Reproduction", "Verification"], "verdict_on_pass": "PASS" },
  "after": "fault-localization"
}`
	if err := os.WriteFile(filepath.Join(phaseDir, "phase.json"), []byte(userPhase), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func buildAndLoad(t *testing.T, opts Options) (Result, Inventory) {
	t.Helper()
	res, err := Build(opts)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	raw, err := os.ReadFile(res.OutputPath)
	if err != nil {
		t.Fatalf("read inventory: %v", err)
	}
	var inv Inventory
	if err := json.Unmarshal(raw, &inv); err != nil {
		t.Fatalf("parse inventory: %v", err)
	}
	return res, inv
}

func entryByName(t *testing.T, inv Inventory, name string) PhaseEntry {
	t.Helper()
	for _, e := range inv.Phases {
		if e.Name == name {
			return e
		}
	}
	t.Fatalf("phase %q not in inventory (have %d phases)", name, len(inv.Phases))
	return PhaseEntry{}
}

func TestBuild_AggregatesBuiltinAndUserRoots(t *testing.T) {
	root := fixtureProject(t)
	res, inv := buildAndLoad(t, Options{ProjectRoot: root, Force: true})

	if res.PhaseCount != 3 {
		t.Errorf("PhaseCount = %d, want 3 (scout, ship, reproduce-bug)", res.PhaseCount)
	}

	repro := entryByName(t, inv, "reproduce-bug")
	if repro.Source != "user" {
		t.Errorf("reproduce-bug source = %q, want user", repro.Source)
	}
	if repro.Root != ".evolve/phases" {
		t.Errorf("reproduce-bug root = %q, want .evolve/phases (project-relative)", repro.Root)
	}
	if repro.Artifact != "reproduce-bug-report.md" {
		t.Errorf("artifact = %q (must be contract-derived basename)", repro.Artifact)
	}
	if len(repro.RequiredSections) != 2 || repro.RequiredSections[0] != "## Reproduction" {
		t.Errorf("required_sections = %v", repro.RequiredSections)
	}
	if !repro.EmitsVerdict {
		t.Error("evaluate + verdict_on_pass must mark emits_verdict")
	}
	if repro.WhenToUse == "" || repro.Description == "" {
		t.Errorf("metadata not carried: %+v", repro)
	}
	if repro.After != "fault-localization" {
		t.Errorf("after = %q", repro.After)
	}

	scout := entryByName(t, inv, "scout")
	if scout.Source != "builtin" || scout.Root != "" {
		t.Errorf("scout source/root = %q/%q, want builtin/\"\"", scout.Source, scout.Root)
	}
	if scout.Agent != "evolve-scout" {
		t.Errorf("scout agent = %q", scout.Agent)
	}
}

func TestBuild_CategoryIndex(t *testing.T) {
	root := fixtureProject(t)
	_, inv := buildAndLoad(t, Options{ProjectRoot: root, Force: true})

	bugfix := inv.CategoryIndex["bugfix"]
	if len(bugfix) != 1 || bugfix[0] != "reproduce-bug" {
		t.Errorf("category_index[bugfix] = %v", bugfix)
	}
	// Built-ins without categories land in uncategorized — degrade, not crash.
	uncat := inv.CategoryIndex["uncategorized"]
	if len(uncat) != 2 {
		t.Errorf("category_index[uncategorized] = %v, want scout+ship", uncat)
	}
}

func TestBuild_CacheHitAndForce(t *testing.T) {
	root := fixtureProject(t)
	now := time.Now()
	nowFn := func() time.Time { return now }

	res1, err := Build(Options{ProjectRoot: root, NowFn: nowFn})
	if err != nil {
		t.Fatalf("first Build: %v", err)
	}
	if res1.CacheHit {
		t.Error("first build must not be a cache hit")
	}

	res2, err := Build(Options{ProjectRoot: root, NowFn: nowFn})
	if err != nil {
		t.Fatalf("second Build: %v", err)
	}
	if !res2.CacheHit {
		t.Error("fresh inventory within TTL must be a cache hit")
	}

	res3, err := Build(Options{ProjectRoot: root, NowFn: nowFn, Force: true})
	if err != nil {
		t.Fatalf("forced Build: %v", err)
	}
	if res3.CacheHit {
		t.Error("Force must bypass the cache")
	}
}

func TestBuild_ExtraRootsAndProvenance(t *testing.T) {
	root := fixtureProject(t)
	pluginRoot := t.TempDir() // absolute, outside the project
	dir := filepath.Join(pluginRoot, "threat-model")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	spec := `{"name":"threat-model","optional":true,"categories":["security"],"description":"STRIDE pass."}`
	if err := os.WriteFile(filepath.Join(dir, "phase.json"), []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := Options{
		ProjectRoot: root,
		Roots:       []string{filepath.Join(root, ".evolve", "phases"), pluginRoot},
		Force:       true,
	}
	_, inv := buildAndLoad(t, opts)

	tm := entryByName(t, inv, "threat-model")
	if tm.Root != pluginRoot {
		t.Errorf("plugin phase root = %q, want absolute %q", tm.Root, pluginRoot)
	}
	if got := inv.CategoryIndex["security"]; len(got) != 1 || got[0] != "threat-model" {
		t.Errorf("category_index[security] = %v", got)
	}
}

func TestBuild_MissingRegistryFailOpen(t *testing.T) {
	root := t.TempDir() // no registry, no phases
	res, inv := buildAndLoad(t, Options{ProjectRoot: root, Force: true})
	if res.PhaseCount != 0 || inv.PhaseCount != 0 {
		t.Errorf("empty project must build an empty inventory; got %d", res.PhaseCount)
	}
	if len(res.Warnings) == 0 {
		t.Error("missing registry should surface a warning")
	}
}
