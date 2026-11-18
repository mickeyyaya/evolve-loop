package fakecontract

import (
	"reflect"
	"testing"
)

func TestAPICover_NamesEveryExportedSymbol(t *testing.T) {
	t.Parallel()
	typ := reflect.TypeOf(Suite{})
	if typ.NumField() != 1 {
		t.Fatalf("Suite has %d fields, want only Tool", typ.NumField())
	}
	f, ok := typ.FieldByName("Tool")
	if !ok || f.Type.Kind() != reflect.String {
		t.Errorf("Suite.Tool = %+v, want a string field", f)
	}
}
