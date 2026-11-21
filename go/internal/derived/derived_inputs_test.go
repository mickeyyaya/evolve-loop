package derived

import "testing"

func TestCatalog_EveryEntryDeclaresItsInputs(t *testing.T) {
	t.Parallel()
	for _, e := range Catalog() {
		if len(e.Inputs) == 0 && e.GoInputs == "" {
			t.Errorf("entry %s declares no Inputs and no GoInputs, so Stale never selects it and no gate checks it", e.Name)
		}
	}
}

func TestStale_AGoFileUnderGoSelectsTheDarwinSet(t *testing.T) {
	t.Parallel()
	got := Stale([]string{"go/cmd/newpkg/main.go"}, nil)

	names := map[string]bool{}
	for _, e := range got {
		names[e.Name] = true
	}
	if !names["darwin-set"] {
		t.Fatalf("Stale = %v, want the darwin-set entry for a new Go package", got)
	}
	if names["research-index"] {
		t.Fatalf("Stale = %v, a Go file must not select the research index", got)
	}
}
