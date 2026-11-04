package gc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoCacheBytes_SumsTheCacheEntriesAndNamesAnUnreadableCache(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for name, body := range map[string]string{"0a/0a1b-a": "1234", "0b/0b1c-d": "123456", "0b/README": "not an entry"} {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	n, err := GoCacheBytes(dir)
	if err != nil || n != 10 {
		t.Fatalf("GoCacheBytes = %d, %v; want 10, nil", n, err)
	}
	if _, err := GoCacheBytes(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("an unreadable cache must be an error, not a zero reading")
	}
}
