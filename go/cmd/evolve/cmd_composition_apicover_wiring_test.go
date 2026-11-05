package main

import (
	"strings"
	"testing"
)

func TestComposedApicoverGate_TargetRecipeEnforces(t *testing.T) {
	t.Parallel()
	target, ok := composedGateTargets["apicover"]
	if !ok {
		t.Fatal("composedGateTargets has no apicover entry — MissingComposedGates would fail-closed, but the gate would never run")
	}
	recipe := strings.Join(makeRecipe(t, apicoverGoRoot(t), target), "\n")
	if !strings.Contains(recipe, "-enforce") {
		t.Fatalf("composedGateTargets[\"apicover\"] = %q whose recipe never passes -enforce — apicover.Run exits 0 regardless of uncovered exports, so the composed gate is a no-op (the six-recurrence warnship-apicover-ci-gap class).\nrecipe:\n%s", target, recipe)
	}
	if !strings.Contains(recipe, ".apicover-enforce") {
		t.Errorf("target %q enforces but does not read go/.apicover-enforce — it must cover the SAME package set as CI's Phase-5 step, not a hardcoded list.\nrecipe:\n%s", target, recipe)
	}
}
