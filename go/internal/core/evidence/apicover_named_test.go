package evidence

import (
	"strings"
	"testing"
)

// TestTrailerKeys_DriveBuildAndParse pins the const→wire-format contract:
// Build must render the Phase under KeyPhase and the Challenge under
// KeyChallenge (the load-bearing verification keys), and Parse must route
// each rendered line back to the matching field. Naming the consts (rather
// than hard-coded "Evolve-Phase"/"Challenge-Token" literals) keeps the test
// coupled to the real keys, so renaming a const that the emitter/detector
// share would fail here instead of silently diverging.
func TestTrailerKeys_DriveBuildAndParse(t *testing.T) {
	in := Trailer{Phase: "build", Challenge: "tok-xyz"}
	block := in.Build()

	// Each value is emitted under its declared key constant.
	if !strings.Contains(block, KeyPhase+": "+in.Phase) {
		t.Errorf("Build() must emit %q under KeyPhase; got:\n%s", in.Phase, block)
	}
	if !strings.Contains(block, KeyChallenge+": "+in.Challenge) {
		t.Errorf("Build() must emit %q under KeyChallenge; got:\n%s", in.Challenge, block)
	}

	// Parse routes each keyed line back to the matching field (round-trip).
	got := Parse("subject\n\nbody\n" + block)
	if got.Phase != in.Phase {
		t.Errorf("KeyPhase round-trip: Phase=%q want %q", got.Phase, in.Phase)
	}
	if got.Challenge != in.Challenge {
		t.Errorf("KeyChallenge round-trip: Challenge=%q want %q", got.Challenge, in.Challenge)
	}
}
