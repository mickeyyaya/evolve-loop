package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Pinned on the recipe text, not a live run, so the test never mutates the tree.
func TestComposedApicoverGate_TargetRecipeEnforces(t *testing.T) {
	target, ok := composedGateTargets["apicover"]
	if !ok {
		t.Fatal("composedGateTargets has no apicover entry — MissingComposedGates would fail-closed, but the gate would never run")
	}
	mk, err := os.ReadFile(filepath.Join(apicoverGoRoot(t), "Makefile"))
	if err != nil {
		t.Fatalf("read go/Makefile: %v", err)
	}
	// Recipe = the tab-indented block following the target line.
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(target) + `:[^\n]*\n((?:\t[^\n]*\n?)+)`)
	m := re.FindSubmatch(mk)
	if m == nil {
		t.Fatalf("composedGateTargets[\"apicover\"] = %q, but go/Makefile has no such target — the composed gate would report fail-closed on every run", target)
	}
	recipe := string(m[1])
	if !strings.Contains(recipe, "-enforce") {
		t.Fatalf("composedGateTargets[\"apicover\"] = %q whose recipe never passes -enforce — apicover.Run exits 0 regardless of uncovered exports, so the composed gate is a no-op (the six-recurrence warnship-apicover-ci-gap class).\nrecipe:\n%s", target, recipe)
	}
	if !strings.Contains(recipe, ".apicover-enforce") {
		t.Errorf("target %q enforces but does not read go/.apicover-enforce — it must cover the SAME package set as CI's Phase-5 step, not a hardcoded list.\nrecipe:\n%s", target, recipe)
	}
}
