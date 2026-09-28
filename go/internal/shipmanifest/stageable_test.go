package shipmanifest

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestStageable_IsThePathspecShipStages(t *testing.T) {
	never := func(string) bool { return false }
	for _, tc := range []struct {
		name, porcelain string
		manifest        []string
		want            []string
	}{
		{"a changed declared path is staged though no file backs it", " M a.go\n", []string{"a.go"}, []string{"a.go"}},
		{"a declared directory takes only the changes it covers", "?? docs/adr/x.md\n?? stray.go\n", []string{"docs/adr"}, []string{"docs/adr/x.md"}},
		{"a manifest that covers nothing selects nothing", " M a.go\n M b.go\n", []string{"docs/none.md"}, nil},
		{"no manifest selects nothing", " M a.go\n?? residue.txt\n", nil, nil},
		{"a staged deletion is not named again", "D  gone.go\n M a.go\n", []string{"gone.go", "a.go"}, []string{"a.go"}},
		{"a staged rename's source is not named", "R  old.go -> new.go\n", []string{"old.go", "new.go"}, []string{"new.go"}},
		{"a both-deleted conflict stays named as its resolution", "DD conflict.go\n", []string{"conflict.go"}, []string{"conflict.go"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Stageable(tc.porcelain, tc.manifest, never); !slices.Equal(got, tc.want) {
				t.Fatalf("Stageable = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRegularFileIn_CountsOnlyRegularFilesUnderRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	isFile := RegularFileIn(root)
	if !isFile("f.txt") || isFile("dir") || isFile("missing.txt") {
		t.Fatalf("file=%v dir=%v missing=%v", isFile("f.txt"), isFile("dir"), isFile("missing.txt"))
	}
}
