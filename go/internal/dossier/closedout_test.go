package dossier

import (
	"os"
	"path/filepath"
	"testing"
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
