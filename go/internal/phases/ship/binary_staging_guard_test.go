package ship

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeStagedFilesRun scripts `git diff --cached --name-only` to return a
// fixed staged-file list; every other subcommand returns rc=0, empty stdout,
// since stageBinaryGuard only ever shells that one query.
type fakeStagedFilesRun struct {
	staged []string
}

func (f *fakeStagedFilesRun) run(_ context.Context, _, _ string, args, _ []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
	if len(args) >= 2 && args[0] == "diff" && args[1] == "--cached" {
		if stdout != nil {
			_, _ = stdout.Write([]byte(strings.Join(f.staged, "\n") + "\n"))
		}
		return 0, nil
	}
	return 0, nil
}

func writeStagedFile(t *testing.T, root, rel string, size int, mode os.FileMode) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, make([]byte, size), mode); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestBinaryStagingGuard_RejectsLargeExecutableOutsideAllowlist(t *testing.T) {
	root := t.TempDir()
	const rel = "go/acs/cycle536/evolve"
	writeStagedFile(t, root, rel, 2*1024*1024, 0o755) // 2MB, executable bit set

	opts := &Options{ProjectRoot: root, Runner: (&fakeStagedFilesRun{staged: []string{rel}}).run}

	err := stageBinaryGuard(context.Background(), opts)
	if err == nil {
		t.Fatal("stageBinaryGuard: want error for a staged >1MB executable outside go/bin//go/evolve, got nil")
	}
	if !strings.Contains(err.Error(), rel) {
		t.Errorf("stageBinaryGuard error must name the offending path %q, got: %v", rel, err)
	}
}

func TestBinaryStagingGuard_AllowsAllowlistedAndSmallFiles(t *testing.T) {
	root := t.TempDir()
	writeStagedFile(t, root, "go/bin/evolve", 2*1024*1024, 0o755)
	writeStagedFile(t, root, "go/evolve", 2*1024*1024, 0o755)
	writeStagedFile(t, root, "internal/phases/ship/gitops.go", 200, 0o644)

	opts := &Options{ProjectRoot: root, Runner: (&fakeStagedFilesRun{
		staged: []string{"go/bin/evolve", "go/evolve", "internal/phases/ship/gitops.go"},
	}).run}

	if err := stageBinaryGuard(context.Background(), opts); err != nil {
		t.Fatalf("stageBinaryGuard: want nil for allowlisted/small paths, got: %v", err)
	}
}

func TestBinaryStagingGuard_AllowsSmallExecutableOutsideAllowlist(t *testing.T) {
	root := t.TempDir()
	const rel = "scripts/tiny-helper"
	writeStagedFile(t, root, rel, 512, 0o755)

	opts := &Options{ProjectRoot: root, Runner: (&fakeStagedFilesRun{staged: []string{rel}}).run}

	if err := stageBinaryGuard(context.Background(), opts); err != nil {
		t.Fatalf("stageBinaryGuard: want nil for a small (<1MB) executable, got: %v", err)
	}
}
