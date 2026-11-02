package supersede

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func TestInboxReader_AFileThatVanishesAfterTheLocateIsAnError(t *testing.T) {
	inbox := filepath.Join(t.TempDir(), "inbox")
	dir := inboxbatch.ProcessingCycleDir(inbox, 5)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "z.json"), []byte(`{"id":"z"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	gone := errors.New("vanished")
	read := inboxReader(inbox, func(string) ([]byte, error) { return nil, gone })
	if _, err := read("z"); !errors.Is(err, gone) {
		t.Errorf("err = %v, want the read failure", err)
	}
}
