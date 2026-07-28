package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

// TestSpineFailOpenAlias_IsTheCyclestateRecord pins that core's SpineFailOpen is
// the SAME type as the cyclestate leaf's (a re-export, not a parallel struct).
// A second definition would let core's records and the dossier's projection
// drift apart — the exact single-source-with-projection rule this telemetry
// follows from SkippedPhase.
func TestSpineFailOpenAlias_IsTheCyclestateRecord(t *testing.T) {
	rec := SpineFailOpen{Phase: "ship", MissingArtifact: "build", Reason: "would-block at enforce"}
	var asLeaf cyclestate.SpineFailOpen = rec // compile-time proof of identity
	if asLeaf.Phase != "ship" || asLeaf.MissingArtifact != "build" {
		t.Errorf("alias round-trip lost fields: %+v", asLeaf)
	}
	cr := &cycleRun{}
	cr.recordSpineFailOpen(Phase(rec.Phase), rec.MissingArtifact, rec.Reason)
	if len(cr.result.SpineFailOpens) != 1 || cr.result.SpineFailOpens[0] != asLeaf {
		t.Errorf("recorded %+v, want exactly [%+v]", cr.result.SpineFailOpens, asLeaf)
	}
}
