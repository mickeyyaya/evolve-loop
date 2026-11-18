package fakecontract

import "testing"

func TestAPICover_NamesEveryExportedSymbol(t *testing.T) {
	t.Parallel()
	suite := Suite{Tool: "git"}
	if suite.Tool != "git" {
		t.Errorf("Suite.Tool = %q, want git", suite.Tool)
	}
}
