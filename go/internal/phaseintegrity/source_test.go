package phaseintegrity

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseblock"
)

func TestSource_TreeSHA_RealGitWorktree(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) error {
		return exec.Command("git", append([]string{"-C", dir}, args...)...).Run()
	}
	if err := run("init"); err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run("add", "-A"); err != nil {
		t.Fatalf("git add: %v", err)
	}
	got, err := (Source{WorktreePath: dir}).TreeSHA() // GitTree nil → defaultGitTree
	if err != nil {
		t.Fatalf("TreeSHA real git: %v", err)
	}
	if len(got) != 40 && len(got) != 64 { // sha1 or sha256 tree oid
		t.Errorf("write-tree sha = %q, want a 40/64-char hex oid", got)
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
	// A child path UNDER a regular file → ENOTDIR (not IsNotExist) → must surface.
	if _, err := (Source{ProfilePath: filepath.Join(f, "child")}).ProfileSHA(); err == nil {
		t.Fatal("expected a non-ENOENT stat error to surface (fail loudly)")
	}
}

func TestSource_ImplementsDigestSource(t *testing.T) {
	var _ phaseblock.DigestSource = Source{}
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
	// empty BinaryPath → the running executable.
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
	// absent report path → "" (no error).
	if got, err := (Source{}).ReportSHA(); err != nil || got != "" {
		t.Fatalf("ReportSHA absent path = %q err=%v", got, err)
	}
	// set-but-missing file → "" (best-effort, never blocks the cycle).
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
	// GitTree nil → defaultGitTree → a non-absolute worktree (which could be
	// read as a git flag) must be rejected, not executed.
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

// Source is a read-only value; concurrent digest computation from a shared
// Source must be race-free and deterministic (capture happens from each phase's
// goroutine). Run with -race.
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
