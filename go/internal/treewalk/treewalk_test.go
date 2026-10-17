package treewalk

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestSnapshotRoundTripAndDiffTable(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	before, err := Take(root)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "b.json")
	if err := before.Write(file); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(file)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(loaded.Paths, before.Paths) {
		t.Fatalf("round trip = %v, want %v", loaded.Paths, before.Paths)
	}
	if err := os.WriteFile(filepath.Join(root, "a", "x"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "a")); err == nil {
		t.Fatal("expected non-empty dir removal to fail")
	}
	after, err := Take(root)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name          string
		before, after Snapshot
		want          []string
	}{
		{"added", loaded, after, []string{filepath.Join("a", "x")}},
		{"removed ignored", after, loaded, []string{}},
		{"identical", after, after, []string{}},
	}
	for _, c := range cases {
		if got := Diff(c.before, c.after); !slices.Equal(got, c.want) {
			t.Errorf("%s: Diff = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestLoadRejectsMalformedBaseline(t *testing.T) {
	file := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(file, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(file); err == nil {
		t.Error("Load of malformed JSON returned nil error")
	}
	if _, err := Take(file); err == nil {
		t.Error("Take of a file root returned nil error")
	}
}
