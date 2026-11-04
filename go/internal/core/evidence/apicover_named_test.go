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

func TestKeyAuditTree_IsATrailerKeyThatParseKeepsInTheBlock(t *testing.T) {
	t.Parallel()
	if KeyAuditTree != "Evolve-Audit-Tree" {
		t.Fatalf("KeyAuditTree = %q, want Evolve-Audit-Tree", KeyAuditTree)
	}
	in := Trailer{Cycle: 7, ArtifactSHA: "def", AuditTree: "abc"}
	block := in.Build()
	if !strings.Contains(block, KeyAuditTree+": abc\n") {
		t.Errorf("Build() must emit the audit tree under %s; got:\n%s", KeyAuditTree, block)
	}
	if got := Parse("subject\n" + block); got != in {
		t.Errorf("Parse(Build()) = %+v, want %+v", got, in)
	}
}
