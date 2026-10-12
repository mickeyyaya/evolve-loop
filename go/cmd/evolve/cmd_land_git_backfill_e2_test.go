//go:build integration

package main

import (
	"archive/tar"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

type e2LandHub struct {
	runtime string
	seed    *gittest.Repo
}

func newE2LandHub(t *testing.T) e2LandHub {
	t.Helper()
	runtime, seed := devHubFixture(t)
	writeE2File(t, filepath.Join(seed.Dir, "a.txt"), "one\n")
	seed.Git("add", "a.txt")
	seed.Git("commit", "-q", "-m", "a one")
	seed.Git("push", "-q", "origin", "main")
	return e2LandHub{runtime: runtime, seed: seed}
}

func (h e2LandHub) patch(t *testing.T, content string) string {
	t.Helper()
	writeE2File(t, filepath.Join(h.seed.Dir, "a.txt"), content)
	diff := h.seed.Git("diff")
	h.seed.Git("checkout", "--", "a.txt")
	path := filepath.Join(t.TempDir(), "change.patch")
	writeE2File(t, path, diff+"\n")
	return path
}

func (h e2LandHub) devDir(task string) string {
	parent, err := filepath.EvalSymlinks(filepath.Dir(h.runtime))
	if err != nil {
		parent = filepath.Dir(h.runtime)
	}
	return filepath.Join(parent, "dev", task)
}

func TestRunLand_APatchLandsUncommittedOnOriginMain(t *testing.T) {
	h := newE2LandHub(t)
	patch := h.patch(t, "two\n")
	var stdout, stderr bytes.Buffer

	code := runLand([]string{"--branch", "fix/landed", "--patch", patch, "--project-root", h.runtime}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	dev := h.devDir("fix-landed")
	if got, err := os.ReadFile(filepath.Join(dev, "a.txt")); err != nil || string(got) != "two\n" {
		t.Fatalf("a.txt = %q err=%v, want the patched content", got, err)
	}
	if !strings.Contains(stdout.String(), "; nothing was committed or pushed\n") {
		t.Fatalf("stdout=%q", stdout.String())
	}
	if head, origin := brOut(t, dev, "rev-parse", "HEAD"), h.seed.Git("rev-parse", "main"); head != origin {
		t.Fatalf("HEAD %s moved off origin/main %s: land must not commit", head, origin)
	}
}

func TestRunLand_ASalvageLeafLandsItsPatchAndUntrackedFiles(t *testing.T) {
	h := newE2LandHub(t)
	leaf := filepath.Join(h.runtime, ".evolve", "operator-salvage", "cycle-9-leaf")
	if err := os.MkdirAll(leaf, 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(h.patch(t, "salvaged\n"))
	if err != nil {
		t.Fatal(err)
	}
	writeE2File(t, filepath.Join(leaf, "uncommitted.patch"), string(raw))
	archive := e2LandArchive(t, []tar.Header{{Name: "notes/new.md", Typeflag: tar.TypeReg, Mode: 0o644}}, []string{"# new\n"})
	tgz, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	writeE2File(t, filepath.Join(leaf, "untracked.tgz"), string(tgz))
	var stdout, stderr bytes.Buffer

	code := runLand([]string{"--branch", "salvage-9", "--salvage", "cycle-9-leaf", "--project-root", h.runtime}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	dev := h.devDir("salvage-9")
	for rel, want := range map[string]string{"a.txt": "salvaged\n", "notes/new.md": "# new\n"} {
		if got, err := os.ReadFile(filepath.Join(dev, rel)); err != nil || string(got) != want {
			t.Fatalf("%s = %q err=%v, want %q", rel, got, err, want)
		}
	}
}

func TestRunLand_AConflictIsRefusedButTheUntrackedFilesStillLand(t *testing.T) {
	h := newE2LandHub(t)
	leaf := filepath.Join(h.runtime, ".evolve", "operator-salvage", "conflict-leaf")
	if err := os.MkdirAll(leaf, 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(h.patch(t, "mine\n"))
	if err != nil {
		t.Fatal(err)
	}
	writeE2File(t, filepath.Join(leaf, "uncommitted.patch"), string(raw))
	archive := e2LandArchive(t, []tar.Header{{Name: "kept.txt", Typeflag: tar.TypeReg, Mode: 0o644}}, []string{"kept\n"})
	tgz, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	writeE2File(t, filepath.Join(leaf, "untracked.tgz"), string(tgz))
	writeE2File(t, filepath.Join(h.seed.Dir, "a.txt"), "theirs\n")
	h.seed.Git("commit", "-q", "-am", "origin moves a.txt")
	h.seed.Git("push", "-q", "origin", "main")
	var stdout, stderr bytes.Buffer

	code := runLand([]string{"--branch", "conflict", "--salvage", "conflict-leaf", "--project-root", h.runtime}, nil, &stdout, &stderr)

	dev := h.devDir("conflict")
	if code != exitRefused || !strings.Contains(stderr.String(), "refused: the patch conflicts with origin/main in a.txt; resolve the conflict markers in "+dev) {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if got, err := os.ReadFile(filepath.Join(dev, "kept.txt")); err != nil || string(got) != "kept\n" {
		t.Fatalf("the untracked files must land beside a refused patch: %q err=%v", got, err)
	}
	if strings.Contains(stdout.String(), "land: landed") {
		t.Fatalf("a refused land reported success: %q", stdout.String())
	}
}

func TestRunLand_AHubThatVanishesAfterTheWorktreeIsAnIOError(t *testing.T) {
	h := newE2LandHub(t)
	patch := h.patch(t, "two\n")
	previous := landDevHub
	landDevHub = func(string) (devHub, error) { return devHub{}, errors.New("hub gone") }
	t.Cleanup(func() { landDevHub = previous })
	var stdout, stderr bytes.Buffer

	code := runLand([]string{"--branch", "gone", "--patch", patch, "--project-root", h.runtime}, nil, &stdout, &stderr)

	if code != exitIO || stderr.String() != landPrefix+"hub gone\n" {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}

func TestApplyLandPatch_RefusesAPatchWithNoBase(t *testing.T) {
	h := newE2LandHub(t)
	if code := runWorktreeCreateDev(h.runtime, "nobase", "nobase", &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("create dev: %d", code)
	}
	patch := filepath.Join(t.TempDir(), "nobase.patch")
	writeE2File(t, patch, "diff --git a/ghost.txt b/ghost.txt\nindex 1111111..2222222 100644\n--- a/ghost.txt\n+++ b/ghost.txt\n@@ -1 +1 @@\n-boo\n+BOO\n")
	var stderr bytes.Buffer

	code := applyLandPatch(h.devDir("nobase"), patch, &stderr)

	if code != exitRefused || !strings.Contains(stderr.String(), "refused: the patch does not apply to origin/main (worktree "+h.devDir("nobase")+" left for inspection): ") {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}

func TestApplyLandPatch_OutsideARepoTheFailedDiffIsAnIOError(t *testing.T) {
	dir := t.TempDir()
	patch := filepath.Join(t.TempDir(), "x.patch")
	writeE2File(t, patch, "diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b\n")
	var stderr bytes.Buffer

	code := applyLandPatch(dir, patch, &stderr)

	if code != exitIO || !strings.HasPrefix(stderr.String(), landPrefix+"git apply --3way rc=") {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}

func TestApplyLandPatch_AMissingWorktreeIsAnIOError(t *testing.T) {
	patch := filepath.Join(t.TempDir(), "x.patch")
	writeE2File(t, patch, "not empty")
	var stderr bytes.Buffer

	code := applyLandPatch(filepath.Join(t.TempDir(), "absent"), patch, &stderr)

	if code != exitIO || !strings.HasPrefix(stderr.String(), landPrefix+"git apply --3way: ") {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}

func TestLandInto_AnIOErrorFromThePatchSkipsTheArchive(t *testing.T) {
	dir := t.TempDir()
	patch := filepath.Join(t.TempDir(), "x.patch")
	writeE2File(t, patch, "diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b\n")
	archive := e2LandArchive(t, []tar.Header{{Name: "skipped.txt", Typeflag: tar.TypeReg, Mode: 0o644}}, []string{"s\n"})
	var stdout, stderr bytes.Buffer

	code := landInto(dir, landInput{patch: patch, untracked: archive}, &stdout, &stderr)

	if code != exitIO || stdout.Len() != 0 {
		t.Fatalf("code=%d stdout=%q", code, stdout.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "skipped.txt")); !os.IsNotExist(err) {
		t.Fatalf("an IO failure must stop before the archive: %v", err)
	}
}
