package evidence

import (
	"strings"
	"testing"
)

func TestTrailerKeys_DriveBuildAndParse(t *testing.T) {
	in := Trailer{Phase: "build", Challenge: "tok-xyz"}
	block := in.Build()

	if !strings.Contains(block, KeyPhase+": "+in.Phase) {
		t.Errorf("Build() must emit %q under KeyPhase; got:\n%s", in.Phase, block)
	}
	if !strings.Contains(block, KeyChallenge+": "+in.Challenge) {
		t.Errorf("Build() must emit %q under KeyChallenge; got:\n%s", in.Challenge, block)
	}

	got := Parse("subject\n\nbody\n" + block)
	if got.Phase != in.Phase {
		t.Errorf("KeyPhase round-trip: Phase=%q want %q", got.Phase, in.Phase)
	}
	if got.Challenge != in.Challenge {
		t.Errorf("KeyChallenge round-trip: Challenge=%q want %q", got.Challenge, in.Challenge)
	}
}
