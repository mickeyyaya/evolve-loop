package modelcatalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWritePersistsCatalogShapeForLiveModels(t *testing.T) {
	evolveDir := t.TempDir()
	fetchedAt := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	want := Catalog{
		FetchedAt: fetchedAt,
		CLIs: map[string]CLIEntry{
			"codex": {
				TierModels: map[string]string{"fast": "gpt-fast", "balanced": "gpt-balanced"},
				Available:  []string{"gpt-fast", "gpt-balanced"},
				Source:     SourceLive,
			},
		},
	}

	if err := Write(evolveDir, want); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	body, err := os.ReadFile(filepath.Join(evolveDir, FileName))
	if err != nil {
		t.Fatalf("reading catalog: %v", err)
	}

	var got Catalog
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("catalog JSON is invalid: %v\n%s", err, string(body))
	}
	if got.FetchedAt.UTC() != fetchedAt {
		t.Fatalf("FetchedAt = %s, want %s", got.FetchedAt.UTC(), fetchedAt)
	}
	if model, ok := got.DispatchModel("codex", "balanced"); !ok || model != "gpt-balanced" {
		t.Fatalf("DispatchModel(codex, balanced) = %q, %v; want gpt-balanced, true", model, ok)
	}
}

func TestWriteSurfacesFilesystemErrors(t *testing.T) {
	fileAsDir := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(fileAsDir, []byte("occupied"), 0o600); err != nil {
		t.Fatalf("setup file: %v", err)
	}

	err := Write(fileAsDir, Catalog{})
	if err == nil {
		t.Fatal("Write() error = nil, want filesystem error for file path used as directory")
	}
}
