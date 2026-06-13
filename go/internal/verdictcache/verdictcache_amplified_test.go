package verdictcache

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAmplifiedLoadCorruptCacheDegradesToEmptyAfterSuccessfulPut(t *testing.T) {
	root := t.TempDir()
	now := func() time.Time { return time.Date(2026, 6, 14, 9, 0, 0, 0, time.UTC) }
	store := NewStore(root, now)

	if err := store.Put(Entry{
		TreeSHA:        "tree-before-corruption",
		Cycle:          329,
		Verdict:        "PASS",
		ArtifactSHA256: "artifact-sha",
		ArtifactPath:   "audit-report.md",
	}); err != nil {
		t.Fatalf("Put seed entry: %v", err)
	}

	cachePath := findFileContaining(t, root, "tree-before-corruption")
	if err := os.WriteFile(cachePath, []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("corrupt cache file: %v", err)
	}

	loaded, err := NewStore(root, now).Load()
	if err != nil {
		t.Fatalf("Load corrupt cache should degrade without error, got: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("Load corrupt cache = %#v, want empty advisory cache", loaded)
	}
}

func TestAmplifiedPutSameTreeSHAReplacesEntry(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root, func() time.Time {
		return time.Date(2026, 6, 14, 9, 30, 0, 0, time.UTC)
	})

	first := Entry{TreeSHA: "same-tree", Cycle: 1, Verdict: "WARN", ArtifactSHA256: "old", ArtifactPath: "old.md"}
	second := Entry{TreeSHA: "same-tree", Cycle: 2, Verdict: "PASS", ArtifactSHA256: "new", ArtifactPath: "new.md"}
	if err := store.Put(first); err != nil {
		t.Fatalf("Put first entry: %v", err)
	}
	if err := store.Put(second); err != nil {
		t.Fatalf("Put replacement entry: %v", err)
	}

	loaded, err := NewStore(root, nil).Load()
	if err != nil {
		t.Fatalf("Load replacement cache: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("cache entry count = %d, want 1 replacement entry", len(loaded))
	}
	got, ok := loaded["same-tree"]
	if !ok {
		t.Fatalf("same-tree entry missing from loaded cache: %#v", loaded)
	}
	if got.Cycle != 2 || got.Verdict != "PASS" || got.ArtifactSHA256 != "new" || got.ArtifactPath != "new.md" {
		t.Fatalf("replacement entry = %+v, want second Put contents", got)
	}
}

func findFileContaining(t *testing.T, root, needle string) string {
	t.Helper()
	var found string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || found != "" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), needle) {
			found = path
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk cache root: %v", err)
	}
	if found == "" {
		t.Fatalf("no cache file under %s contained %q", root, needle)
	}
	return found
}
