package phasecoherence

import "testing"

func TestCanonicalRoleFoldsCaseBeforeAliasLookup(t *testing.T) {
	tests := map[string]string{
		"Build":       "builder",
		"BUILD":       "builder",
		"Audit":       "auditor",
		"AUDIT":       "auditor",
		"Scout":       "scout",
		"SHIP":        "ship",
		"CustomPhase": "customphase",
		"scout":       "scout",
		"builder":     "builder",
		"build":       "builder",
		"auditor":     "auditor",
		"audit":       "auditor",
		"intent":      "intent",
		"memo":        "memo",
	}

	for input, want := range tests {
		if got := canonicalRole(input); got != want {
			t.Fatalf("canonicalRole(%q) = %q, want %q", input, got, want)
		}
	}
}
