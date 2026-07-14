package apicover

import "testing"

func TestEnumerate_FindsExportedFuncTypeVar(t *testing.T) {
	syms, err := Enumerate("testdata/sample")
	if err != nil {
		t.Fatalf("Enumerate: %v", err)
	}
	got := make(map[string]Symbol, len(syms))
	for _, s := range syms {
		got[s.Name] = s
	}

	want := map[string]SymbolKind{
		"ExportedFunc":                KindFunc,
		"ExportedType":                KindType,
		"ExportedType.ExportedMethod": KindMethod,
		"ExportedVar":                 KindVar,
		"ExportedConst":               KindConst,
	}
	for name, kind := range want {
		s, ok := got[name]
		if !ok {
			t.Errorf("expected exported symbol %q to be enumerated; got %v", name, names(syms))
			continue
		}
		if s.Kind != kind {
			t.Errorf("symbol %q: kind = %v, want %v", name, s.Kind, kind)
		}
	}

	notWant := []string{
		"unexportedFunc",
		"ExportedType.unexportedMethod",
		"unexportedVar",
		"IntegrationOnlyFunc", // behind //go:build integration — must be skipped
	}
	for _, name := range notWant {
		if _, ok := got[name]; ok {
			t.Errorf("did not expect unexported/build-tagged symbol %q to be enumerated", name)
		}
	}
}

func names(syms []Symbol) []string {
	out := make([]string, len(syms))
	for i, s := range syms {
		out[i] = s.Name
	}
	return out
}
