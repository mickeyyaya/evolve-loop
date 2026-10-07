package faillearn

import (
	"errors"
	"os"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func TestWriteInboxItems_RefusesAnItemWithNoPriorityClassAndWritesNone(t *testing.T) {
	dir := t.TempDir()
	items := []InboxItem{
		{ID: "classed", Title: "t", Weight: 0.75, Kind: "bug", Priority: "H", PriorityClass: "correctness", InjectedBy: "x"},
		{ID: "classless", Title: "t", Weight: 0.75, Kind: "bug", Priority: "H", InjectedBy: "x"},
	}
	err := writeConfig{inboxDir: dir, inboxItems: items}.writeInboxItems()
	if !errors.Is(err, inboxbatch.ErrNoPriorityClass) {
		t.Fatalf("writeInboxItems = %v, want ErrNoPriorityClass", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("the batch is refused before any write: %v", entries)
	}
}
