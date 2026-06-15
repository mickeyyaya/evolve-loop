package sample

import "testing"

// usage_test.go is a parse fixture for NamesReferencedInTests. testdata/ is
// ignored by the go tool, so this file is never compiled as part of any build.
func TestUsesExportedAPI(t *testing.T) {
	ExportedFunc()
	v := ExportedVar
	_ = v
	e := ExportedType{}
	e.ExportedMethod()
}
