package phaseorder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestList_DisabledReturnsHardcoded(t *testing.T) {
	got, err := List("/nonexistent.json", false)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(HardcodedOrder) {
		t.Errorf("len=%d want %d", len(got), len(HardcodedOrder))
	}
	if got[0] != "intent" {
		t.Errorf("first=%q want intent", got[0])
	}
}

func TestList_MissingFileReturnsHardcoded(t *testing.T) {
	got, err := List(filepath.Join(t.TempDir(), "no-such.json"), true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(HardcodedOrder) {
		t.Errorf("len=%d want %d", len(got), len(HardcodedOrder))
	}
}

func TestList_ValidRegistry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "phase-registry.json")
	body := `{"phases":[{"name":"intent"},{"name":"scout"},{"name":"build"}]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := List(path, true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{"intent", "scout", "build"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("got=%v want=%v", got, want)
	}
}

func TestList_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "phase-registry.json")
	if err := os.WriteFile(path, []byte("not-json{"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := List(path, true); err == nil {
		t.Errorf("expected parse error")
	}
}

func TestList_EmptyPhasesArray(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "phase-registry.json")
	if err := os.WriteFile(path, []byte(`{"phases":[]}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := List(path, true); err == nil {
		t.Errorf("expected empty-array error")
	}
}

func TestList_FilterEmptyNames(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "phase-registry.json")
	body := `{"phases":[{"name":"intent"},{"name":""},{"name":"build"}]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := List(path, true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 (empty name dropped), got %v", got)
	}
}
