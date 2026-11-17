package scopedset

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
)

func TestAPICover_NamesEveryExportedSymbol(t *testing.T) {
	t.Parallel()
	var (
		_ Set
		_ Escape
		_ Record
	)
	set := Compute("", nil, nil)
	if !set.All || ArtifactName == "" || len(Escapes(set, "test", baseattr.Reds{Unnamed: true})) != 0 {
		t.Error("an unknown root must select the full set, which has no escape")
	}
	if _, err := Append("", Record{}); err == nil {
		t.Error("Append accepted an empty plane")
	}
}
