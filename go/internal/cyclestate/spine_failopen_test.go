package cyclestate

import (
	"encoding/json"
	"testing"
)

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
