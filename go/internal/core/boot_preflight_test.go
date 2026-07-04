package core

// boot_preflight_test.go — RED tests (cycle 507, task
// wire-boot-recovery-functions) for the loop's boot-time recovery from a dirty
// main tree. Function-level behavior contract for the two recovery primitives
// the Builder (re)implements in boot_preflight.go.
//
// Root cause (scout-report.md cycle 506/507 Key Finding 1/3): when a leak
// escapes into the main tree (from any source), EVERY subsequent cycle's
// tree-diff guard FAILs, attributing the pre-existing dirt to whichever phase
// runs first, wedging the loop until a human `git stash`es. The boot path must
// self-heal: quarantine tracked-source dirt (non-destructively, via stash)
// BEFORE the first cycle dispatches, while leaving the loop's own managed dirs
// (.evolve/, knowledge-base/) untouched, and surface a ship-binary SHA mismatch
// at boot rather than only when the ship phase fails (the 498/500/502 cascade).
//
// Cycle 506 built these functions but NEVER WIRED them (audit F1, CRITICAL);
// the cycle was reset so they no longer exist on disk. This cycle re-establishes
// the function contract HERE and the wiring contract in
// cmd/evolve/cmd_loop_boot_recovery_test.go (the piece 506 lacked).
//
// References classifyDirtyPaths / QuarantineDirtyTree / ShipSHAMismatch, which
// the Builder implements. RED now (undefined symbols → core test package fails
// to compile). Do NOT modify this file — implement the production seam.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// AC1 (positive): a dirty tracked-source path is classified for quarantine.
func TestClassifyDirtyPaths_QuarantinesTrackedSource(t *testing.T) {
	quarantine, _ := classifyDirtyPaths([]string{"go/internal/triagecap/project.go"})
	if len(quarantine) != 1 || quarantine[0] != "go/internal/triagecap/project.go" {
		t.Fatalf("tracked-source dirt must be quarantined; got %v", quarantine)
	}
}

// AC2 (negative / exclusion): the loop's own managed dirs must be EXCLUDED from
// the dirty-scan so a normal in-flight cycle's workspace/knowledge-base writes
// never trigger a false-positive quarantine of its own state.
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

// AC2b (regression, cycle 514): the ship binary go/bin/evolve is verified and
// re-pinned by the SAME boot-recovery pass, so quarantine must NEVER stash it —
// stashing a rebuilt binary would revert on-disk to the old committed one and
// re-open the SELF_SHA mismatch the auto-repin just healed (the 508-513 cascade).
func TestClassifyDirtyPaths_ExcludesShipBinary(t *testing.T) {
	quarantine, ignored := classifyDirtyPaths([]string{"go/bin/evolve"})
	if len(quarantine) != 0 {
		t.Fatalf("the ship binary go/bin/evolve must NOT be quarantined (boot re-pins it in the same pass); got %v", quarantine)
	}
	if len(ignored) != 1 || ignored[0] != "go/bin/evolve" {
		t.Errorf("go/bin/evolve must be reported as ignored/loop-managed; got %v", ignored)
	}
}

// AC3 (clean case): a clean tree yields no quarantine action and no noise — the
// preflight must not misfire on the common case.
func TestClassifyDirtyPaths_CleanTreeNoAction(t *testing.T) {
	quarantine, ignored := classifyDirtyPaths(nil)
	if len(quarantine) != 0 || len(ignored) != 0 {
		t.Fatalf("clean tree must produce no quarantine and no ignored entries; got quarantine=%v ignored=%v", quarantine, ignored)
	}
}

// AC1 gaming-check: QuarantineDirtyTree must ACTUALLY leave `git status
// --porcelain` clean for tracked source afterward — a log line alone is a fake.
// It must also be NON-DESTRUCTIVE (stash, not checkout): popping the stash
// restores the content.
func TestQuarantineDirtyTree_LeavesStatusCleanAndPreservesContent(t *testing.T) {
	repo := initTempGitRepo(t)
	src := filepath.Join(repo, "go", "internal", "foo.go")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFileT(t, src, "package foo\n")
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-m", "seed")
	// Now introduce uncommitted tracked-source dirt (the leaked write).
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
	// Non-destructive: the leaked content is preserved in the stash, recoverable.
	runGit(t, repo, "stash", "pop")
	if got := readFileT(t, src); got != "package foo\n// leaked edit\n" {
		t.Fatalf("quarantine must PRESERVE content (stash, not checkout); recovered %q", got)
	}
}

// AC4 (edge): a ship-binary SHA mismatch vs state.json:expected_ship_sha is
// detected at boot (the SELF_SHA_TAMPERED cascade), and a matching SHA is not a
// false positive.
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

// --- test helpers (git fixtures) ---

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
