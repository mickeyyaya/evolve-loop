package inputindex

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHardLinked_ASecondLinkIsHardLinkedAndAPlainFileIsNot(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	plain := filepath.Join(dir, "plain.json")
	if err := os.WriteFile(plain, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(plain)
	if err != nil || HardLinked(info) {
		t.Fatalf("a file with one link: HardLinked = true (err %v), want false", err)
	}
	if err := os.Link(plain, filepath.Join(dir, "second.json")); err != nil {
		t.Fatal(err)
	}
	info, err = os.Lstat(plain)
	if err != nil || !HardLinked(info) {
		t.Fatalf("a file with two links: HardLinked = false (err %v), want true", err)
	}
}
