package scout

import (
	"reflect"
	"testing"
)

func sliceCount(slices []ScanSlice) (total int) {
	for _, s := range slices {
		total += len(s.Packages)
	}
	return
}

func TestPartitionPackages(t *testing.T) {
	t.Parallel()

	t.Run("chunks into n contiguous slices, all packages preserved", func(t *testing.T) {
		t.Parallel()
		pkgs := []string{"a", "j", "c", "e", "b", "h", "d", "g", "f", "i"} // 10, unsorted
		got := partitionPackages(pkgs, 4)
		if len(got) != 4 {
			t.Fatalf("want 4 slices, got %d: %+v", len(got), got)
		}
		if n := sliceCount(got); n != 10 {
			t.Errorf("want 10 packages across slices, got %d", n)
		}
		// chunked ceil(10/4)=3 ⇒ [3,3,3,1]; sorted+contiguous
		if !reflect.DeepEqual(got[0].Packages, []string{"a", "b", "c"}) {
			t.Errorf("slice-1 = %v, want [a b c] (sorted+contiguous)", got[0].Packages)
		}
		if got[3].Packages[0] != "j" {
			t.Errorf("last slice should hold the lexical tail; got %v", got[3].Packages)
		}
		if got[0].ID != "slice-1" {
			t.Errorf("ID=%q, want slice-1", got[0].ID)
		}
	})

	t.Run("excludes the core hub", func(t *testing.T) {
		t.Parallel()
		pkgs := []string{
			"github.com/x/go/internal/config",
			"github.com/x/go/internal/core",          // hub — excluded
			"github.com/x/go/internal/core/evidence", // hub subpkg — excluded
			"github.com/x/go/internal/dossier",
		}
		got := partitionPackages(pkgs, 4)
		if n := sliceCount(got); n != 2 {
			t.Errorf("core hub must be excluded; want 2 packages, got %d (%+v)", n, got)
		}
		for _, s := range got {
			for _, p := range s.Packages {
				if isCoreHub(p) {
					t.Errorf("core hub leaked into a MAP slice: %q", p)
				}
			}
		}
	})

	t.Run("n greater than package count clamps to one slice per package", func(t *testing.T) {
		t.Parallel()
		got := partitionPackages([]string{"a", "b"}, 5)
		if len(got) != 2 {
			t.Errorf("want 2 slices, got %d", len(got))
		}
	})

	t.Run("degenerate inputs", func(t *testing.T) {
		t.Parallel()
		if got := partitionPackages(nil, 4); got != nil {
			t.Errorf("empty input ⇒ nil, got %+v", got)
		}
		if got := partitionPackages([]string{"a", "b", "c"}, 0); len(got) != 1 || sliceCount(got) != 3 {
			t.Errorf("n<1 ⇒ single slice with all packages; got %+v", got)
		}
	})
}

// Uneven chunking (len not divisible by n) leaves a trailing slot empty;
// partitionPackages must compact it — pinning that the compaction is NOT dead
// code (5 pkgs at n=4: chunks [2,2,1] fill 3 slots, the 4th would be empty).
func TestPartitionPackages_UnevenLeavesNoEmptySlices(t *testing.T) {
	t.Parallel()
	got := partitionPackages([]string{"a", "b", "c", "d", "e"}, 4)
	if len(got) != 3 {
		t.Fatalf("5 pkgs at n=4 ⇒ 3 non-empty slices, got %d: %+v", len(got), got)
	}
	if sliceCount(got) != 5 {
		t.Errorf("all 5 packages must be present, got %d", sliceCount(got))
	}
	for _, s := range got {
		if len(s.Packages) == 0 {
			t.Errorf("no empty slice may be returned: %+v", got)
		}
	}
}
