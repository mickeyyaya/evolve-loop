package llmroute

import "testing"

// TestDefaultTriggers_IsTheConservativeSet names the exported default the
// bridge-chain decorator falls back to when a resolver returns no triggers:
// the same {80, 81, 85, 124, 127} every profile without cli_fallback_on_exit gets.
func TestDefaultTriggers_IsTheConservativeSet(t *testing.T) {
	got := DefaultTriggers()
	if len(got) != 5 || got[0] != 80 || got[1] != 81 || got[2] != 85 || got[3] != 124 || got[4] != 127 {
		t.Fatalf("DefaultTriggers = %v", got)
	}
	got[0] = 1
	if DefaultTriggers()[0] != 80 {
		t.Fatal("DefaultTriggers must return a copy")
	}
}
