package main

import "testing"

func TestEnumerate_IgnoreDirective_MarksSymbol(t *testing.T) {
	syms, err := Enumerate("testdata/sample")
	if err != nil {
		t.Fatalf("Enumerate: %v", err)
	}
	var found *Symbol
	for i := range syms {
		if syms[i].Name == "LegacyShim" {
			found = &syms[i]
		}
	}
	if found == nil {
		t.Fatal("LegacyShim should still be enumerated (just marked ignored)")
	}
	if !found.Ignored {
		t.Error("LegacyShim carries //apicover:ignore and must be marked Ignored")
	}
	if found.IgnoreReason != "legacy shim, removed in Phase 6" {
		t.Errorf("IgnoreReason = %q, want %q", found.IgnoreReason, "legacy shim, removed in Phase 6")
	}
}

func TestEnumerate_MalformedIgnore_ReturnsError(t *testing.T) {
	_, err := Enumerate("testdata/badignore")
	if err == nil {
		t.Fatal("Enumerate must return an error for an //apicover:ignore with no reason=")
	}
}
