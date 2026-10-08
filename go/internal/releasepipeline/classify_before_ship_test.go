package releasepipeline

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func writeRepoFile(t *testing.T, repoDir, rel, content string) {
	t.Helper()
	path := filepath.Join(repoDir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func configOnlyReleaseRepo(t *testing.T) *gittest.Repo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	repo := gittest.Fixture(t)
	writeRepoFile(t, repo.Dir, "go/main.go", "package main\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "binary change")
	repo.Git("tag", "v1.2.2")
	writeRepoFile(t, repo.Dir, "CHANGELOG.md", "## [1.2.3]\n\n- docs only\n\n## [1.2.2]\n\n- binary change\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "docs only")
	return repo
}

func TestRun_ReleaseClassComputedBeforeShipCommits(t *testing.T) {
	repo := configOnlyReleaseRepo(t)
	steps := allOkSteps()
	steps.RebuildBinary = func(root, _ string, _ bool) error {
		writeRepoFile(t, root, "go/evolve", "rebuilt binary\n")
		return nil
	}
	var notesGivenToShip []string
	steps.Ship = func(_, msg, notes string) (string, error) {
		notesGivenToShip = append(notesGivenToShip, notes)
		repo.Git("add", "-A")
		repo.Git("commit", "-q", "-m", msg)
		return repo.Git("rev-parse", "HEAD"), nil
	}

	_, err := Run(Options{
		Target:      "1.2.3",
		RepoRoot:    repo.Dir,
		FromTag:     "v1.2.2",
		JournalDir:  t.TempDir(),
		MaxPollWait: time.Second,
		Now:         fixedNow(t),
		Steps:       steps,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(notesGivenToShip) != 1 {
		t.Fatalf("Ship called %d times, want 1", len(notesGivenToShip))
	}
	configBanner := bannerFor(ConfigRelease, "v1.2.2")
	if !strings.HasPrefix(notesGivenToShip[0], configBanner+"\n\n") {
		t.Errorf("Ship got notes classified after its own commit; want them to open with the pre-ship class %q, got:\n%s", configBanner, notesGivenToShip[0])
	}
	if !strings.Contains(notesGivenToShip[0], "- docs only") {
		t.Errorf("Ship notes lost the changelog body:\n%s", notesGivenToShip[0])
	}
	postShipBanner, err := releaseClassBanner(repo.Dir, "1.2.3", "v1.2.2")
	if err != nil {
		t.Fatalf("classify the post-ship tree: %v", err)
	}
	if postShipBanner != bannerFor(BinaryRelease, "1.2.3") {
		t.Fatalf("the fixture cannot tell the orders apart: the post-ship tree classifies as %q, want the binary-release banner", postShipBanner)
	}
}
