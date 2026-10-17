//go:build acs

package cycle1937

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

type hubFixture struct {
	t       *testing.T
	store   *gittest.Repo
	root    string
	runtime string
	now     time.Time
}

func newHubFixture(t *testing.T) *hubFixture {
	t.Helper()
	src := gittest.Fixture(t)
	writeFile(t, filepath.Join(src.Dir, "base.txt"), "base\n")
	src.Git("add", "-A")
	src.Git("commit", "-q", "-m", "base")
	store := gittest.Bare(t)
	src.Git("push", "-q", store.Dir, "main")
	store.Git("update-ref", "refs/remotes/origin/main", "main")
	root := filepath.Dir(store.Dir)
	runtime := filepath.Join(root, "runtime")
	store.Git("worktree", "add", "-q", "--detach", runtime, "main")
	return &hubFixture{t: t, store: store, root: root, runtime: runtime, now: time.Now()}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (h *hubFixture) addDev(name string) string {
	h.t.Helper()
	dir := filepath.Join(h.root, "dev", name)
	h.store.Git("worktree", "add", "-q", "-b", "br-"+name, dir, "main")
	return dir
}

func (h *hubFixture) commitIn(dir, rel string) {
	h.t.Helper()
	writeFile(h.t, filepath.Join(dir, rel), rel+"\n")
	h.store.Git("-C", dir, "add", "-A")
	h.store.Git("-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "work "+rel)
}

func (h *hubFixture) mergeIntoMain(dir string) {
	h.t.Helper()
	head := h.store.Git("-C", dir, "rev-parse", "HEAD")
	h.store.Git("update-ref", "refs/heads/main", head)
	h.store.Git("update-ref", "refs/remotes/origin/main", head)
}

func (h *hubFixture) squashIntoMain(branch string) {
	h.t.Helper()
	scratch := filepath.Join(h.root, "scratch-"+branch)
	h.store.Git("worktree", "add", "-q", "--detach", scratch, "refs/remotes/origin/main")
	h.store.Git("-C", scratch, "merge", "-q", "--squash", branch)
	h.store.Git("-C", scratch, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "squash "+branch)
	h.store.Git("update-ref", "refs/remotes/origin/main", h.store.Git("-C", scratch, "rev-parse", "HEAD"))
	h.store.Git("worktree", "remove", "--force", scratch)
}

func (h *hubFixture) plan(prs map[string][]time.Time) gc.DevManifest {
	h.t.Helper()
	o := gc.DevOptions{
		ProjectRoot: h.runtime,
		EvolveDir:   filepath.Join(h.runtime, ".evolve"),
		Now:         func() time.Time { return h.now },
		Exec:        sysexec.DefaultRunner,
		Processes:   func(context.Context) ([]gc.Process, error) { return nil, nil },
		OpenPRs: func(context.Context, string) (map[string][]time.Time, error) {
			return prs, nil
		},
	}
	m, err := gc.PlanDevWorktrees(context.Background(), o)
	if err != nil {
		h.t.Fatalf("PlanDevWorktrees: %v", err)
	}
	return m
}

func itemFor(t *testing.T, m gc.DevManifest, dir string) gc.DevItem {
	t.Helper()
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range m.Items {
		if got, err := filepath.EvalSymlinks(it.Path); err == nil && got == want {
			return it
		}
	}
	t.Fatalf("no manifest item for %s in %+v", dir, m.Items)
	return gc.DevItem{}
}

func TestC1937_001_ALandedCleanDevWorktreeWithNoOpenPRIsArchivedInsideTheIdleLimit(t *testing.T) {
	cases := []struct {
		name string
		land func(h *hubFixture, dir string)
	}{
		{"merged", func(h *hubFixture, dir string) { h.mergeIntoMain(dir) }},
		{"squashed", func(h *hubFixture, _ string) { h.squashIntoMain("br-squashed") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHubFixture(t)
			dir := h.addDev(tc.name)
			h.commitIn(dir, tc.name+".txt")
			tc.land(h, dir)
			it := itemFor(t, h.plan(nil), dir)
			if it.Action != gc.DevActionArchive {
				t.Fatalf("RED: a fresh, clean dev worktree whose branch landed in origin/main (%s) with no open PR = %+v, want archived without waiting out the idle limit", tc.name, it)
			}
			if !strings.Contains(it.Reason, "main") {
				t.Errorf("archive reason %q, want it to say the branch landed in main", it.Reason)
			}
		})
	}
}

func TestC1937_002_ALandedDevWorktreeIsKeptWhenDirtyWithAnOpenPROrAheadOfMain(t *testing.T) {
	cases := []struct {
		name    string
		arrange func(h *hubFixture, dir string) map[string][]time.Time
	}{
		{"dirty", func(h *hubFixture, dir string) map[string][]time.Time {
			writeFile(h.t, filepath.Join(dir, "base.txt"), "uncommitted edit\n")
			return nil
		}},
		{"stale-open-pr", func(h *hubFixture, _ string) map[string][]time.Time {
			return map[string][]time.Time{"br-stale-open-pr": {h.now.Add(-30 * 24 * time.Hour)}}
		}},
		{"commit-beyond-main", func(h *hubFixture, dir string) map[string][]time.Time {
			h.commitIn(dir, "beyond.txt")
			return nil
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHubFixture(t)
			dir := h.addDev(tc.name)
			h.commitIn(dir, tc.name+".txt")
			h.mergeIntoMain(dir)
			prs := tc.arrange(h, dir)
			if it := itemFor(t, h.plan(prs), dir); it.Action != gc.DevActionKeep {
				t.Errorf("a fresh landed dev worktree that is %s = %+v, want kept", tc.name, it)
			}
		})
	}
}

func TestC1937_003_AFreshDevWorktreeWithNoWorkOfItsOwnIsStillKept(t *testing.T) {
	h := newHubFixture(t)
	dir := h.addDev("just-created")
	if it := itemFor(t, h.plan(nil), dir); it.Action != gc.DevActionKeep {
		t.Errorf("a dev worktree created at main a moment ago, with no commit of its own = %+v, want kept: it has no merged work, it is new", it)
	}
}

func buildEvolve(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "evolve")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/evolve")
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build evolve: %v\n%s", err, out)
	}
	return bin
}

