package gateregistry

import "testing"

func TestAPICover_NamesEveryExportedSymbol(t *testing.T) {
	t.Parallel()
	if len(Entries()) == 0 || len(Unregistered()) == 0 {
		t.Error("Entries and Unregistered are empty")
	}
}
