package qualitypanel

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

const day = 24 * time.Hour

var fixtureNow = time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

func commitAt(t *testing.T, repo *gittest.Repo, when time.Time, files map[string]string, messages ...string) string {
	t.Helper()
	stamp := when.UTC().Format(time.RFC3339)
	t.Setenv("GIT_AUTHOR_DATE", stamp)
	t.Setenv("GIT_COMMITTER_DATE", stamp)
	for rel, body := range files {
		path := filepath.Join(repo.Dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if body == "" {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	repo.Git("add", "-A")
	args := []string{"commit", "-q", "--allow-empty"}
	for _, m := range messages {
		args = append(args, "-m", m)
	}
	repo.Git(args...)
	return repo.Git("rev-parse", "HEAD")
}

func numberedLines(prefix string, n int) string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = prefix + strconv.Itoa(i+1) + "\n"
	}
	return strings.Join(lines, "")
}

func countingRunner(calls *int, failAt int) gitRunner {
	return func(dir, stdin, name string, args ...string) (string, error) {
		*calls++
		if *calls == failAt {
			return "", errors.New("injected git failure")
		}
		return runIn(dir, stdin, name, args...)
	}
}

func readerFailingAt(repo string, calls *int, failAt int) *gitReader {
	return &gitReader{repo: repo, run: countingRunner(calls, failAt)}
}
