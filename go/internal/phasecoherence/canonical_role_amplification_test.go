package phasecoherence

import "testing"

func TestCanonicalRoleNormalizesKnownAliasesAndUnknowns(t *testing.T) {
	tests := map[string]string{
		"Build":       "build",
		"BUILD":       "build",
		"Audit":       "audit",
		"AUDIT":       "audit",
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
