package inputindex

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileEntry_ListsAPipelineFileWithItsDigestAndSkipsAMissingFile(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	path := f.write(".evolve/incidents/inc-a.json", `{"summary":"a CI run on main ran past the hang limit"}`)

	e, ok, err := FileEntry(f.root, path, "incident", "incident", "the incident record")
	if err != nil || !ok {
		t.Fatalf("FileEntry: ok=%v err=%v", ok, err)
	}
	if e.Path != ".evolve/incidents/inc-a.json" || e.Class != "incident" || e.Summary != "a CI run on main ran past the hang limit" || e.SHA256 == "" || e.Why != "the incident record" {
		t.Fatalf("entry %+v", e)
	}
	if _, ok, err := FileEntry(f.root, filepath.Join(f.root, "absent.json"), "incident", "incident", ""); ok || err != nil {
		t.Fatalf("a missing file is no entry: ok=%v err=%v", ok, err)
	}
	dir := filepath.Join(f.root, "dir.json")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := FileEntry(f.root, dir, "incident", "incident", ""); err == nil {
		t.Fatal("a directory must be an error")
	}
}
