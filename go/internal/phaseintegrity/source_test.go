package phaseintegrity

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseblock"
	"github.com/mickeyyaya/evolve-loop/go/internal/testhost"
)

func TestSource_TreeSHA_RealGitWorktree(t *testing.T) {
	testhost.Tool(t, "git")
	dir := t.TempDir()
	run := func(args ...string) []byte {
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
		if err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
		return out
	}
	run("init")
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	got, err := (Source{WorktreePath: dir}).TreeSHA()
	if err != nil {
		t.Fatalf("TreeSHA real git: %v", err)
	}
	listing := string(run("ls-tree", got))
	blob := strings.TrimSpace(string(run("hash-object", "f.txt")))
	if !strings.Contains(listing, blob) || !strings.Contains(listing, "f.txt") {
		t.Errorf("tree %s lists %q, want f.txt at blob %s", got, listing, blob)
	}
}

func TestSource_TreeSHA_DefaultErrorsOnNonGitDir(t *testing.T) {
	if _, err := (Source{WorktreePath: t.TempDir()}).TreeSHA(); err == nil {
		t.Fatal("expected git write-tree to error in a non-git directory")
	}
}

func TestSource_ProfileSHA_SurfacesNonENOENTStatError(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "afile")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (Source{ProfilePath: filepath.Join(f, "child")}).ProfileSHA(); err == nil {
		t.Fatal("expected a non-ENOENT stat error to surface (fail loudly)")
	}
}

func TestSource_ImplementsDigestSource(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.WriteFile(bin, []byte("binbytes"), 0o755); err != nil {
		t.Fatal(err)
	}
	var src phaseblock.DigestSource = Source{BinaryPath: bin, WorktreePath: dir, GitTree: func(string) (string, error) { return "tree-oid", nil }}
	d, err := phaseblock.Compute("audit", "run-1", "now", "", src)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("binbytes"))
	if d.BinarySHA != hex.EncodeToString(sum[:]) || d.TreeSHA != "tree-oid" {
		t.Errorf("digest through the interface = %+v, want the bytes of %s and tree-oid", d, bin)
	}
}

func TestSource_BinarySHA_FromPathOrRunning(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bin")
	if err := os.WriteFile(p, []byte("binbytes"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Source{BinaryPath: p}.BinarySHA()
	if err != nil || len(got) != 64 {
		t.Fatalf("BinarySHA from path = %q err=%v", got, err)
	}
	got2, err := Source{}.BinarySHA()
	if err != nil || len(got2) != 64 {
		t.Fatalf("BinarySHA running = %q err=%v", got2, err)
	}
}

func TestSource_ProfileAndReportSHA_BestEffort(t *testing.T) {
	dir := t.TempDir()
	prof := filepath.Join(dir, "profile.json")
	if err := os.WriteFile(prof, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Source{ProfilePath: prof}.ProfileSHA()
	if err != nil || got == "" {
		t.Fatalf("ProfileSHA present = %q err=%v", got, err)
	}
	if got, err := (Source{}).ReportSHA(); err != nil || got != "" {
		t.Fatalf("ReportSHA absent path = %q err=%v", got, err)
	}
	if got, err := (Source{ReportPath: filepath.Join(dir, "missing.md")}).ReportSHA(); err != nil || got != "" {
		t.Fatalf("ReportSHA missing file = %q err=%v", got, err)
	}
}

func TestSource_TreeSHA_SeamAndEmpty(t *testing.T) {
	if got, err := (Source{}).TreeSHA(); err != nil || got != "" {
		t.Fatalf("TreeSHA empty worktree = %q err=%v", got, err)
	}
	called := ""
	s := Source{WorktreePath: "/wt", GitTree: func(wt string) (string, error) { called = wt; return "treesha", nil }}
	got, err := s.TreeSHA()
	if err != nil || got != "treesha" || called != "/wt" {
		t.Fatalf("TreeSHA seam: got=%q called=%q err=%v", got, called, err)
	}
}

func TestSource_TreeSHA_ErrorPropagates(t *testing.T) {
	s := Source{WorktreePath: "/wt", GitTree: func(string) (string, error) { return "", errors.New("git boom") }}
	if _, err := s.TreeSHA(); err == nil {
		t.Fatal("expected the git error to propagate")
	}
}

func TestSource_TreeSHA_DefaultRejectsRelativePath(t *testing.T) {
	if _, err := (Source{WorktreePath: "relative/wt"}).TreeSHA(); err == nil {
		t.Fatal("expected an error for a non-absolute worktree path")
	}
}

func TestSource_Compute_EndToEnd(t *testing.T) {
	dir := t.TempDir()
	prof := filepath.Join(dir, "p.json")
	if err := os.WriteFile(prof, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := Source{ProfilePath: prof, WorktreePath: "/wt", GitTree: func(string) (string, error) { return "tree", nil }}
	d, err := phaseblock.Compute("build", "run", "ts", "", s)
	if err != nil {
		t.Fatal(err)
	}
	if d.Combined == "" || d.ProfileSHA == "" || d.TreeSHA != "tree" {
		t.Fatalf("end-to-end digest: %+v", d)
	}
}

func TestSource_ConcurrentSafe(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	prof := filepath.Join(dir, "p.json")
	if err := os.WriteFile(prof, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := Source{ProfilePath: prof, WorktreePath: "/wt", GitTree: func(string) (string, error) { return "tree", nil }}
	golden, err := phaseblock.Compute("build", "run", "ts", "", s)
	if err != nil {
		t.Fatal(err)
	}
	const n = 32
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			d, err := phaseblock.Compute("build", "run", "ts", "", s)
			if err != nil || d.Combined != golden.Combined {
				t.Errorf("concurrent compute mismatch: err=%v combined=%q", err, d.Combined)
			}
		}()
	}
	wg.Wait()
}
