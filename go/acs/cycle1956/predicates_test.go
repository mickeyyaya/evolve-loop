//go:build acs

package cycle1956

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	archiveDir = "docs/history/code-comments"
	indexName  = "README.md"
)

var (
	packages = []string{"alpha", "beta", "gamma", "delta"}
	indexRow = regexp.MustCompile("^\\| `[^`]*` \\| (\\d+) \\| \\[([^\\]]+)\\]\\(([^)]+)\\) \\|$")
)

var commentauditBuild struct {
	once        sync.Once
	dir, path   string
	buildFailed string
}

func TestMain(m *testing.M) {
	code := m.Run()
	if commentauditBuild.dir != "" {
		if err := os.RemoveAll(commentauditBuild.dir); err != nil {
			fmt.Fprintf(os.Stderr, "cycle1956: remove %s: %v\n", commentauditBuild.dir, err)
		}
	}
	os.Exit(code)
}

func commentauditBin(t *testing.T) string {
	t.Helper()
	commentauditBuild.once.Do(func() {
		dir, err := os.MkdirTemp("", "cycle1956-commentaudit-")
		if err != nil {
			commentauditBuild.buildFailed = err.Error()
			return
		}
		commentauditBuild.dir, commentauditBuild.path = dir, filepath.Join(dir, "commentaudit")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", "build", "-o", commentauditBuild.path, "./cmd/commentaudit")
		cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
		if out, err := cmd.CombinedOutput(); err != nil {
			commentauditBuild.buildFailed = fmt.Sprintf("%v\n%s", err, out)
		}
	})
	if commentauditBuild.buildFailed != "" {
		t.Fatalf("go build ./cmd/commentaudit: %s", commentauditBuild.buildFailed)
	}
	return commentauditBuild.path
}

func goSource(pkg string, cycle int) string {
	return fmt.Sprintf("package %s\n\n// %s\nfunc F%d() {}\n", pkg, note(cycle, pkg), cycle)
}

func note(cycle int, pkg string) string {
	return fmt.Sprintf("Cycle %d found the retry double-counted in %s; see the 2026-09-14 incident.", cycle, pkg)
}

func sourcePath(pkg string, cycle int) string {
	return fmt.Sprintf("go/internal/%s/c%d.go", pkg, cycle)
}

func runIn(t *testing.T, dir, name string, args ...string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir, cmd.WaitDelay = dir, 5*time.Second
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return out.String(), 0
	case errors.As(err, &exitErr):
		return out.String(), exitErr.ExitCode()
	default:
		t.Fatalf("run %s %v: %v", name, args, err)
		return "", -1
	}
}

func seededArchive(t *testing.T) *gittest.Repo {
	t.Helper()
	bin := commentauditBin(t)
	attributes, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), ".gitattributes"))
	if err != nil {
		t.Fatalf("read the repo .gitattributes: %v", err)
	}
	r := gittest.Fixture(t)
	r.Write(".gitattributes", string(attributes))
	for _, pkg := range packages {
		for _, cycle := range []int{10, 11, 12} {
			r.Write(sourcePath(pkg, cycle), goSource(pkg, cycle))
		}
	}
	r.Git("add", "-A")
	r.Git("commit", "-q", "-m", "init")
	for _, pkg := range packages {
		r.Write(sourcePath(pkg, 10), "package "+pkg+"\n\nfunc F10() {}\n")
	}
	out, code := runIn(t, r.Dir, bin, "history", "-base", "HEAD", "-label", "seed", "-out", archiveDir)
	if code != 0 || !strings.Contains(out, "recorded") {
		t.Fatalf("seed archive: exit=%d\n%s", code, out)
	}
	r.Git("add", "-A")
	r.Git("commit", "-q", "-m", "seed")
	return r
}

func archiveOnBranch(t *testing.T, r *gittest.Repo, branch, pkg string, cycle int) {
	t.Helper()
	r.Git("checkout", "-q", "main")
	r.Git("checkout", "-q", "-b", branch)
	r.Write(sourcePath(pkg, cycle), "package "+pkg+"\n\nfunc F"+fmt.Sprint(cycle)+"() {}\n")
	out, code := runIn(t, r.Dir, commentauditBin(t), "history", "-base", "main", "-label", branch, "-out", archiveDir)
	if code != 0 || !strings.Contains(out, "recorded 1 history comment group(s) in 1 archive page(s)") {
		t.Fatalf("archive on %s: exit=%d\n%s", branch, code, out)
	}
	r.Git("add", "-A")
	r.Git("commit", "-q", "-m", branch)
}

func mergeBranches(t *testing.T, r *gittest.Repo, into, from string) (string, int) {
	t.Helper()
	r.Git("checkout", "-q", into)
	return runIn(t, r.Dir, "git", "-C", r.Dir, "merge", "--no-edit", from)
}

func readIndex(t *testing.T, r *gittest.Repo) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(r.Dir, archiveDir, indexName))
	if err != nil {
		t.Fatalf("read the merged index: %v", err)
	}
	return string(body)
}

type indexEntry struct {
	page  string
	count string
}

func indexEntries(index string) []indexEntry {
	var rows []indexEntry
	for _, line := range strings.Split(index, "\n") {
		if m := indexRow.FindStringSubmatch(line); m != nil {
			rows = append(rows, indexEntry{page: m[3], count: m[1]})
		}
	}
	return rows
}

func actualPages(t *testing.T, r *gittest.Repo) map[string]string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(r.Dir, archiveDir, "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	pages := map[string]string{}
	for _, f := range files {
		if filepath.Base(f) == indexName {
			continue
		}
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(body), "# Comment history: `") {
			t.Fatalf("%s is not a history page after the merge:\n%s", f, body)
		}
		pages[filepath.Base(f)] = fmt.Sprint(strings.Count(string(body), "\n### `"))
	}
	return pages
}

