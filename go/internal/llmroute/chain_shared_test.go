package llmroute

import "testing"

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
