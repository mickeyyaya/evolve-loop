package flagregistry

import (
	"sort"
	"testing"
)

// coreInfraExpected pins the irreducible core-infrastructure set by name. These
// are process-config flags (writable/readonly roots, test-harness mode), NOT
// operator feature dials, so the flag-reduction campaign metric excludes them.
// A typo/rename in the registry's Cluster marker — or accidental membership
// change — fails this test loudly rather than silently miscounting the metric.
var coreInfraExpected = []string{
	"EVOLVE_PLUGIN_ROOT",
	"EVOLVE_PROJECT_ROOT",
	"EVOLVE_TESTING",
}

func TestIsCoreInfra_MarksOnlyTheNeverConsolidateSet(t *testing.T) {
	want := map[string]bool{}
	for _, n := range coreInfraExpected {
		want[n] = true
	}
	for _, f := range All {
		got := IsCoreInfra(f)
		if got != want[f.Name] {
			t.Errorf("IsCoreInfra(%s) = %v, want %v (cluster %q)", f.Name, got, want[f.Name], f.Cluster)
		}
	}
}

func TestIsCoreInfra_ClusterMarkerConstMatchesData(t *testing.T) {
	// ClusterCoreInfra must equal the Cluster string actually used on the rows,
	// or IsCoreInfra silently classifies nothing as core.
	var found int
	for _, f := range All {
		if f.Cluster == ClusterCoreInfra {
			found++
		}
	}
	if found != len(coreInfraExpected) {
		t.Errorf("rows with Cluster==ClusterCoreInfra = %d, want %d — the marker const drifted from the data", found, len(coreInfraExpected))
	}
}

func TestLiveFeatureFlags_ExcludesCoreInfraAndNonActive(t *testing.T) {
	live := LiveFeatureFlags()
	for _, f := range live {
		if f.Status != StatusActive {
			t.Errorf("LiveFeatureFlags returned non-active flag %s (status %s)", f.Name, f.Status)
		}
		if IsCoreInfra(f) {
			t.Errorf("LiveFeatureFlags returned core-infra flag %s — core infra is not a feature dial", f.Name)
		}
	}
	// The 3 core-infra flags are Active but must NOT appear in the metric.
	names := map[string]bool{}
	for _, f := range live {
		names[f.Name] = true
	}
	for _, n := range coreInfraExpected {
		if names[n] {
			t.Errorf("core-infra flag %s leaked into LiveFeatureFlags", n)
		}
	}
}

// TestLiveFeatureFlags_EqualsActiveMinusCore documents the metric identity that
// the campaign ratchet and the ACS baseline guard both rely on:
//
//	len(LiveFeatureFlags) == count(StatusActive) - count(core-infra)
func TestLiveFeatureFlags_EqualsActiveMinusCore(t *testing.T) {
	active, core := 0, 0
	for _, f := range All {
		if f.Status == StatusActive {
			active++
			if IsCoreInfra(f) {
				core++
			}
		}
	}
	if got := len(LiveFeatureFlags()); got != active-core {
		t.Errorf("len(LiveFeatureFlags) = %d, want active(%d)-core(%d) = %d", got, active, core, active-core)
	}
	// Report the live metric so the ratchet const / baseline can be confirmed.
	t.Logf("live feature flags = %d (active=%d core=%d total rows=%d)", len(LiveFeatureFlags()), active, core, len(All))
}

func TestLiveFeatureFlags_SortedByName(t *testing.T) {
	live := LiveFeatureFlags()
	if !sort.SliceIsSorted(live, func(i, j int) bool { return live[i].Name < live[j].Name }) {
		t.Error("LiveFeatureFlags not sorted by Name (All must stay sorted)")
	}
}
