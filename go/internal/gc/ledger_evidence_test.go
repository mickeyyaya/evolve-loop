package gc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ledgerartifacts"
)

func TestApply_RefusesTheLedgerEvidenceStore(t *testing.T) {
	dir := t.TempDir()
	digest, err := ledgerartifacts.Open(dir).Put([]byte("diff --git a/x b/x\n"))
	if err != nil {
		t.Fatal(err)
	}
	object, err := ledgerartifacts.Open(dir).Path(digest)
	if err != nil {
		t.Fatal(err)
	}
	store := filepath.Join(dir, ledgerartifacts.DirName)

	err = Apply(dir, Manifest{Items: []Item{
		{Path: object, Action: ActionDelete, Rule: "x"},
		{Path: store, Action: ActionArchive, Rule: "x"},
	}})

	if err == nil {
		t.Fatal("Apply must refuse the evidence store: a composition-verdict line whose diff is gone breaks the ledger chain")
	}
	if _, statErr := os.Stat(object); statErr != nil {
		t.Errorf("a stored diff must survive a refused Apply: %v", statErr)
	}
}
