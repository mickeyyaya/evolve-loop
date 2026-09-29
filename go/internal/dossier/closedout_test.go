package dossier

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestClosedOut_APublishedOrPendingDossierClosesTheCycle(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, p := range []string{filepath.Join(CyclesDir(root), "cycle-1757.json"), filepath.Join(PendingDir(root), "cycle-1761.json")} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if !ClosedOut(root, 1757) || !ClosedOut(root, 1761) {
		t.Error("a published or a pending dossier means the cycle closed out")
	}
	if ClosedOut(root, 1762) {
		t.Error("a cycle without a dossier (a live lane) is not closed out")
	}
}

func TestClosedOutAt_ReportsWhenTheDossierLanded(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	p := filepath.Join(PendingDir(root), "cycle-1761.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	landed := time.Date(2026, 9, 28, 9, 30, 0, 0, time.UTC)
	if err := os.Chtimes(p, landed, landed); err != nil {
		t.Fatal(err)
	}

	at, ok := ClosedOutAt(root, 1761)
	if !ok || !at.Equal(landed) {
		t.Errorf("ClosedOutAt = %v, %v; want %v, true", at, ok, landed)
	}
	if _, ok := ClosedOutAt(root, 1762); ok {
		t.Error("a cycle without a dossier has no closeout time")
	}
}
