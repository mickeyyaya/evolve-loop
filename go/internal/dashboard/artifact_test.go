package dashboard

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestReadArtifact_AllowedRegularFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := core.RunWorkspacePath(root, 5)
	writeFile(t, filepath.Join(ws, "audit-report.md"), "# Audit\n")
	got, err := ReadArtifact(root, 5, "audit-report.md")
	if err != nil || string(got) != "# Audit\n" {
		t.Fatalf("ReadArtifact = %q, %v", got, err)
	}
}

func TestReadArtifact_RejectsTraversalHiddenAndUnknownExtension(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := core.RunWorkspacePath(root, 5)
	writeFile(t, filepath.Join(ws, "ok.md"), "x")
	writeFile(t, filepath.Join(root, "secret.md"), "s")
	for _, name := range []string{"../../secret.md", "..", ".lease", "cycle-state.json.lock", "sub/ok.md", "binary.exe", ""} {
		if _, err := ReadArtifact(root, 5, name); !errors.Is(err, ErrArtifactNotAllowed) {
			t.Errorf("ReadArtifact(%q) err = %v, want ErrArtifactNotAllowed", name, err)
		}
	}
}

func TestReadArtifact_RejectsSymlink(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := core.RunWorkspacePath(root, 5)
	writeFile(t, filepath.Join(root, "outside.md"), "leak")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "outside.md"), filepath.Join(ws, "report.md")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	if _, err := ReadArtifact(root, 5, "report.md"); !errors.Is(err, ErrArtifactNotAllowed) {
		t.Fatalf("symlinked artifact err = %v, want ErrArtifactNotAllowed", err)
	}
}

func TestReadArtifact_SizeCapAndMissing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := core.RunWorkspacePath(root, 5)
	writeFile(t, filepath.Join(ws, "big.log"), strings.Repeat("x", ArtifactMaxBytes+1))
	if _, err := ReadArtifact(root, 5, "big.log"); !errors.Is(err, ErrArtifactTooLarge) {
		t.Fatalf("oversize err = %v, want ErrArtifactTooLarge", err)
	}
	if _, err := ReadArtifact(root, 5, "missing.md"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing err = %v, want ErrNotExist", err)
	}
}

func TestListArtifacts_SortedAllowlistedRegularOnly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := core.RunWorkspacePath(root, 5)
	writeFile(t, filepath.Join(ws, "b.md"), "b")
	writeFile(t, filepath.Join(ws, "a.json"), "{}")
	writeFile(t, filepath.Join(ws, ".lease"), "{}")
	writeFile(t, filepath.Join(ws, "x.json.lock"), "")
	writeFile(t, filepath.Join(ws, "audit-probes", "go.txt"), "")
	list, err := ListArtifacts(root, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Name != "a.json" || list[1].Name != "b.md" || list[1].Size != 1 {
		t.Fatalf("ListArtifacts = %+v", list)
	}
	if _, err := ListArtifacts(root, 6); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing workspace err = %v", err)
	}
}
