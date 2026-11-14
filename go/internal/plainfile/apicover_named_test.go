package plainfile_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/plainfile"
)

func TestExportsAreNamed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plain.go")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := plainfile.Open(path)
	if err != nil {
		t.Fatalf("Open = %v", err)
	}
	_ = f.Close()
	if body, err := plainfile.Read(path, 1); err != nil || string(body) != "x" {
		t.Fatalf("Read = (%q, %v)", body, err)
	}
	if _, err := plainfile.ReadWith(path, 0, plainfile.Open); !errors.Is(err, plainfile.ErrTooLarge) {
		t.Fatalf("ReadWith over the limit = %v, want ErrTooLarge", err)
	}
	if errors.Is(plainfile.ErrNotPlain, plainfile.ErrTooLarge) {
		t.Fatal("the two refusals must stay distinct")
	}
}
