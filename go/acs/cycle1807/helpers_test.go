//go:build acs

package cycle1807

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/releasetargets"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	exitOK      = 0
	exitRefused = 1
	exitIO      = 2
	releaseTag  = "v9.9.9"
	releaseID   = "424242"
	runID       = "777001"
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
			fmt.Fprintf(os.Stderr, "cycle1807: remove %s: %v\n", buildDir, err)
		}
	}
	os.Exit(code)
}

func evolveBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "cycle1807-evolve-")
		if err != nil {
			buildFailure = err.Error()
			return
		}
		buildDir, buildPath = dir, filepath.Join(dir, "evolve")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", "build", "-o", buildPath, "./cmd/evolve")
		cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
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

func isolatedEnv(extra ...string) []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "EVOLVE_") || strings.HasPrefix(name, "TMUX") || strings.HasPrefix(name, "GIT_") || strings.HasPrefix(name, "GH_") || name == "GITHUB_TOKEN" {
			continue
		}
		env = append(env, kv)
	}
	return append(append(env, "GIT_TERMINAL_PROMPT=0"), extra...)
}

func runEvolve(t *testing.T, dir string, env []string, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
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

func commitFile(t *testing.T, r *gittest.Repo, name, content, msg string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(r.Dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	r.Git("add", name)
	r.Git("-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", msg)
	return r.Git("rev-parse", "HEAD")
}

func bundleOf(t *testing.T, r *gittest.Repo, dir, name, rev string) {
	t.Helper()
	r.Git("bundle", "create", filepath.Join(dir, name), rev)
}

func newestPatchText(file, content string) string {
	return fmt.Sprintf("diff --git a/%[1]s b/%[1]s\nnew file mode 100644\n--- /dev/null\n+++ b/%[1]s\n@@ -0,0 +1 @@\n+%[2]s\n", file, content)
}

func editPatchText(file, from, to string) string {
	return fmt.Sprintf("diff --git a/%[1]s b/%[1]s\n--- a/%[1]s\n+++ b/%[1]s\n@@ -1 +1 @@\n-%[2]s\n+%[3]s\n", file, from, to)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func repoState(r *gittest.Repo) string {
	return strings.Join([]string{
		r.Git("status", "--porcelain=v1", "--untracked-files=all"),
		r.Git("ls-files", "--stage"),
		r.Git("for-each-ref"),
	}, "\n--\n")
}

func rowStatus(t *testing.T, stdout, file string) string {
	t.Helper()
	for _, line := range strings.Split(stdout, "\n") {
		fields := strings.Fields(line)
		if !slices.Contains(fields, file) {
			continue
		}
		for _, status := range []string{"UNAPPLIED", "APPLIED"} {
			if slices.Contains(fields, status) {
				return status
			}
		}
	}
	t.Fatalf("no PATCH row with a status for %s in:\n%s", file, stdout)
	return ""
}

func dirSnapshot(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&b, "%s %d %d %x\n", e.Name(), info.Size(), info.ModTime().UnixNano(), data)
	}
	return b.String()
}

type backupFixture struct {
	repo   *gittest.Repo
	dir    string
	base   string
	mainAt string
}

func newBackupFixture(t *testing.T) backupFixture {
	t.Helper()
	repo := gittest.Fixture(t)
	base := commitFile(t, repo, "a.txt", "one\n", "base")
	repo.Git("update-ref", "refs/remotes/origin/main", base)
	return backupFixture{repo: repo, dir: t.TempDir(), base: base, mainAt: base}
}

func (f backupFixture) verify(t *testing.T, extra ...string) result {
	t.Helper()
	env := isolatedEnv("EVOLVE_PROJECT_ROOT=" + f.repo.Dir)
	args := append([]string{"backups", "verify", "--dir", f.dir}, extra...)
	return runEvolve(t, f.repo.Dir, env, args...)
}

func (f backupFixture) landOnOriginMain(t *testing.T, name, content, msg string) string {
	t.Helper()
	sha := commitFile(t, f.repo, name, content, msg)
	f.repo.Git("update-ref", "refs/remotes/origin/main", sha)
	return sha
}

func (f backupFixture) orphanCommit(t *testing.T, file string) string {
	t.Helper()
	f.repo.Git("checkout", "-q", "-b", "scratch-"+file, f.base)
	sha := commitFile(t, f.repo, file, file+"\n", "orphan "+file)
	f.repo.Git("checkout", "-q", "main")
	return sha
}

func fakeGH(t *testing.T) (binDir, logPath string) {
	t.Helper()
	binDir = t.TempDir()
	logPath = filepath.Join(t.TempDir(), "gh.log")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$FAKE_GH_LOG"
if [ -n "$FAKE_GH_FAIL" ]; then
  echo "gh: HTTP 500 simulated" >&2
  exit 1
fi
if [ "$1" = api ]; then
  shift
  endpoint=""
  value_next=""
  for a in "$@"; do
    if [ -n "$value_next" ]; then value_next=""; continue; fi
    case "$a" in
      -X|--method|-F|--field|-f|--raw-field|-H|--header|--hostname|-q|--jq|-t|--template|--input|--cache|-p|--preview) value_next=1 ;;
      -i|--include|--paginate|--silent|--slurp|--verbose|--allow-escape-sequences) ;;
      --method=*|--field=*|--raw-field=*|--header=*|--hostname=*|--jq=*|--template=*|--input=*|--cache=*|--preview=*) ;;
      -[XFfHqtp]?*) ;;
      --*) echo "unknown flag: ${a%%=*}" >&2; echo "Usage:  gh api <endpoint> [flags]" >&2; exit 1 ;;
      -?*) echo "unknown shorthand flag: '$(printf '%s' "$a" | cut -c2)' in $a" >&2; echo "Usage:  gh api <endpoint> [flags]" >&2; exit 1 ;;
      *) if [ -z "$endpoint" ]; then endpoint="${a#/}"; fi ;;
    esac
  done
  case "$endpoint" in
    "repos/$FAKE_GH_REPO/"*) ;;
    *) echo "gh: Not Found (HTTP 404)" >&2; exit 1 ;;
  esac
  set -- api "$@"
