package inputindex

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestOpen_ReturnsTheBodyOfAListedEntryAndRefusesEachOtherRead(t *testing.T) {
	f := retryFixture(t)
	idx := f.build(7, "build")
	scout := mustEntry(t, idx, "scout-report", 7)
	e, body, err := idx.Open(f.root, scout.ID)
	if err != nil || e != scout || string(body) != "# Scout: replay harness\n" {
		t.Fatalf("Open(%s) = %+v %q %v", scout.ID, e, body, err)
	}
	if _, _, err := idx.Open(f.root, "z9"); !errors.Is(err, ErrUnknownEntry) {
		t.Fatalf("an unknown id: err = %v, want ErrUnknownEntry", err)
	}
	foreign := idx
	foreign.Entries = []Entry{{ID: "d1", Path: "../outside"}, {ID: "d2", Path: "/etc/passwd"}}
	for _, id := range []string{"d1", "d2"} {
		if _, _, err := foreign.Open(f.root, id); !errors.Is(err, ErrOutsideRoot) {
			t.Fatalf("%s: err = %v, want ErrOutsideRoot", id, err)
		}
	}
	f.write(scout.Path, "# a later write\n")
	if _, _, err := idx.Open(f.root, scout.ID); !errors.Is(err, ErrChangedBody) {
		t.Fatalf("a body that differs from its sha256: err = %v, want ErrChangedBody", err)
	}
	if err := os.Remove(filepath.Join(f.root, scout.Path)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := idx.Open(f.root, scout.ID); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a missing body: err = %v, want fs.ErrNotExist", err)
	}
}
