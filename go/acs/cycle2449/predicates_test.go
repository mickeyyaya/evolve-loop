//go:build acs

package cycle2449

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	movedItem       = ".evolve/inbox/moved.json"
	quarantinedItem = ".evolve/inbox/quarantine/moved.json"
	trackedBody     = `{"id":"moved","weight":0.5,"failure_count":1}`
	stampedBody     = `{"id":"moved","weight":0.5,"route":"console-manual","routed_reason":"protected surface","failure_count":2}`
	editedBody      = `{"id":"moved","weight":0.9,"failure_count":1}`
	refusedExit     = 3
)

var (
	binaryOnce sync.Once
	binaryPath string
	binaryErr  string
)

func evolveBinary(t *testing.T) string {
	t.Helper()
	binaryOnce.Do(func() {
		dir, err := os.MkdirTemp("", "cycle2449-evolve-")
		if err != nil {
			binaryErr = err.Error()
			return
		}
		binaryPath = filepath.Join(dir, "evolve")
		_, stderr, code, runErr := acsassert.SubprocessOutput("go", "-C", acsassert.GoModuleDir(t), "build", "-o", binaryPath, "./cmd/evolve")
		if runErr != nil || code != 0 {
			binaryErr = fmt.Sprintf("go build ./cmd/evolve exit %d: %s", code, stderr)
		}
	})
	if binaryErr != "" {
		t.Fatalf("cannot build the evolve binary under test: %s", binaryErr)
	}
	return binaryPath
}

func writeFile(t *testing.T, repo *gittest.Repo, rel, body string) {
	t.Helper()
	path := filepath.Join(repo.Dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func plane(t *testing.T) (*gittest.Repo, *gittest.Repo) {
	t.Helper()
	origin := gittest.Bare(t)
	seed := gittest.Clone(t, origin.Dir)
	seed.Git("config", "user.email", "ci@example.com")
	seed.Git("config", "user.name", "ci")
	seed.Git("config", "commit.gpgsign", "false")
	writeFile(t, seed, movedItem, trackedBody)
	writeFile(t, seed, "base.txt", "base\n")
	seed.Git("add", "-A")
	seed.Git("commit", "-q", "-m", "seed")
	seed.Git("push", "-q", "origin", "main")

	repo := gittest.Clone(t, origin.Dir)
	repo.Git("config", "user.email", "ci@example.com")
	repo.Git("config", "user.name", "ci")
	repo.Git("config", "commit.gpgsign", "false")
	writeFile(t, seed, "origin-moves-on.txt", "origin moved\n")
	seed.Git("add", "-A")
	seed.Git("commit", "-q", "-m", "origin moves on")
	seed.Git("push", "-q", "origin", "main")
	return repo, origin
}

func syncMain(t *testing.T, repo *gittest.Repo) (string, string, int) {
	t.Helper()
	stdout, stderr, code, _ := acsassert.SubprocessOutput(evolveBinary(t), "sync-main", "--project-root", repo.Dir)
	return stdout, stderr, code
}

func quarantineMove(t *testing.T, repo *gittest.Repo, body string) {
	t.Helper()
	if err := os.Remove(filepath.Join(repo.Dir, movedItem)); err != nil {
		t.Fatal(err)
	}
	writeFile(t, repo, quarantinedItem, body)
}

func TestC2449_001_SyncMainCarriesTheLoopsQuarantineMoveAndMainRecordsIt(t *testing.T) {
	repo, _ := plane(t)
	quarantineMove(t, repo, stampedBody)
	headBefore := repo.Git("rev-parse", "HEAD")

	stdout, stderr, code := syncMain(t, repo)

	if code != 0 {
		t.Fatalf("sync-main exit %d, want 0: a loop quarantine move is the loop's own state, not dirt\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	if got := repo.Git("ls-tree", "-r", "--name-only", "HEAD", "--", movedItem, quarantinedItem); got != quarantinedItem {
		t.Errorf("HEAD tracks %q, want only the quarantine copy %q: main must record the move", got, quarantinedItem)
	}
	if got := repo.Git("status", "--porcelain", "--untracked-files=no"); got != "" {
		t.Errorf("tracked tree still dirty after the sync:\n%s", got)
	}
	if got := repo.Git("log", "--format=%s", headBefore+"..HEAD"); !strings.Contains(got, "quarantine move") {
		t.Errorf("no commit records the move since %s; log:\n%s", headBefore, got)
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, "origin-moves-on.txt")); err != nil {
		t.Errorf("the merge of origin did not happen after the move was recorded: %v", err)
	}
}

func TestC2449_002_AnyOtherDeletionOrEditOfATrackedInboxItemStillRefuses(t *testing.T) {
	cases := []struct {
		name  string
		dirty func(t *testing.T, repo *gittest.Repo)
	}{
		{"deletion with no quarantine copy", func(t *testing.T, repo *gittest.Repo) {
			if err := os.Remove(filepath.Join(repo.Dir, movedItem)); err != nil {
				t.Fatal(err)
			}
		}},
		{"deletion with a quarantine copy the operator edited", func(t *testing.T, repo *gittest.Repo) {
			quarantineMove(t, repo, editedBody)
		}},
		{"deletion with a copy outside the quarantine directory", func(t *testing.T, repo *gittest.Repo) {
			if err := os.Remove(filepath.Join(repo.Dir, movedItem)); err != nil {
				t.Fatal(err)
			}
			writeFile(t, repo, ".evolve/inbox/elsewhere/moved.json", stampedBody)
		}},
		{"in-place edit of a field the loop never writes", func(t *testing.T, repo *gittest.Repo) {
			writeFile(t, repo, movedItem, editedBody)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, _ := plane(t)
			tc.dirty(t, repo)
			headBefore := repo.Git("rev-parse", "HEAD")

			_, stderr, code := syncMain(t, repo)

			if code != refusedExit || !strings.Contains(stderr, movedItem) {
				t.Errorf("sync-main exit %d stderr %q, want exit %d naming %s", code, stderr, refusedExit, movedItem)
			}
			if got := repo.Git("rev-parse", "HEAD"); got != headBefore {
				t.Errorf("HEAD moved from %s to %s: a refusal records and merges nothing", headBefore, got)
			}
		})
	}
}

// acs-predicate: config-check
func TestC2449_003_TheSyncMainSectionOfTheBoundaryDocsRecordsTheQuarantineMoveRule(t *testing.T) {
	root := acsassert.RepoRoot(t)
	docs := []string{
		filepath.Join(root, "docs", "operations", "runtime-reference.md"),
		filepath.Join(root, "docs", "operations", "wave-operations.md"),
	}
	found := false
	for _, doc := range docs {
		if acsassert.LineContainsAll(doc, "sync-main", "quarantine", ".evolve/inbox/quarantine/") {
			found = true
		}
	}
	if !found {
		t.Errorf("no line of %v names sync-main, the quarantine move and .evolve/inbox/quarantine/", docs)
	}
}
