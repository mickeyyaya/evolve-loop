package landed_test

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/landed"
)

func TestVerdict_NamedAnUnlandedVerdictAlwaysCarriesItsReason(t *testing.T) {
	t.Parallel()
	tr := newTree(t, map[string]string{"seed.txt": "seed\n"})
	tr.write("seed.txt", "changed\n")

	var v landed.Verdict
	v, err := landed.Changes(context.Background(), gitexec.Default(tr.repo.Dir), tr.base)

	if err != nil || v.Landed || v.Reason != "seed.txt: the tree's change is not in origin/main" {
		t.Errorf("Changes = %+v (err=%v), want not landed with seed.txt named", v, err)
	}
}

func TestBlob_NamedAMissingPathIsAnAbsentBlobNotAnEmptyOne(t *testing.T) {
	t.Parallel()
	blobs, err := landed.ParseCatFileBatch([]string{"main:gone.txt", "main:empty.txt"}, "main:gone.txt missing\n0123 blob 0\n\n")
	if err != nil {
		t.Fatal(err)
	}

	var gone, empty landed.Blob = blobs["main:gone.txt"], blobs["main:empty.txt"]

	if gone.Present || !empty.Present || len(empty.Data) != 0 {
		t.Errorf("gone=%+v empty=%+v: a missing path is absent, an empty file is a present zero-length blob", gone, empty)
	}
}