else
  repo="$GH_REPO"
  prev=""
  for a in "$@"; do
    case "$prev" in -R|--repo) repo="$a" ;; esac
    case "$a" in --repo=*) repo="${a#--repo=}" ;; -R?*) repo="${a#-R}" ;; esac
    prev="$a"
  done
  if [ -z "$repo" ]; then
    echo "failed to run git: fatal: not a git repository (or any of the parent directories): .git" >&2
    exit 1
  fi
  if [ "$repo" != "$FAKE_GH_REPO" ]; then
    echo "HTTP 404: Not Found" >&2
    exit 1
  fi
fi
case "$*" in
  *PATCH*) echo '{}'; exit 0 ;;
  "run rerun"*|"run watch"*) exit 0 ;;
  "run list"*|"run view"*)
    conclusion="$FAKE_GH_CONCLUSION"
    if [ "$1 $2" = "run list" ]; then
      printf '[{"databaseId":%s,"status":"completed","conclusion":"%s","workflowName":"release","headBranch":"%s","name":"release"}]\n' "$FAKE_GH_RUN_ID" "$conclusion" "$FAKE_GH_TAG"
    else
      printf '{"databaseId":%s,"status":"completed","conclusion":"%s","workflowName":"release","headBranch":"%s","name":"release"}\n' "$FAKE_GH_RUN_ID" "$conclusion" "$FAKE_GH_TAG"
    fi
    exit 0 ;;
esac
assets=""
for a in $FAKE_GH_ASSETS; do
  if [ -n "$assets" ]; then assets="$assets,"; fi
  assets="$assets{\"name\":\"$a\"}"
done
printf '{"id":%s,"tag_name":"%s","prerelease":true,"draft":false,"assets":[%s]}\n' "$FAKE_GH_RELEASE_ID" "$FAKE_GH_TAG" "$assets"
`
	if err := os.WriteFile(filepath.Join(binDir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return binDir, logPath
}

func releaseRepo(t *testing.T) string {
	t.Helper()
	cfg, err := releasetargets.ParseConfig(filepath.Join(acsassert.RepoRoot(t), ".goreleaser.yml"))
	if err != nil {
		t.Fatal(err)
	}
	return cfg.RepoOwner + "/" + cfg.RepoName
}

func expectedAssets(t *testing.T) []string {
	t.Helper()
	cfg, err := releasetargets.ParseConfig(filepath.Join(acsassert.RepoRoot(t), ".goreleaser.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tg := range cfg.Targets {
		n, err := cfg.AssetName(tg)
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, n)
	}
	return append(names, cfg.ChecksumsName)
}

type promoteOpts struct {
	conclusion string
	assets     []string
	fail       bool
	args       []string
}

func promote(t *testing.T, o promoteOpts) (result, []string) {
	t.Helper()
	binDir, logPath := fakeGH(t)
	env := isolatedEnv(
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"FAKE_GH_LOG="+logPath,
		"FAKE_GH_CONCLUSION="+o.conclusion,
		"FAKE_GH_ASSETS="+strings.Join(o.assets, " "),
		"FAKE_GH_TAG="+releaseTag,
		"FAKE_GH_RELEASE_ID="+releaseID,
		"FAKE_GH_RUN_ID="+runID,
		"FAKE_GH_REPO="+releaseRepo(t),
		"EVOLVE_PROJECT_ROOT="+acsassert.RepoRoot(t),
		"EVOLVE_WORKTREE_ROOT="+acsassert.RepoRoot(t),
	)
	if o.fail {
		env = append(env, "FAKE_GH_FAIL=1")
	}
	r := runEvolve(t, t.TempDir(), env, append([]string{"release-promote"}, o.args...)...)
	raw, err := os.ReadFile(logPath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var calls []string
	for _, l := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if l != "" {
			calls = append(calls, l)
		}
	}
	return r, calls
}

func patchCalls(calls []string) []string {
	var out []string
	for _, c := range calls {
		if strings.Contains(c, "PATCH") {
			out = append(out, c)
		}
	}
	return out
}
