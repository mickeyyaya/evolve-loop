//go:build acs

package cycle1810

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	exitOK      = 0
	exitRefused = 1
	exitIO      = 2
	closedPRSHA = "abababababababababababababababababababab"
)

var (
	buildOnce    sync.Once
	buildDir     string
	buildPath    string
	buildFailure string
)

func TestMain(m *testing.M) {
	code := m.Run()
	if buildDir != "" {
		if err := os.RemoveAll(buildDir); err != nil {
			fmt.Fprintf(os.Stderr, "cycle1810: remove %s: %v\n", buildDir, err)
		}
	}
	os.Exit(code)
}

func evolveBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "cycle1810-evolve-")
		if err != nil {
			buildFailure = err.Error()
			return
		}
		buildDir, buildPath = dir, filepath.Join(dir, "evolve")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", "build", "-o", buildPath, "./cmd/evolve")
		cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
		cmd.WaitDelay = 5 * time.Second
		if out, err := cmd.CombinedOutput(); err != nil {
			buildFailure = fmt.Sprintf("%v\n%s", err, out)
		}
	})
	if buildFailure != "" {
		t.Fatalf("go build ./cmd/evolve: %s", buildFailure)
	}
	return buildPath
}

type result struct {
	stdout, stderr string
	code           int
}

func (r result) String() string {
	return fmt.Sprintf("exit=%d\n--- stdout ---\n%s--- stderr ---\n%s", r.code, r.stdout, r.stderr)
}

func (r result) combined() string { return r.stdout + r.stderr }

func sandboxEnv(projectRoot, fakeBinDir string) []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "EVOLVE_") || strings.HasPrefix(name, "TMUX") || strings.HasPrefix(name, "GIT_") || strings.HasPrefix(name, "GH_") || name == "GITHUB_TOKEN" || name == "PATH" {
			continue
		}
		env = append(env, kv)
	}
	path := os.Getenv("PATH")
	if fakeBinDir != "" {
		path = fakeBinDir + string(os.PathListSeparator) + path
	}
	env = append(env, "PATH="+path, "GIT_TERMINAL_PROMPT=0")
	if projectRoot != "" {
		env = append(env, "EVOLVE_PROJECT_ROOT="+projectRoot)
	}
	return env
}

func runEvolve(t *testing.T, dir string, env []string, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, evolveBinary(t), args...)
	cmd.Dir, cmd.Env, cmd.WaitDelay = dir, env, 5*time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	r := result{stdout: stdout.String(), stderr: stderr.String()}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		r.code = exitErr.ExitCode()
	default:
		t.Fatalf("evolve %v: %v", args, err)
	}
	return r
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, code := gitCode(t, dir, args...)
	if code != 0 {
		t.Fatalf("git -C %s %s: exit %d\n%s", dir, strings.Join(args, " "), code, out)
	}
	return out
}

func gitCode(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
	cmd.WaitDelay = 5 * time.Second
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return strings.TrimSpace(string(out)), 0
	case errors.As(err, &exitErr):
		return strings.TrimSpace(string(out)), exitErr.ExitCode()
	default:
		t.Fatalf("git -C %s %s: %v", dir, strings.Join(args, " "), err)
		return "", -1
	}
}

func gitStdout(t *testing.T, dir string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
	cmd.WaitDelay = 5 * time.Second
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git -C %s %s: %v", dir, strings.Join(args, " "), err)
	}
	if len(out) == 0 {
		t.Fatalf("git -C %s %s: empty output; the fixture edit produced no patch", dir, strings.Join(args, " "))
	}
	return string(out)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

const (
	baseConflictText = "one\ntwo\nthree\n"
	baseCleanText    = "clean\n"
)

type hubFixture struct {
	origin  *gittest.Repo
	seed    *gittest.Repo
	root    string
	store   string
	runtime string
	baseSHA string
}

