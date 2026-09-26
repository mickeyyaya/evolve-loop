package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestClassifyDirtyPaths_QuarantinesTrackedSource(t *testing.T) {
	quarantine, _ := classifyDirtyPaths([]string{"go/internal/triagecap/project.go"})
	if len(quarantine) != 1 || quarantine[0] != "go/internal/triagecap/project.go" {
		t.Fatalf("tracked-source dirt must be quarantined; got %v", quarantine)
	}
}

func TestClassifyDirtyPaths_IgnoresLoopManaged(t *testing.T) {
	quarantine, ignored := classifyDirtyPaths([]string{
		".evolve/runs/cycle-507/scout-report.md",
		"knowledge-base/cycles/cycle-507.json",
	})
	if len(quarantine) != 0 {
		t.Fatalf("loop-managed paths (.evolve/, knowledge-base/) must NOT be quarantined; got %v", quarantine)
	}
	if len(ignored) != 2 {
		t.Errorf("both loop-managed paths should be reported as ignored; got %v", ignored)
	}
}

func TestClassifyDirtyPaths_ExcludesShipBinary(t *testing.T) {
	quarantine, ignored := classifyDirtyPaths([]string{"go/bin/evolve"})
	if len(quarantine) != 0 {
		t.Fatalf("the ship binary go/bin/evolve must NOT be quarantined (boot re-pins it in the same pass); got %v", quarantine)
	}
	if len(ignored) != 1 || ignored[0] != "go/bin/evolve" {
		t.Errorf("go/bin/evolve must be reported as ignored/loop-managed; got %v", ignored)
	}
}

func TestClassifyDirtyPaths_CleanTreeNoAction(t *testing.T) {
	quarantine, ignored := classifyDirtyPaths(nil)
	if len(quarantine) != 0 || len(ignored) != 0 {
		t.Fatalf("clean tree must produce no quarantine and no ignored entries; got quarantine=%v ignored=%v", quarantine, ignored)
	}
}

func TestQuarantineDirtyTree_LeavesStatusCleanAndPreservesContent(t *testing.T) {
	repo := initTempGitRepo(t)
	src := filepath.Join(repo, "go", "internal", "foo.go")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFileT(t, src, "package foo\n")
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-m", "seed")
	writeFileT(t, src, "package foo\n// leaked edit\n")

	stashed, err := QuarantineDirtyTree(context.Background(), repo, "boot-quarantine-cycle-507")
	if err != nil {
		t.Fatalf("QuarantineDirtyTree: %v", err)
	}
	if !stashed {
		t.Fatal("dirty tracked-source tree must report stashed=true")
	}
	if out := porcelain(t, repo); out != "" {
		t.Fatalf("tracked source must be clean after quarantine; git status --porcelain = %q", out)
	}
	runGit(t, repo, "stash", "pop")
	if got := readFileT(t, src); got != "package foo\n// leaked edit\n" {
		t.Fatalf("quarantine must PRESERVE content (stash, not checkout); recovered %q", got)
	}
}

func TestShipSHAMismatch_DetectsTamperNotFalsePositive(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "evolve")
	writeFileT(t, bin, "\x7fELF-fake-binary-bytes")
	sum := sha256.Sum256([]byte("\x7fELF-fake-binary-bytes"))
	correct := hex.EncodeToString(sum[:])

	mismatch, actual, err := ShipSHAMismatch(bin, "0000deadbeef")
	if err != nil {
		t.Fatalf("ShipSHAMismatch: %v", err)
	}
	if !mismatch {
		t.Error("a tampered ship binary (SHA != expected) must be flagged at boot")
	}
	if actual != correct {
		t.Errorf("must report the actual on-disk SHA; want %s got %s", correct, actual)
	}
	if m, _, _ := ShipSHAMismatch(bin, correct); m {
		t.Error("a matching SHA must NOT be flagged (no false positive on a clean binary)")
	}
}

func initTempGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.email", "ci@example.com")
	runGit(t, dir, "config", "user.name", "ci")
	runGit(t, dir, "config", "commit.gpgsign", "false")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func porcelain(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git status: %v\n%s", err, out)
	}
	return string(out)
}

func writeFileT(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFileT(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
