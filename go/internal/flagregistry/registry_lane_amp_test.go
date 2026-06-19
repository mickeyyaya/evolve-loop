// registry_lane_amp_test.go — Cycle-1 test-amplification adversarial tests
// for the EVOLVE_LANE registry row added in Slice 2 (flagregistry fix).
//
// These probe invariants orthogonal to the basic Lookup spot-check:
//   - Row completeness: StatusActive, non-empty Cluster, non-empty Doc
//   - Cluster references the correct ADR (ADR-0049)
//   - Doc describes the override semantics (not a placeholder)
//   - EVOLVE_LANE is alphabetically ordered relative to its neighbors
//
// Anti-bias: written from the specification only; implementation not read.

package flagregistry

import (
	"strings"
	"testing"
)

// TestAmplify_EvolveLane_RowIsActiveWithCluster verifies that the EVOLVE_LANE
// row carries StatusActive (not Internal/TestSeam) and has a non-empty
// Cluster that names the concurrency/fleet grouping.
func TestAmplify_EvolveLane_RowIsActiveWithCluster(t *testing.T) {
	f, ok := Lookup("EVOLVE_LANE")
	if !ok {
		t.Fatal("EVOLVE_LANE missing from registry")
	}
	if f.Status != StatusActive {
		t.Errorf("EVOLVE_LANE.Status = %q, want %q", f.Status, StatusActive)
	}
	if f.Cluster == "" {
		t.Error("EVOLVE_LANE.Cluster is empty; expected fleet/concurrency grouping")
	}
}

// TestAmplify_EvolveLane_ClusterMentionsADR0049 guards that the Cluster field
// references the ADR that owns the concurrency design (ADR-0049), preventing
// future misclassification under a different cluster group.
func TestAmplify_EvolveLane_ClusterMentionsADR0049(t *testing.T) {
	f, ok := Lookup("EVOLVE_LANE")
	if !ok {
		t.Fatal("EVOLVE_LANE missing from registry")
	}
	if !strings.Contains(f.Cluster, "ADR-0049") {
		t.Errorf("EVOLVE_LANE.Cluster = %q; want it to reference ADR-0049 (the concurrent-worktree architecture ADR)", f.Cluster)
	}
}

// TestAmplify_EvolveLane_DocIsNonEmpty guards against a Doc that is empty
// or a StatusInternal placeholder ("Undocumented production reader"). An
// operator-facing Active flag MUST carry a meaningful Doc for the index.
func TestAmplify_EvolveLane_DocIsNonEmpty(t *testing.T) {
	f, ok := Lookup("EVOLVE_LANE")
	if !ok {
		t.Fatal("EVOLVE_LANE missing from registry")
	}
	if f.Doc == "" {
		t.Error("EVOLVE_LANE.Doc is empty")
	}
	if strings.HasPrefix(f.Doc, "Undocumented") {
		t.Errorf("EVOLVE_LANE.Doc still has placeholder text: %q", f.Doc)
	}
}

// TestAmplify_EvolveLane_DocMentionsOverrideSemantics verifies that the Doc
// explains the readability-only override semantics (the hash default is still
// the correctness ground truth). This prevents future confusion about whether
// EVOLVE_LANE affects collision safety.
func TestAmplify_EvolveLane_DocMentionsOverrideSemantics(t *testing.T) {
	f, ok := Lookup("EVOLVE_LANE")
	if !ok {
		t.Fatal("EVOLVE_LANE missing from registry")
	}
	// The Doc should mention that the flag provides readability only (not correctness).
	doc := strings.ToLower(f.Doc)
	if !strings.Contains(doc, "readab") {
		t.Errorf("EVOLVE_LANE.Doc does not mention readability semantics: %q", f.Doc)
	}
}

// TestAmplify_EvolveLane_AlphabeticalOrderInvariant checks that EVOLVE_LANE
// sits between its immediate alphabetical neighbors in the sorted All slice,
// which guards the "registry must stay sorted" invariant for the new row.
func TestAmplify_EvolveLane_AlphabeticalOrderInvariant(t *testing.T) {
	var laneIdx int
	found := false
	for i, f := range All {
		if f.Name == "EVOLVE_LANE" {
			laneIdx = i
			found = true
			break
		}
	}
	if !found {
		t.Fatal("EVOLVE_LANE not found in All")
	}
	if laneIdx > 0 && All[laneIdx-1].Name >= "EVOLVE_LANE" {
		t.Errorf("sort violation: %q >= %q (predecessor must be less)", All[laneIdx-1].Name, "EVOLVE_LANE")
	}
	if laneIdx < len(All)-1 && All[laneIdx+1].Name <= "EVOLVE_LANE" {
		t.Errorf("sort violation: %q <= %q (successor must be greater)", All[laneIdx+1].Name, "EVOLVE_LANE")
	}
}
