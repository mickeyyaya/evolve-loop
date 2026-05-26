package bridge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// driver_artifact_relocate_test.go — artifactReady tolerance for the
// cycle-108 ExitArtifactTimeout root cause: agents intermittently wrote the
// report to <workspace>/workspace/<file> (reading the doc's "workspace/"
// prefix as a literal subdir) while the driver polls only the canonical
// <workspace>/<file>. artifactReady accepts either location and relocates the
// non-canonical write so downstream phases — which read the canonical path —
// still resolve it. See docs/architecture/adr/0024-*.md (Step 0).

func TestArtifactReady_CanonicalPresent(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "scout-report.md")
	if err := os.WriteFile(canonical, []byte("## Proposed Tasks\n"), 0o644); err != nil {
		t.Fatalf("seed canonical: %v", err)
	}
	cfg := &Config{Workspace: ws, Artifact: canonical}

	ready, from, err := artifactReady(cfg)
	if !ready {
		t.Fatal("artifactReady should be true when the canonical artifact is present")
	}
	if from != "" || err != nil {
		t.Fatalf("no relocation/error expected when canonical present; got from=%q err=%v", from, err)
	}
}

func TestArtifactReady_RelocatesFromWorkspaceSubdir(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "scout-report.md")
	subdir := filepath.Join(ws, "workspace", "scout-report.md")
	if err := os.MkdirAll(filepath.Dir(subdir), 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}
	body := []byte("## Proposed Tasks\n1. do the thing\n")
	if err := os.WriteFile(subdir, body, 0o644); err != nil {
		t.Fatalf("seed subdir: %v", err)
	}
	cfg := &Config{Workspace: ws, Artifact: canonical}

	ready, from, err := artifactReady(cfg)
	if !ready || err != nil {
		t.Fatalf("artifactReady should succeed when the artifact is in the workspace/ subdir; got ready=%v err=%v", ready, err)
	}
	if from != subdir {
		t.Fatalf("relocatedFrom = %q, want %q", from, subdir)
	}
	// The content must now live at the canonical path the driver/runner read.
	got, err := os.ReadFile(canonical)
	if err != nil {
		t.Fatalf("canonical not present after relocation: %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("relocated content = %q, want %q", got, body)
	}
	// The non-canonical copy must be gone (single source of truth).
	if _, statErr := os.Stat(subdir); !os.IsNotExist(statErr) {
		t.Fatalf("subdir copy should be removed after relocation; stat err=%v", statErr)
	}
	// No temp file may linger in the workspace after a successful relocate.
	assertNoTempArtifacts(t, ws)
}

