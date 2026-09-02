package dashboard

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// writeNDJSON writes one JSON object per line, the shape every *.ndjson
// artifact in a cycle workspace uses.
func writeNDJSON(t *testing.T, path string, lines ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// itoa keeps fixture JSON readable.
func itoa(i int) string { return strconv.Itoa(i) }

// writeFile is the plain fixture writer for markdown/json artifacts.
func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
