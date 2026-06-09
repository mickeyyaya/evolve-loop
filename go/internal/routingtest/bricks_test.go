package routingtest

import "testing"

// TestBricks_VariadicSlicesAreDefensivelyCopied pins the cycle-263 audit
// finding (C263_002): Mandatory() and Done() stored the caller's variadic
// backing array directly, so a caller mutating (or reusing) its slice after
// building the spec silently corrupted the scenario — shared-backing-storage
// aliasing. Immutability rule: return/store new objects, never alias caller
// memory.
func TestBricks_VariadicSlicesAreDefensivelyCopied(t *testing.T) {
	t.Parallel()
	args := []string{"scout", "build"}
	done := []string{"scout"}
	var s ScenarioSpec
	Mandatory(args...)(&s)
	Done(done...)(&s)
	args[0] = "MUTATED"
	done[0] = "MUTATED"
	if s.Mandatory[0] != "scout" {
		t.Fatalf("Mandatory must defensively copy its variadic args; spec saw caller mutation: %v", s.Mandatory)
	}
	if s.Completed[0] != "scout" {
		t.Fatalf("Done must defensively copy its variadic args; spec saw caller mutation: %v", s.Completed)
	}
}
