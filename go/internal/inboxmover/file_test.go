package inboxmover

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFile_FilesAnItemTheClaimFloorCanHandToALane(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	opts := Options{ProjectRoot: root, Stderr: os.Stderr}
	item := `{"id":"cli-x","kind":"feature","weight":0.4,"title":"t","summary":"s","fix":"f","acceptance":["a"]}`

	res, err := File(opts, []byte(item))

	if err != nil || filepath.Dir(res.Path) != filepath.Join(root, ".evolve", "inbox") {
		t.Fatalf("File = (%+v, %v)", res, err)
	}
	var _ FileResult = res
	if _, err := Claim(opts, "cli-x", "1790"); err != nil {
		t.Errorf("Claim of the filed item: %v", err)
	}
	if _, err := File(opts, []byte(item)); !errors.Is(err, ErrInvalidItem) {
		t.Errorf("filing the same id again: err = %v, want ErrInvalidItem", err)
	}
}
