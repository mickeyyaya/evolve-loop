package cyclestate

import (
	"encoding/json"
	"testing"
)

// TestSpineFailOpen_JSONShapeIsTheOperatorSurface pins the wire form of
// SpineFailOpen: the dossier's on-disk record is the ONLY surface an operator or
// a later sweep can read, so the (phase, missing_artifact) pair must serialize
// under those exact keys, and an absent reason must not emit an empty field.
func TestSpineFailOpen_JSONShapeIsTheOperatorSurface(t *testing.T) {
	raw, err := json.Marshal(SpineFailOpen{Phase: "ship", MissingArtifact: "build", Reason: "would-block at enforce"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const want = `{"phase":"ship","missing_artifact":"build","reason":"would-block at enforce"}`
	if string(raw) != want {
		t.Errorf("SpineFailOpen JSON = %s, want %s", raw, want)
	}

	bare, err := json.Marshal(SpineFailOpen{Phase: "audit", MissingArtifact: "build"})
	if err != nil {
		t.Fatalf("marshal bare: %v", err)
	}
	if string(bare) != `{"phase":"audit","missing_artifact":"build"}` {
		t.Errorf("reason must be omitempty; got %s", bare)
	}
}

// TestCycleResult_AccumulatesSpineFailOpens — the counter's whole point is that
// repeats ADD UP: a 76-event epidemic must read as 76, never as 1.
func TestCycleResult_AccumulatesSpineFailOpens(t *testing.T) {
	var r CycleResult
	if len(r.SpineFailOpens) != 0 {
		t.Fatalf("zero-value CycleResult carries %d fail-opens, want 0", len(r.SpineFailOpens))
	}
	for i := 0; i < 3; i++ {
		r.SpineFailOpens = append(r.SpineFailOpens, SpineFailOpen{Phase: "ship", MissingArtifact: "build"})
	}
	if len(r.SpineFailOpens) != 3 {
		t.Errorf("accumulated %d records, want 3", len(r.SpineFailOpens))
	}
}
