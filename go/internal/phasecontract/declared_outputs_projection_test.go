package phasecontract

// declared_outputs_projection_test.go — ADR-0100 projection pins over the REAL
// registry. Reads docs/architecture/phase-registry.json, so run with -count=1.

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func realRegistry(t *testing.T) phasespec.Catalog {
	t.Helper()
	cat, err := phasespec.Load(filepath.Join("..", "..", "..", "docs", "architecture", "phase-registry.json"))
	if err != nil {
		t.Skipf("real registry not reachable: %v", err)
	}
	return cat
}

// TestRealRegistry_SecondariesArePartitioned: every declared secondary output
// is classified exactly once — owed by the agent or produced by the harness —
// so an unclassified file fails at commit time, never as a runtime surprise
// (an agent re-dispatched for a file it cannot write, or a file nobody gates).
func TestRealRegistry_SecondariesArePartitioned(t *testing.T) {
	for _, spec := range realRegistry(t).All() {
		files := spec.Outputs.Files
		if len(files) < 2 {
			if len(spec.Outputs.AgentOwed)+len(spec.Outputs.HarnessProduced) != 0 {
				t.Errorf("%s classifies secondaries but declares none", spec.Name)
			}
			continue
		}
		var secondaries []string
		for _, f := range files[1:] {
			secondaries = append(secondaries, filepath.Base(f))
		}
		classified := append(append([]string{}, spec.Outputs.AgentOwed...), spec.Outputs.HarnessProduced...)
		slices.Sort(secondaries)
		slices.Sort(classified)
		if !slices.Equal(secondaries, classified) {
			t.Errorf("%s: outputs.files[1:] = %v but agent_owed ∪ harness_produced = %v — every secondary must be classified exactly once", spec.Name, secondaries, classified)
		}
		for _, owed := range spec.Outputs.AgentOwed {
			if slices.Contains(spec.Outputs.HarnessProduced, owed) {
				t.Errorf("%s: %s is both agent-owed and harness-produced", spec.Name, owed)
			}
			if owed != filepath.Base(owed) {
				t.Errorf("%s: agent_owed entry %q must be a basename", spec.Name, owed)
			}
		}
	}
}

// TestRealRegistry_BuiltinArtifactNameMatchesDeclaredPrimary pins the two
// homes of "which file is the primary" together: the built-in contract's
// ArtifactName and the registry's outputs.files[0]. FromSpec has always used
// only Files[0]; this is the projection the adversarial review named as the
// fossil that let the two drift unobserved.
func TestRealRegistry_BuiltinArtifactNameMatchesDeclaredPrimary(t *testing.T) {
	for _, spec := range realRegistry(t).All() {
		if len(spec.Outputs.Files) == 0 {
			continue
		}
		c, ok := BuiltinResolver{}.Resolve(spec.Name)
		if !ok || c.NoArtifact {
			continue
		}
		if got, want := c.ArtifactName, filepath.Base(spec.Outputs.Files[0]); got != want {
			t.Errorf("%s: built-in ArtifactName %q != registry outputs.files[0] %q", spec.Name, got, want)
		}
	}
}

// TestCatalogResolver_OverlaysDeclaredFieldsOntoBuiltins: a built-in contract
// still takes its owed secondaries and effects from the registry entry, looked
// up by the canonical key (retro → retrospective).
func TestCatalogResolver_OverlaysDeclaredFieldsOntoBuiltins(t *testing.T) {
	cat := realRegistry(t)
	r := NewCatalogResolver(cat.Get)
	triage, ok := r.Resolve("triage")
	if !ok || !slices.Contains(triage.AgentOwedFiles, "triage-decision.json") {
		t.Fatalf("triage's built-in contract did not receive the registry's agent_owed overlay: %+v", triage.AgentOwedFiles)
	}
	spec, _ := cat.Get("retrospective")
	retro, ok := r.Resolve("retro")
	if !ok {
		t.Fatal("retro must resolve")
	}
	if !slices.Equal(retro.Effects, spec.Effects) || !slices.Equal(retro.AgentOwedFiles, spec.Outputs.AgentOwed) {
		t.Fatalf("retro resolved by its core name must carry the retrospective entry's declaration (effects %v owed %v), got effects %v owed %v",
			spec.Effects, spec.Outputs.AgentOwed, retro.Effects, retro.AgentOwedFiles)
	}
	if RegistryKey("retro") != "retrospective" || RegistryKey("advisor") != "router" || RegistryKey("build") != "build" {
		t.Fatal("RegistryKey must map retro→retrospective and advisor→router and leave others alone")
	}
	if _, ok := For("retro"); !ok {
		t.Fatal("the built-in lookup must still resolve the core name retro")
	}
}

// TestRealRegistry_EveryBuiltinWithArtifactResolvesARegistryEntry is the
// reverse direction of the ArtifactName pin: every built-in contract that
// produces a file must find its registry entry through RegistryKey, so a
// missing alias cannot leave a built-in without its declared secondaries and
// effects (the overlay would silently no-op). Built-ins with no registry
// entry at all are listed explicitly — adding one here is a decision.
func TestRealRegistry_EveryBuiltinWithArtifactResolvesARegistryEntry(t *testing.T) {
	cat := realRegistry(t)
	// Control-plane contracts, not phases: the routing brain's three
	// deliverables and the orchestrator's own state file have no registry
	// entry by design.
	notInRegistry := map[string]bool{"router": true, "router-proposal": true, "router-replan": true, "orchestrator": true}
	for _, c := range Contracts() {
		name := c.Phase
		if c.NoArtifact || notInRegistry[name] {
			continue
		}
		if _, ok := cat.Get(RegistryKey(name)); !ok {
			t.Errorf("built-in %q (artifact %s) has no registry entry under RegistryKey → %q; its declared secondaries and effects could never overlay", name, c.ArtifactName, RegistryKey(name))
		}
	}
}

// TestBuiltins_DeclareNoOwedFilesOrEffects makes the overlay's "pure
// projection" claim enforced rather than conventional: a built-in that ever
// set these would be silently clobbered by overlayDeclared.
func TestBuiltins_DeclareNoOwedFilesOrEffects(t *testing.T) {
	for _, c := range Contracts() {
		if len(c.AgentOwedFiles) != 0 || len(c.Effects) != 0 {
			t.Errorf("built-in %q declares AgentOwedFiles=%v Effects=%v — these come from the registry only", c.Phase, c.AgentOwedFiles, c.Effects)
		}
	}
}