func TestArtifactReady_RelocatesOverEmptyCanonical(t *testing.T) {
	// Agent created an empty canonical placeholder but wrote the real content
	// to the workspace/ subdir. The empty canonical must not count as ready;
	// the relocation must overwrite it with the real content.
	ws := t.TempDir()
	canonical := filepath.Join(ws, "scout-report.md")
	subdir := filepath.Join(ws, "workspace", "scout-report.md")
	if err := os.WriteFile(canonical, nil, 0o644); err != nil {
		t.Fatalf("seed empty canonical: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(subdir), 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}
	body := []byte("## Proposed Tasks\n1. real content\n")
	if err := os.WriteFile(subdir, body, 0o644); err != nil {
		t.Fatalf("seed subdir: %v", err)
	}
	cfg := &Config{Workspace: ws, Artifact: canonical}

	ready, from, err := artifactReady(cfg)
	if !ready || err != nil || from != subdir {
		t.Fatalf("expected relocation over empty canonical; got ready=%v from=%q err=%v", ready, from, err)
	}
	got, err := os.ReadFile(canonical)
	if err != nil || string(got) != string(body) {
		t.Fatalf("canonical should hold the relocated body; got %q err=%v", got, err)
	}
}

func TestArtifactReady_NeitherPresent(t *testing.T) {
	ws := t.TempDir()
	cfg := &Config{Workspace: ws, Artifact: filepath.Join(ws, "scout-report.md")}

	ready, from, err := artifactReady(cfg)
	if ready || from != "" || err != nil {
		t.Fatalf("artifactReady should be (false, \"\", nil) when no artifact exists; got (%v, %q, %v)", ready, from, err)
	}
}

func TestArtifactReady_EmptyFilesDoNotCount(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "scout-report.md")
	subdir := filepath.Join(ws, "workspace", "scout-report.md")
	if err := os.WriteFile(canonical, nil, 0o644); err != nil {
		t.Fatalf("seed empty canonical: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(subdir), 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}
	if err := os.WriteFile(subdir, nil, 0o644); err != nil {
		t.Fatalf("seed empty subdir: %v", err)
	}
	cfg := &Config{Workspace: ws, Artifact: canonical}

	ready, from, err := artifactReady(cfg)
	if ready || from != "" || err != nil {
		t.Fatalf("empty artifacts must not count as ready; got (%v, %q, %v)", ready, from, err)
	}
}

func TestArtifactReady_RelocationFailureSurfacesError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("read-only directory permissions are not enforced for root")
	}
	// Fallback artifact exists but the workspace dir is read-only, so the
	// relocation cannot write the canonical path. The error must surface
	// (not be swallowed into a silent (false, "")), and no canonical file
	// may be created.
	ws := t.TempDir()
	canonical := filepath.Join(ws, "scout-report.md")
	subdir := filepath.Join(ws, "workspace", "scout-report.md")
	if err := os.MkdirAll(filepath.Dir(subdir), 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}
	if err := os.WriteFile(subdir, []byte("## Proposed Tasks\n1. x\n"), 0o644); err != nil {
		t.Fatalf("seed subdir: %v", err)
	}
	if err := os.Chmod(ws, 0o555); err != nil {
		t.Fatalf("chmod ws read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(ws, 0o755) }) // let t.TempDir clean up

	cfg := &Config{Workspace: ws, Artifact: canonical}
	ready, _, err := artifactReady(cfg)
	if ready {
		t.Fatal("artifactReady must not report ready when relocation fails")
	}
	if err == nil {
		t.Fatal("relocation failure must surface as a non-nil error, not a silent (false, \"\")")
	}
	if _, statErr := os.Stat(canonical); !os.IsNotExist(statErr) {
		t.Fatalf("no canonical file should exist after a failed relocation; stat err=%v", statErr)
	}
}

func TestRelocateFile_HappyPath(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "sub", "report.md")
	dst := filepath.Join(dir, "report.md")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	body := []byte("content\n")
	if err := os.WriteFile(src, body, 0o644); err != nil {
		t.Fatalf("seed src: %v", err)
	}

	if err := relocateFile(src, dst); err != nil {
		t.Fatalf("relocateFile: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil || string(got) != string(body) {
		t.Fatalf("dst content = %q err=%v, want %q", got, err, body)
	}
	if _, statErr := os.Stat(src); !os.IsNotExist(statErr) {
		t.Fatalf("src should be gone after relocate; stat err=%v", statErr)
	}
	assertNoTempArtifacts(t, dir)
}

func TestListWorkspaceFiles_PrunesDepthIncludesArtifactPaths(t *testing.T) {
	ws := t.TempDir()
	// depth 0 (canonical) and depth 1 (non-canonical workspace/) are the two
	// paths the diagnostic must always show; depth 2 is still listed; a file
	// whose parent dir is at depth >= wsListMaxDepth must be pruned.
	seed := map[string][]byte{
		filepath.Join(ws, "scout-report.md"):                       []byte("a"),
		filepath.Join(ws, "workspace", "scout-report.md"):          []byte("b"),
		filepath.Join(ws, "workspace", "sub", "c.md"):              []byte("c"),
		filepath.Join(ws, "workspace", "sub", "deep", "pruned.md"): []byte("d"),
	}
	for p, body := range seed {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", p, err)
		}
		if err := os.WriteFile(p, body, 0o644); err != nil {
			t.Fatalf("seed %s: %v", p, err)
		}
	}
	joined := strings.Join(listWorkspaceFiles(ws), "\n")
	for _, want := range []string{"scout-report.md", filepath.Join("workspace", "scout-report.md"), filepath.Join("workspace", "sub", "c.md")} {
		if !strings.Contains(joined, want) {
			t.Errorf("listing should include %q; got:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "pruned.md") {
		t.Errorf("file under a dir at depth >= %d should be pruned; got:\n%s", wsListMaxDepth, joined)
	}
}

func TestListWorkspaceFiles_CapsEntries(t *testing.T) {
	ws := t.TempDir()
	for i := 0; i < wsListMaxEntries+5; i++ {
		if err := os.WriteFile(filepath.Join(ws, fmt.Sprintf("f%03d.md", i)), []byte("x"), 0o644); err != nil {
			t.Fatalf("seed f%03d: %v", i, err)
		}
	}
	got := listWorkspaceFiles(ws)
	if len(got) != wsListMaxEntries+1 { // capped entries + one truncation marker
		t.Fatalf("expected %d lines (cap + marker), got %d", wsListMaxEntries+1, len(got))
	}
	if !strings.Contains(got[len(got)-1], "truncated") {
		t.Errorf("last line should be the truncation marker; got %q", got[len(got)-1])
	}
}

// assertNoTempArtifacts fails if any ".tmp" temp file from the atomic-write
// path is left behind under root.
func assertNoTempArtifacts(t *testing.T, root string) {
	t.Helper()
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.Contains(filepath.Base(path), ".tmp") {
			t.Errorf("leftover temp artifact: %s", path)
		}
		return nil
	})
}
