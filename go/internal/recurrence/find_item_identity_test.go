package recurrence

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindItem_FindsAnIDLessItemByItsFileName(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "no-id.json"), []byte(`{"weight":0.4}`), 0o644); err != nil {
		t.Fatal(err)
	}

	weight, state := findItem(dir, "no-id")

	if state != itemOpen || weight != 0.4 {
		t.Errorf("findItem = %v, %v; want the id-less item open at weight 0.4", weight, state)
	}
}

func TestFindItem_SkipsAnItemItCannotRead(t *testing.T) {
	dir := t.TempDir()
	if err := os.Symlink(filepath.Join(dir, "gone"), filepath.Join(dir, "dangling.json")); err != nil {
		t.Fatal(err)
	}

	if _, state := findItem(dir, "dangling"); state != itemAbsent {
		t.Errorf("an unreadable file is no item, got state %v", state)
	}
}
