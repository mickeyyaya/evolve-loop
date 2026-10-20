package shiperr

import "testing"

func TestCodeForeignPredicatePackage_Vocab(t *testing.T) {
	if got, want := string(CodeForeignPredicatePackage), "FOREIGN_PREDICATE_PACKAGE"; got != want {
		t.Errorf("CodeForeignPredicatePackage = %q, want %q", got, want)
	}
	if SignalCode(CodeForeignPredicatePackage) != "SHIP_FOREIGN_PREDICATE_PACKAGE" {
		t.Errorf("SignalCode = %q, want SHIP_FOREIGN_PREDICATE_PACKAGE", SignalCode(CodeForeignPredicatePackage))
	}
}
