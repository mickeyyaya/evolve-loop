package phasecontract

import (
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestSourceWriterPhases_AreTheRealTreeCatalogsSourceWriters(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	cat, _, warns, err := phasespec.MergedCatalog(root)
	if err != nil {
		t.Fatalf("MergedCatalog: %v (warnings %v)", err, warns)
	}
	var fromSpecs []string
	for _, name := range cat.Names() {
		if spec, ok := cat.Get(name); ok && spec.WritesSource {
			fromSpecs = append(fromSpecs, name)
		}
	}
	got := slices.Sorted(slices.Values(SourceWriterPhases()))
	slices.Sort(fromSpecs)
	if !slices.Equal(got, fromSpecs) {
		t.Fatalf("SourceWriterPhases = %v, but the phase specs that keep writes_source are %v; the role guard and ship staging read this list, so it must match the specs", got, fromSpecs)
	}
}
