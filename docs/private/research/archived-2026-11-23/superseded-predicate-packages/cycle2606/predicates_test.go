//go:build acs

package cycle2606

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var (
	buildOnce sync.Once
	buildDir  string
	buildBin  string
	buildErr  string
)

func TestMain(m *testing.M) {
	code := m.Run()
	if buildDir != "" {
		_ = os.RemoveAll(buildDir)
	}
	os.Exit(code)
}

func evolveBin(t *testing.T) string {
	t.Helper()
	mod := acsassert.GoModuleDir(t)
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "acs2606-bin-")
		if err != nil {
			buildErr = err.Error()
			return
		}
		buildDir = dir
		buildBin = filepath.Join(dir, "evolve")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", "build", "-o", buildBin, "./cmd/evolve")
		cmd.Dir = mod
		cmd.WaitDelay = 10 * time.Second
		if out, err := cmd.CombinedOutput(); err != nil {
			buildErr = err.Error() + "\n" + string(out)
		}
	})
	if buildErr != "" {
		t.Fatalf("RED: go build ./cmd/evolve: %s", buildErr)
	}
	return buildBin
}

type treeFile struct {
	rel  string
	size int
}

func writeExecutableDir(t *testing.T, cache, rel string, files []treeFile, age time.Duration, now time.Time) string {
	t.Helper()
	root := filepath.Join(cache, filepath.FromSlash(rel))
	stamp := now.Add(-age)
	var dirs []string
	for _, f := range files {
		p := filepath.Join(root, filepath.FromSlash(f.rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(strings.Repeat("x", f.size)), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, stamp, stamp); err != nil {
			t.Fatal(err)
		}
		dirs = append(dirs, filepath.Dir(p))
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := os.Chtimes(dirs[i], stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chtimes(root, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeCacheFile(t *testing.T, cache, rel string, size int, age time.Duration, now time.Time) string {
	t.Helper()
	p := filepath.Join(cache, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(strings.Repeat("x", size)), 0o644); err != nil {
		t.Fatal(err)
	}
	stamp := now.Add(-age)
	if err := os.Chtimes(p, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	return p
}

func regularFileBytes(t *testing.T, root string) int64 {
	t.Helper()
	var total int64
	err := filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return total
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

func lsofListing(openPaths ...string) func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
	return func(_ context.Context, _, _ string, _, _ []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
		fmt.Fprint(stdout, "p4242\n")
		for _, p := range openPaths {
			fmt.Fprintf(stdout, "n%s\n", p)
		}
		return 0, nil
	}
}

var now = time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)

func TestC2606_001_ExecutableDirUnusedPastTheHorizonIsTrimmedAndItsBytesAreCounted(t *testing.T) {
	cache := t.TempDir()
	staleDir := writeExecutableDir(t, cache, "0a/0a01-d", []treeFile{{"exe", 300}, {"pkg/obj.o", 200}}, 30*time.Hour, now)
	freshFile := writeCacheFile(t, cache, "0b/0b01-a", 100, time.Hour, now)

	rep := gc.TrimGoCache(cache, gc.GoCacheBounds{Now: now, UnusedFor: 24 * time.Hour}, os.RemoveAll)

	if want := (gc.CacheTrimReport{Files: 1, Bytes: 500, RemainingBytes: 100}); !reflect.DeepEqual(rep, want) {
		t.Errorf("RED: report = %+v, want %+v: the stale executable dir is one trimmed entry of 500 bytes", rep, want)
	}
	if exists(staleDir) {
		t.Errorf("RED: executable dir %s unused past the horizon survived the trim", staleDir)
	}
	if !exists(freshFile) {
		t.Errorf("RED: the entry used an hour ago was trimmed")
	}
}

func TestC2606_002_ExecutableDirBytesCountInTheCapAndTheLeastRecentlyUsedGoesFirst(t *testing.T) {
	cache := t.TempDir()
	oldDir := writeExecutableDir(t, cache, "0a/0a01-d", []treeFile{{"exe", 300}, {"pkg/obj.o", 200}}, 20*time.Hour, now)
	newerFile := writeCacheFile(t, cache, "0b/0b01-a", 400, 10*time.Hour, now)

	rep := gc.TrimGoCache(cache, gc.GoCacheBounds{Now: now, MaxBytes: 600}, os.RemoveAll)

	if want := (gc.CacheTrimReport{CapFiles: 1, CapBytes: 500, RemainingBytes: 400}); !reflect.DeepEqual(rep, want) {
		t.Errorf("RED: report = %+v, want %+v: 900 bytes (500 in the dir) must shrink under 600 by taking the dir", rep, want)
	}
	if exists(oldDir) {
		t.Errorf("RED: the least recently used entry, executable dir %s, survived the cap", oldDir)
	}
	if !exists(newerFile) {
		t.Errorf("RED: the cap took the newer file after the dir already brought the cache under it")
	}
}

func TestC2606_003_ExecutableDirNewestInnerFileDecidesItsAge(t *testing.T) {
	cache := t.TempDir()
	staleOuterFreshInner := writeExecutableDir(t, cache, "0a/0a01-d", []treeFile{{"exe", 300}, {"pkg/obj.o", 200}}, 40*time.Hour, now)
	fresh := now.Add(-10 * time.Minute)
	if err := os.Chtimes(filepath.Join(staleOuterFreshInner, "pkg", "obj.o"), fresh, fresh); err != nil {
		t.Fatal(err)
	}
	insideWindow := writeExecutableDir(t, cache, "0b/0b01-d", []treeFile{{"exe", 500}}, 30*time.Minute, now)

	rep := gc.TrimGoCache(cache, gc.GoCacheBounds{Now: now, UnusedFor: 24 * time.Hour, MaxBytes: 100}, os.RemoveAll)

	if want := (gc.CacheTrimReport{RemainingBytes: 1000}); !reflect.DeepEqual(rep, want) {
		t.Errorf("RED: report = %+v, want %+v: a dir with a file used ten minutes ago, or inside the in-use window, is never trimmed, and its bytes stay counted", rep, want)
	}
	for _, p := range []string{staleOuterFreshInner, insideWindow} {
		if !exists(p) {
			t.Errorf("RED: %s was trimmed although a file inside it is in use", p)
		}
	}
}

func TestC2606_004_ExecutableDirWithAFileInTheHolderSnapshotStays(t *testing.T) {
	cache := t.TempDir()
	heldDir := writeExecutableDir(t, cache, "0a/0a01-d", []treeFile{{"exe", 300}, {"pkg/mapped.so", 200}}, 30*time.Hour, now)
	freeDir := writeExecutableDir(t, cache, "0b/0b01-d", []treeFile{{"exe", 250}}, 30*time.Hour, now)
	probe := gc.LiveHolderProbe(context.Background(), lsofListing(filepath.Join(heldDir, "pkg", "mapped.so")))

	rep := gc.TrimGoCache(cache, gc.GoCacheBounds{Now: now, UnusedFor: 24 * time.Hour}, os.RemoveAll, gc.SparingHeld(probe))

	if want := (gc.CacheTrimReport{Files: 1, Bytes: 250, RemainingBytes: 500}); !reflect.DeepEqual(rep, want) {
		t.Errorf("RED: report = %+v, want %+v: only the dir no process holds goes, and the held dir's 500 bytes remain", rep, want)
	}
	if !exists(filepath.Join(heldDir, "pkg", "mapped.so")) {
		t.Errorf("RED: the dir with a mapped file in the holder snapshot was trimmed")
	}
	if exists(freeDir) {
		t.Errorf("RED: the unheld stale dir %s survived: a holder must spare only the dir it holds", freeDir)
	}
}

func TestC2606_005_HeldExecutableDirIsSparedFromTheCapToo(t *testing.T) {
	cache := t.TempDir()
	heldDir := writeExecutableDir(t, cache, "0a/0a01-d", []treeFile{{"exe", 500}}, 20*time.Hour, now)
	writeCacheFile(t, cache, "0b/0b01-a", 400, 10*time.Hour, now)
	probe := gc.LiveHolderProbe(context.Background(), lsofListing(filepath.Join(heldDir, "exe")))

	rep := gc.TrimGoCache(cache, gc.GoCacheBounds{Now: now, MaxBytes: 600}, os.RemoveAll, gc.SparingHeld(probe))

	if !exists(heldDir) {
		t.Errorf("RED: the cap trimmed a dir a live process holds")
	}
	if rep.CapBytes > 400 || rep.RemainingBytes != 900-rep.CapBytes || len(rep.Errors) != 0 {
		t.Errorf("RED: report = %+v, want the held dir's 500 bytes never counted as trimmed and still remaining", rep)
	}
}

func TestC2606_006_UnknownLivenessSparesTheDirAndNamesTheError(t *testing.T) {
	cache := t.TempDir()
	staleDir := writeExecutableDir(t, cache, "0a/0a01-d", []treeFile{{"exe", 300}}, 30*time.Hour, now)
	unknown := func(string) (bool, error) { return false, fmt.Errorf("lsof unavailable") }

	rep := gc.TrimGoCache(cache, gc.GoCacheBounds{Now: now, UnusedFor: 24 * time.Hour}, os.RemoveAll, gc.SparingHeld(unknown))

	if !exists(staleDir) {
		t.Errorf("RED: a dir whose liveness is unknown was trimmed")
	}
	if rep.Files != 0 || rep.RemainingBytes != 300 || len(rep.Errors) != 1 {
		t.Errorf("RED: report = %+v, want nothing trimmed, 300 bytes remaining and one error naming the unknown liveness", rep)
	}
}

func TestC2606_007_ReportBytesAreTheSumOfTheTrimmedEntries(t *testing.T) {
	cache := t.TempDir()
	ageDirA := writeExecutableDir(t, cache, "0a/0a01-d", []treeFile{{"exe", 300}, {"pkg/obj.o", 200}}, 40*time.Hour, now)
	ageDirB := writeExecutableDir(t, cache, "0b/0b01-d", []treeFile{{"a/b/deep.o", 700}, {"top", 13}}, 35*time.Hour, now)
	ageFile := writeCacheFile(t, cache, "0c/0c01-a", 100, 30*time.Hour, now)
	capDir := writeExecutableDir(t, cache, "0d/0d01-d", []treeFile{{"exe", 450}, {"x/y", 55}}, 20*time.Hour, now)
	keptFile := writeCacheFile(t, cache, "0e/0e01-a", 400, 5*time.Hour, now)
	ageBytes := regularFileBytes(t, ageDirA) + regularFileBytes(t, ageDirB) + regularFileBytes(t, ageFile)
	capBytes := regularFileBytes(t, capDir)
	totalBefore := regularFileBytes(t, cache)

	rep := gc.TrimGoCache(cache, gc.GoCacheBounds{Now: now, UnusedFor: 24 * time.Hour, MaxBytes: 500}, os.RemoveAll)

	if rep.Files != 3 || rep.Bytes != ageBytes {
		t.Errorf("RED: age trim = %d files / %d bytes, want 3 files / %d bytes (the sum of the three trimmed entries)", rep.Files, rep.Bytes, ageBytes)
	}
	if rep.CapFiles != 1 || rep.CapBytes != capBytes {
		t.Errorf("RED: cap trim = %d files / %d bytes, want 1 file / %d bytes", rep.CapFiles, rep.CapBytes, capBytes)
	}
	if want := totalBefore - ageBytes - capBytes; rep.RemainingBytes != want {
		t.Errorf("RED: RemainingBytes = %d, want %d", rep.RemainingBytes, want)
	}
	if onDisk := regularFileBytes(t, cache); totalBefore-onDisk != rep.Bytes+rep.CapBytes {
		t.Errorf("RED: %d bytes left the disk, the report claims %d", totalBefore-onDisk, rep.Bytes+rep.CapBytes)
	}
	if !exists(keptFile) {
		t.Errorf("RED: the file inside the cap was trimmed")
	}
}

func TestC2606_008_PreviewCountsTheExecutableDirsAndRemovesNothing(t *testing.T) {
	cache := t.TempDir()
	staleDir := writeExecutableDir(t, cache, "0a/0a01-d", []treeFile{{"exe", 300}, {"pkg/obj.o", 200}}, 30*time.Hour, now)

	rep := gc.TrimGoCache(cache, gc.GoCacheBounds{Now: now, UnusedFor: 24 * time.Hour}, func(string) error { return nil })

	if rep.Files != 1 || rep.Bytes != 500 {
		t.Errorf("RED: preview = %+v, want the dir counted: 1 file, 500 bytes", rep)
	}
	if !exists(filepath.Join(staleDir, "pkg", "obj.o")) {
		t.Errorf("RED: the preview removed the dir")
	}
}

type gcProject struct {
	repo  *gittest.Repo
	cache string
	home  string
}

func newGCProject(t *testing.T) gcProject {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write("seed.txt", "seed\n")
	repo.Git("add", "seed.txt")
	repo.Git("commit", "-m", "seed")
	repo.Write(".evolve/policy.json", `{"gc":{"go_cache_ttl_hours":24}}`+"\n")
	home := t.TempDir()
	cache := filepath.Join(home, "gocache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	return gcProject{repo: repo, cache: cache, home: home}
}

func (p gcProject) run(t *testing.T) string {
	t.Helper()
	cmd := exec.Command(evolveBin(t), "gc", "--project-root", p.repo.Dir)
	cmd.Dir = p.repo.Dir
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "EVOLVE_") {
			env = append(env, kv)
		}
	}
	cmd.Env = append(env, "HOME="+p.home, "TMPDIR="+p.home, "GOCACHE="+p.cache, "TMUX_TMPDIR="+p.home, "GIT_TERMINAL_PROMPT=0")
	out, _ := cmd.CombinedOutput()
	return string(out)
}

func TestC2606_009_EvolveGcTrimsAStaleExecutableDirThroughTheCommand(t *testing.T) {
	p := newGCProject(t)
	realNow := time.Now()
	stale := writeExecutableDir(t, p.cache, "0a/0a01-d", []treeFile{{"exe", 300}, {"pkg/obj.o", 200}}, 30*time.Hour, realNow)
	fresh := writeExecutableDir(t, p.cache, "0b/0b01-d", []treeFile{{"exe", 100}}, 5*time.Hour, realNow)

	out := p.run(t)

	if exists(stale) {
		t.Errorf("RED: `evolve gc` left the stale executable dir %s:\n%s", stale, acsassert.OutputTail(out, 20))
	}
	if !exists(fresh) {
		t.Errorf("RED: `evolve gc` removed an executable dir used five hours ago")
	}
	if !strings.Contains(out, "trimmed 1 go build cache file(s)") {
		t.Errorf("RED: the report does not name the one trimmed entry:\n%s", acsassert.OutputTail(out, 20))
	}
}

func TestC2606_010_EvolveGcSparesAnExecutableDirALiveProcessHolds(t *testing.T) {
	if _, err := exec.LookPath("lsof"); err != nil {
		t.Skip("lsof is not installed")
	}
	p := newGCProject(t)
	realNow := time.Now()
	held := writeExecutableDir(t, p.cache, "0a/0a01-d", []treeFile{{"exe", 300}, {"pkg/mapped.so", 200}}, 30*time.Hour, realNow)
	free := writeExecutableDir(t, p.cache, "0b/0b01-d", []treeFile{{"exe", 100}}, 30*time.Hour, realNow)
	f, err := os.Open(filepath.Join(held, "pkg", "mapped.so"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	out := p.run(t)

	if !exists(filepath.Join(held, "pkg", "mapped.so")) {
		t.Errorf("RED: `evolve gc` trimmed a dir this live process holds open:\n%s", acsassert.OutputTail(out, 20))
	}
	if exists(free) {
		t.Errorf("RED: `evolve gc` left the stale dir nobody holds:\n%s", acsassert.OutputTail(out, 20))
	}
}
