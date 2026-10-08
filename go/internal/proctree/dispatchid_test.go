package proctree

import "testing"

func TestDispatchID_RoundTripsThroughItsString(t *testing.T) {
	t.Parallel()
	id := DispatchID{Run: "01M4CY79", Cycle: 1835, Agent: "build", Owner: 4242, Nonce: "1t9f3a"}

	got, ok := ParseDispatchID(id.String())

	if !ok || got != id {
		t.Errorf("ParseDispatchID(%q) = %+v, %v, want %+v", id.String(), got, ok, id)
	}
	if id.String() != "01M4CY79/1835/build/p4242n1t9f3a" {
		t.Errorf("String() = %q", id.String())
	}
}

func TestDispatchID_AnAgentOrRunWithASlashCannotSplitTheID(t *testing.T) {
	t.Parallel()
	id := DispatchID{Run: "a/b", Cycle: 1, Agent: "code/review", Owner: 77, Nonce: "2"}

	got, ok := ParseDispatchID(id.String())

	if !ok || got.Owner != 77 || got.Cycle != 1 {
		t.Errorf("ParseDispatchID(%q) = %+v, %v, want owner 77 and cycle 1", id.String(), got, ok)
	}
}

func TestParseDispatchID_RefusesEveryMalformedTag(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"", "x", "r/1/a", "r/x/a/p5n1", "r/1/a/5n1", "r/1/a/p1n1", "r/1/a/p0n1", "r/1/a/p-5n1", "r/1/a/p5", "r/1/a/p5n", "r/-1/a/p5n1"} {
		if got, ok := ParseDispatchID(s); ok {
			t.Errorf("ParseDispatchID(%q) = %+v, want a refusal", s, got)
		}
	}
}
