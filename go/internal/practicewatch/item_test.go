package practicewatch

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func TestPracticeItem_DeclaresSurfaceForEveryFamily(t *testing.T) {
	t.Parallel()

	cases := []struct {
		family    string
		wantFiles []string
	}{
		{"claude", []string{"skills/model-notes-claude/COMPACT.md"}},
		{"agy", []string{"skills/model-notes-agy/COMPACT.md"}},
		{"codex", []string{"agents/"}},
	}

	at := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	prev := snapshot{Hash: "00000000", FetchedAt: "2026-10-31T00:00:00Z"}

	for _, tc := range cases {
		t.Run(tc.family, func(t *testing.T) {
			t.Parallel()
			raw, id := practiceItem(Source{ID: "src", Family: tc.family, Kind: KindGithubFile, Locator: "https://example.invalid/doc"}, "abcdef0123456789", prev, "new line\n", at)
			inboxDir := t.TempDir()
			if err := os.WriteFile(filepath.Join(inboxDir, "2026-11-01T00-00-00Z-"+id+".json"), raw, 0o644); err != nil {
				t.Fatal(err)
			}
			scan, err := inboxbatch.ScanDir(inboxDir)
			if err != nil || len(scan.Warnings) > 0 || len(scan.Items) != 1 {
				t.Fatalf("scan = %+v, err = %v", scan, err)
			}
			it := scan.Items[0]
			if !it.HasDeclaredSurface() {
				t.Errorf("HasDeclaredSurface() = false for family %s (files=%v)", tc.family, it.Files)
			}
			if !slices.Equal(it.Files, tc.wantFiles) {
				t.Errorf("files = %v, want %v", it.Files, tc.wantFiles)
			}
		})
	}
}