func newHub(t *testing.T) hubFixture {
	t.Helper()
	origin := gittest.Bare(t)
	seed := gittest.Fixture(t)
	writeFile(t, filepath.Join(seed.Dir, "conflict.txt"), baseConflictText)
	writeFile(t, filepath.Join(seed.Dir, "clean.txt"), baseCleanText)
	seed.Git("add", "-A")
	seed.Git("commit", "-q", "-m", "base")
	seed.Git("remote", "add", "origin", origin.Dir)
	seed.Git("push", "-q", "origin", "main")
	root := t.TempDir()
	store := filepath.Join(root, ".repo.git")
	seed.Git("clone", "-q", "--bare", origin.Dir, store)
	for _, kv := range append(gittest.MaintenanceConfig(), [2]string{"user.name", "u"}, [2]string{"user.email", "u@example.com"}) {
		gitIn(t, store, "config", kv[0], kv[1])
	}
	runtime := filepath.Join(root, "runtime")
	gitIn(t, store, "worktree", "add", "-q", runtime, "main")
	if err := os.MkdirAll(filepath.Join(runtime, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	return hubFixture{origin: origin, seed: seed, root: root, store: store, runtime: runtime, baseSHA: seed.Git("rev-parse", "HEAD")}
}

func (h hubFixture) advanceOrigin(t *testing.T, name, content string) string {
	t.Helper()
	writeFile(t, filepath.Join(h.seed.Dir, name), content)
	h.seed.Git("add", "-A")
	h.seed.Git("commit", "-q", "-m", "origin moves on: "+name)
	h.seed.Git("push", "-q", "origin", "main")
	return h.originMain(t)
}

func (h hubFixture) originMain(t *testing.T) string {
	t.Helper()
	return h.origin.Git("rev-parse", "refs/heads/main")
}

func (h hubFixture) lanePatch(t *testing.T, edits map[string]string) string {
	t.Helper()
	for name, content := range edits {
		writeFile(t, filepath.Join(h.seed.Dir, name), content)
	}
	h.seed.Git("add", "-A")
	patch := gitStdout(t, h.seed.Dir, "diff", "--cached", "--binary", "HEAD")
	h.seed.Git("reset", "-q", "--hard", "HEAD")
	path := filepath.Join(t.TempDir(), "lane.patch")
	writeFile(t, path, patch)
	return path
}

func (h hubFixture) branchExists(t *testing.T, repo, branch string) bool {
	t.Helper()
	_, code := gitCode(t, repo, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return code == 0
}

func (h hubFixture) worktreeOnBranch(t *testing.T, branch string) (string, bool) {
	t.Helper()
	listing := gitIn(t, h.store, "worktree", "list", "--porcelain")
	var dir string
	for _, line := range strings.Split(listing, "\n") {
		if p, ok := strings.CutPrefix(line, "worktree "); ok {
			dir = p
			continue
		}
		if line == "branch refs/heads/"+branch {
			return dir, true
		}
	}
	return "", false
}

func (h hubFixture) worktreeCount(t *testing.T) int {
	t.Helper()
	return strings.Count(gitIn(t, h.store, "worktree", "list", "--porcelain"), "worktree ")
}

func (h hubFixture) brakePath() string {
	return filepath.Join(h.runtime, ".evolve", "loop-stop")
}

func (h hubFixture) runtimeHead(t *testing.T) string {
	t.Helper()
	return gitIn(t, h.runtime, "rev-parse", "HEAD")
}

type salvageLeaf struct {
	name      string
	untracked map[string]string
	patched   map[string]string
}

func (h hubFixture) writeSalvageLeaf(t *testing.T, leaf salvageLeaf) string {
	t.Helper()
	dir := filepath.Join(h.runtime, ".evolve", "operator-salvage", leaf.name)
	for name, content := range leaf.patched {
		writeFile(t, filepath.Join(h.seed.Dir, name), content)
	}
	patch := gitStdout(t, h.seed.Dir, "diff", "HEAD", "--binary")
	h.seed.Git("checkout", "-q", "--", ".")
	writeFile(t, filepath.Join(dir, "HEAD"), h.baseSHA+" "+leaf.name+"\n")
	writeFile(t, filepath.Join(dir, "uncommitted.patch"), patch)
	writeUntrackedArchive(t, filepath.Join(dir, "untracked.tgz"), leaf.untracked)
	return dir
}

func writeUntrackedArchive(t *testing.T, dest string, files map[string]string) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(content)), ModTime: time.Unix(1_700_000_000, 0), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dest, buf.String())
}