func goOnlyPath(t *testing.T) string {
	t.Helper()
	goroot, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		t.Fatalf("go env GOROOT: %v", err)
	}
	bin := filepath.Join(t.TempDir(), "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(strings.TrimSpace(string(goroot)), "bin", "go"), filepath.Join(bin, "go")); err != nil {
		t.Fatal(err)
	}
	return bin + string(os.PathListSeparator) + "/usr/bin" + string(os.PathListSeparator) + "/bin"
}

type gcSandbox struct {
	project, gocache, tmp  string
	staleCache, freshCache string
	staleTemp, foreignTemp string
}

func newGCSandbox(t *testing.T) gcSandbox {
	t.Helper()
	base := t.TempDir()
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, "base.txt"), "base\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	writeFile(t, filepath.Join(repo.Dir, ".evolve", "policy.json"), `{"gc":{"go_cache_ttl_hours":24,"temp_ttl_hours":24}}`)
	s := gcSandbox{project: repo.Dir, gocache: filepath.Join(base, "gocache"), tmp: filepath.Join(base, "tmp")}
	s.staleCache = filepath.Join(s.gocache, "ab", "abcd-a")
	s.freshCache = filepath.Join(s.gocache, "ab", "abce-d")
	s.staleTemp = filepath.Join(s.tmp, "cycle1-stale")
	s.foreignTemp = filepath.Join(s.tmp, "someone-elses-file")
	old := time.Unix(1600000000, 0)
	for _, p := range []string{s.staleCache, s.freshCache, filepath.Join(s.staleTemp, "x"), s.foreignTemp} {
		writeFile(t, p, "payload\n")
	}
	for _, p := range []string{s.staleCache, filepath.Join(s.staleTemp, "x"), s.staleTemp, s.foreignTemp} {
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func runGC(t *testing.T, bin string, s gcSandbox, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(bin, append([]string{"gc"}, args...)...)
	cmd.Dir = s.project
	cmd.Env = append(os.Environ(), "PATH="+goOnlyPath(t), "GOCACHE="+s.gocache, "TMPDIR="+s.tmp)
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("run evolve gc: %v", err)
	}
	return stdout.String(), stderr.String(), code
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func TestC1937_004_GCOnlyCacheTrimsTheCacheAndTempAndRunsNoOtherSweep(t *testing.T) {
	bin := buildEvolve(t)
	s := newGCSandbox(t)
	if !gittest.BranchExists(t, s.project, "main") {
		t.Fatal("fixture error: the project has no main branch before the cache scope")
	}
	stdout, stderr, code := runGC(t, bin, s, "--only", "cache", "--project-root", s.project)
	if code != 0 {
		t.Fatalf("RED: evolve gc --only cache exit %d, want 0\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if exists(s.staleCache) || !exists(s.freshCache) {
		t.Errorf("go build cache: stale entry present=%v, fresh entry present=%v; want the stale one trimmed and the fresh one kept\nstdout:\n%s", exists(s.staleCache), exists(s.freshCache), stdout)
	}
	if exists(s.staleTemp) || !exists(s.foreignTemp) {
		t.Errorf("temp dir: stale pipeline artifact present=%v, foreign file present=%v; want only the pipeline artifact removed", exists(s.staleTemp), exists(s.foreignTemp))
	}
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		lower := strings.ToLower(line)
		for _, other := range []string{"session", "socket", "worktree", "process", "branch", "run-dir", "claim"} {
			if strings.Contains(lower, other) {
				t.Errorf("the cache scope ran another sweep: stdout line %q names %q", line, other)
			}
		}
	}
	if !strings.Contains(stdout, "go build cache") || !strings.Contains(stdout, "pipeline temp") {
		t.Errorf("stdout = %q, want the go build cache trim and the pipeline temp sweep reported", stdout)
	}
	if !gittest.BranchExists(t, s.project, "main") {
		t.Errorf("the project branch main is gone after the cache scope")
	}
}

func TestC1937_005_GCOnlyRefusesAnUnknownScopeWithAUsageError(t *testing.T) {
	bin := buildEvolve(t)
	s := newGCSandbox(t)
	stdout, stderr, code := runGC(t, bin, s, "--only", "nonsense", "--project-root", s.project)
	if code == 0 {
		t.Fatalf("evolve gc --only nonsense exit 0, want a usage refusal\nstdout:\n%s", stdout)
	}
	if strings.Contains(stderr, "flag provided but not defined") {
		t.Fatalf("RED: --only is not a flag of evolve gc yet: %s", stderr)
	}
	if !strings.Contains(stderr, "nonsense") {
		t.Errorf("stderr = %q, want the refused scope value named", stderr)
	}
	if !exists(s.staleCache) || !exists(s.staleTemp) {
		t.Errorf("a refused scope still swept: stale cache present=%v, stale temp present=%v; want nothing touched", exists(s.staleCache), exists(s.staleTemp))
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("a refused scope printed sweep output: %q", stdout)
	}
}

func TestC1937_006_GCCacheTrimFreeGBIsAStrictlyDecodedPolicyKey(t *testing.T) {
	p, err := policy.Parse([]byte(`{"gc":{"cache_trim_free_gb":50}}`))
	if err != nil {
		t.Fatalf("a valid gc.cache_trim_free_gb is refused: %v", err)
	}
	raw, err := json.Marshal(p.GC)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"cache_trim_free_gb":50`) {
		t.Errorf("RED: gc.cache_trim_free_gb is dropped on decode: the gc block re-encodes as %s", raw)
	}
	if _, err := policy.Parse([]byte(`{"gc":{"cache_trim_free_gb":"fifty"}}`)); err == nil {
		t.Errorf("RED: a non-numeric gc.cache_trim_free_gb parsed without an error, want it refused")
	}
}

func TestC1937_007_GCOnlyCacheDryRunPreviewsTheTwoStepsAndMutatesNothing(t *testing.T) {
	bin := buildEvolve(t)
	s := newGCSandbox(t)
	stdout, stderr, code := runGC(t, bin, s, "--only", "cache", "--dry-run", "--project-root", s.project)
	if code != 0 {
		t.Fatalf("evolve gc --only cache --dry-run exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !exists(s.staleCache) || !exists(s.staleTemp) {
		t.Errorf("a dry run removed files: stale cache present=%v, stale temp present=%v", exists(s.staleCache), exists(s.staleTemp))
	}
	if !strings.Contains(stdout, "go build cache") || !strings.Contains(stdout, "pipeline temp") {
		t.Errorf("dry-run stdout = %q, want the go build cache and pipeline temp previews", stdout)
	}
	if !gittest.BranchExists(t, s.project, "main") {
		t.Errorf("the project branch main is gone after the dry-run cache scope")
	}
}

func TestC1937_008_GCOnlyCacheTrimsNothingWhileDiskFreeIsAboveTheTrimFloor(t *testing.T) {
	bin := buildEvolve(t)
	s := newGCSandbox(t)
	writeFile(t, filepath.Join(s.project, ".evolve", "policy.json"), `{"gc":{"go_cache_ttl_hours":24,"temp_ttl_hours":24,"cache_trim_free_gb":1}}`)
	stdout, stderr, code := runGC(t, bin, s, "--only", "cache", "--project-root", s.project)
	if code != 0 {
		t.Fatalf("evolve gc --only cache exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !exists(s.staleCache) || !exists(s.staleTemp) {
		t.Errorf("trimmed while disk free is above gc.cache_trim_free_gb=1: stale cache present=%v, stale temp present=%v\nstdout:\n%s", exists(s.staleCache), exists(s.staleTemp), stdout)
	}
}