func requireIndexCoversPages(t *testing.T, r *gittest.Repo, exactRows bool) {
	t.Helper()
	index := readIndex(t, r)
	if strings.Contains(index, "<<<<<<<") || strings.Contains(index, ">>>>>>>") {
		t.Fatalf("conflict markers in the merged index:\n%s", index)
	}
	pages := actualPages(t, r)
	listed := map[string][]string{}
	for _, row := range indexEntries(index) {
		listed[row.page] = append(listed[row.page], row.count)
	}
	for _, page := range slices.Sorted(maps.Keys(pages)) {
		counts := listed[page]
		switch {
		case len(counts) == 0:
			t.Errorf("index omits %s:\n%s", page, index)
		case exactRows && (len(counts) != 1 || counts[0] != pages[page]):
			t.Errorf("index lists %s as %v, page holds %s entries", page, counts, pages[page])
		case !slices.Contains(counts, pages[page]):
			t.Errorf("no index row gives %s its %s entries, rows say %v", page, pages[page], counts)
		}
	}
	for page := range listed {
		if _, ok := pages[page]; !ok {
			t.Errorf("index lists %s, which is no page", page)
		}
	}
}

func TestC1956_001_AdjacentPagesFromTwoBranchesMergeWithoutConflict(t *testing.T) {
	r := seededArchive(t)
	archiveOnBranch(t, r, "lane-a", "alpha", 11)
	archiveOnBranch(t, r, "lane-b", "beta", 11)

	out, code := mergeBranches(t, r, "lane-a", "lane-b")

	if code != 0 || strings.Contains(out, "CONFLICT") {
		t.Fatalf("merging a branch that archived beta into one that archived alpha: exit=%d\n%s", code, out)
	}
}

func TestC1956_002_MergedIndexOfAdjacentPagesListsEveryPageWithItsEntryCount(t *testing.T) {
	r := seededArchive(t)
	archiveOnBranch(t, r, "lane-a", "alpha", 11)
	archiveOnBranch(t, r, "lane-b", "beta", 11)

	out, code := mergeBranches(t, r, "lane-a", "lane-b")
	if code != 0 {
		t.Fatalf("the merge did not finish, so no index to read: exit=%d\n%s", code, out)
	}

	requireIndexCoversPages(t, r, false)
}

func TestC1956_003_DistantPagesFromTwoBranchesStillMergeWithCorrectIndex(t *testing.T) {
	r := seededArchive(t)
	archiveOnBranch(t, r, "lane-a", "alpha", 11)
	archiveOnBranch(t, r, "lane-d", "delta", 11)

	out, code := mergeBranches(t, r, "lane-a", "lane-d")
	if code != 0 || strings.Contains(out, "CONFLICT") {
		t.Fatalf("merging distant pages: exit=%d\n%s", code, out)
	}

	requireIndexCoversPages(t, r, true)
}

func TestC1956_004_SamePageSectionsFromTwoBranchesMergeWithoutConflict(t *testing.T) {
	r := seededArchive(t)
	archiveOnBranch(t, r, "lane-a", "alpha", 11)
	archiveOnBranch(t, r, "lane-b", "alpha", 12)

	out, code := mergeBranches(t, r, "lane-a", "lane-b")

	if code != 0 || strings.Contains(out, "CONFLICT") {
		t.Fatalf("two branches appended a section to the same page: exit=%d\n%s", code, out)
	}
	page, err := os.ReadFile(filepath.Join(r.Dir, archiveDir, "internal-alpha.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, section := range []string{"seed", "lane-a", "lane-b"} {
		if n := strings.Count(string(page), "\n## "+section+"\n"); n != 1 {
			t.Errorf("merged page holds section %q %d times, want once:\n%s", section, n, page)
		}
	}
	if strings.Contains(string(page), "<<<<<<<") {
		t.Errorf("conflict markers in the merged page:\n%s", page)
	}
}

func TestC1956_005_UnionMergeStaysOnTheHistoryArchive(t *testing.T) {
	root := acsassert.RepoRoot(t)
	attr := func(path string) string {
		out, code := runIn(t, root, "git", "-C", root, "check-attr", "merge", "--", path)
		if code != 0 {
			t.Fatalf("git check-attr merge %s: exit=%d\n%s", path, code, out)
		}
		return strings.TrimSpace(out)
	}
	for _, path := range []string{
		"docs/conventions/code-comments.md",
		"docs/architecture/adr/0115-commit-gate-keeps-history-and-records-waivers.md",
		"go/internal/commentaudit/history.go",
		"CLAUDE.md",
	} {
		if got := attr(path); strings.HasSuffix(got, ": union") {
			t.Errorf("%s merges as union; the rule must stay on the history archive: %s", path, got)
		}
	}
	if got := attr("go/.apicover-enforce"); !strings.HasSuffix(got, ": union") {
		t.Errorf("the existing apicover union registry lost its rule: %s", got)
	}
}

func TestC1956_006_NoChangeUnderTheProtectedCommentauditPackage(t *testing.T) {
	root := acsassert.RepoRoot(t)
	base, code := runIn(t, root, "git", "-C", root, "merge-base", "HEAD", "main")
	if code != 0 {
		t.Fatalf("git merge-base HEAD main: exit=%d\n%s", code, base)
	}
	out, code := runIn(t, root, "git", "-C", root, "diff", "--name-only", strings.TrimSpace(base), "--", "go/internal/commentaudit")
	if code != 0 {
		t.Fatalf("git diff: exit=%d\n%s", code, out)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("the lane changed the protected surface go/internal/commentaudit:\n%s", out)
	}
}
