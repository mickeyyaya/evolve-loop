//go:build darwin || linux

package term

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsTerminal_OnlyAFileOnATerminalCounts(t *testing.T) {
	t.Parallel()
	if IsTerminal(strings.NewReader("x")) {
		t.Error("a reader that is not a file read as a terminal")
	}
	null, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	if IsTerminal(null) {
		t.Error("/dev/null read as a terminal")
	}
	null.Close()
	if IsTerminal(null) {
		t.Error("a closed file read as a terminal")
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if IsTerminal(r) || IsFD(int(w.Fd())) {
		t.Error("a pipe read as a terminal")
	}
	plain := filepath.Join(t.TempDir(), "plain.txt")
	if err := os.WriteFile(plain, []byte("data\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(plain)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if IsTerminal(f) {
		t.Error("a regular file read as a terminal")
	}
}