type fakeBin struct {
	dir   string
	ghLog string
}

func newFakeBin(t *testing.T) fakeBin {
	t.Helper()
	dir := t.TempDir()
	ghLog := filepath.Join(t.TempDir(), "gh-calls.log")
	gh := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$*" >> '%s'
if [ "$1" = "pr" ] && [ "$2" = "view" ]; then
  printf '{"number":%%s,"state":"CLOSED","isDraft":false,"mergeable":"MERGEABLE","headRefOid":"%s","baseRefName":"main"}\n' "$3"
  exit 0
fi
echo "fake gh: unexpected call: $*" >&2
exit 1
`, ghLog, closedPRSHA)
	writeExecutable(t, filepath.Join(dir, "gh"), gh)
	for _, agentCLI := range []string{"tmux", "claude", "codex", "agy", "ollama", "gemini"} {
		writeExecutable(t, filepath.Join(dir, agentCLI), "#!/bin/sh\necho \"fake "+agentCLI+": refused in a predicate sandbox\" >&2\nexit 97\n")
	}
	return fakeBin{dir: dir, ghLog: ghLog}
}

func (f fakeBin) calls(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(f.ghLog)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.FieldsFunc(string(b), func(r rune) bool { return r == '\n' })
}

func (f fakeBin) reset(t *testing.T) {
	t.Helper()
	if err := os.Remove(f.ghLog); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

type planStep struct {
	label    string
	patterns []*regexp.Regexp
}

func word(w string) *regexp.Regexp {
	return regexp.MustCompile(`(^|[^A-Za-z0-9_-])` + regexp.QuoteMeta(w) + `($|[^A-Za-z0-9_-])`)
}

func boundaryPlanSteps(prs ...string) []planStep {
	merge := []*regexp.Regexp{regexp.MustCompile(`(^|[^A-Za-z0-9_-])pr[ -]merge($|[^A-Za-z0-9_-])`)}
	for _, n := range prs {
		merge = append(merge, word(n))
	}
	return []planStep{
		{"loop-stop --wait", []*regexp.Regexp{word("loop-stop"), word("--wait")}},
		{"pr merge " + strings.Join(prs, ","), merge},
		{"sync-main", []*regexp.Regexp{word("sync-main")}},
		{"gc", []*regexp.Regexp{word("gc")}},
		{"loop-stop --release", []*regexp.Regexp{word("loop-stop"), word("--release")}},
		{"loop --detach", []*regexp.Regexp{word("loop"), word("--detach")}},
	}
}

func stepLinesInOrder(out string, steps []planStep) ([]int, string) {
	lines := strings.Split(out, "\n")
	var found []int
	next := 0
	for _, s := range steps {
		hit := -1
		for i := next; i < len(lines); i++ {
			if matchesAll(lines[i], s.patterns) {
				hit = i
				break
			}
		}
		if hit < 0 {
			return found, s.label
		}
		found = append(found, hit)
		next = hit + 1
	}
	return found, ""
}

func matchesAll(line string, patterns []*regexp.Regexp) bool {
	for _, p := range patterns {
		if !p.MatchString(line) {
			return false
		}
	}
	return true
}

func launchPlanLine(out string) (string, bool) {
	launch := boundaryPlanSteps()[5]
	for _, line := range strings.Split(out, "\n") {
		if matchesAll(line, launch.patterns) {
			return line, true
		}
	}
	return "", false
}

func flagValueIn(line, name string) (string, bool) {
	m := regexp.MustCompile(`(?:^|\s)` + regexp.QuoteMeta(name) + `(?:=|\s+)("[^"]*"|'[^']*'|\S+)`).FindStringSubmatch(line)
	if m == nil {
		return "", false
	}
	return strings.Trim(m[1], `"'`), true
}
