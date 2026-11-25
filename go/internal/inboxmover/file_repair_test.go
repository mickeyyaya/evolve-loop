package inboxmover

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func TestFileRepair_FilesAConsoleRoutedItemCarryingItsTier(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	opts := Options{ProjectRoot: root, Stderr: os.Stderr, PriorityClasses: []string{"stability"}, Origin: "cycle"}
	item := `{"id":"repair-x","kind":"pipeline-repair","priority_class":"stability","weight":0.4,"title":"t","summary":"s","fix":"f","acceptance":["a"]}`

	res, err := FileRepair(opts, []byte(item), RepairStamp{Tier: "T1"})

	if err != nil {
		t.Fatalf("FileRepair: %v", err)
	}
	filed, _, err := inboxbatch.LoadFile(res.Path)
	if err != nil || filed.RepairTier() != "T1" || filed.Route != "console" {
		t.Errorf("filed = tier %q route %q, %v", filed.RepairTier(), filed.Route, err)
	}
}
