//go:build acs

package cycle2613

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
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var now = time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)

var longUnused = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

func writeTree(t *testing.T, cache, rel string, files map[string]int, stamp time.Time) string {
	t.Helper()
	root := filepath.Join(cache, filepath.FromSlash(rel))
	for name, size := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(strings.Repeat("x", size)), 0o644); err != nil {
			t.Fatal(err)
		}
		if stamp.IsZero() {
			continue
		}
		if err := os.Chtimes(p, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	return root
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

func TestC2613_001_ExecutableDirUnusedPastTheHorizonIsTrimmedAndItsBytesCountInTheCap(t *testing.T) {
	cache := t.TempDir()
	stale := writeTree(t, cache, "0a/0a01-d", map[string]int{"exe": 300, "pkg/obj.o": 200}, now.Add(-30*time.Hour))
	fresh := writeTree(t, cache, "0b/0b01-d", map[string]int{"exe": 100}, now.Add(-time.Hour))

	rep := gc.TrimGoCache(cache, gc.GoCacheBounds{Now: now, UnusedFor: 24 * time.Hour}, os.RemoveAll)

	if rep.Files != 1 || rep.Bytes != 500 || rep.RemainingBytes != 100 || exists(stale) || !exists(fresh) {
		t.Errorf("RED: report = %+v, stale present = %v, fresh present = %v, want 1 file / 500 bytes, 100 remaining, stale gone", rep, exists(stale), exists(fresh))
	}
}

func TestC2613_002_ExecutableDirBytesCountInTheCapAndTheLeastRecentlyUsedEntryGoesFirst(t *testing.T) {
	cache := t.TempDir()
	oldDir := writeTree(t, cache, "0a/0a01-d", map[string]int{"exe": 300, "pkg/obj.o": 200}, now.Add(-20*time.Hour))
	newer := writeTree(t, cache, "0b/0b01-d", map[string]int{"exe": 400}, now.Add(-10*time.Hour))

	rep := gc.TrimGoCache(cache, gc.GoCacheBounds{Now: now, MaxBytes: 600}, os.RemoveAll)

	if want := (gc.CacheTrimReport{CapFiles: 1, CapBytes: 500, RemainingBytes: 400}); !reflect.DeepEqual(rep, want) {
		t.Errorf("RED: report = %+v, want %+v: 900 bytes shrink under 600 by taking the older dir", rep, want)
	}
	if exists(oldDir) || !exists(newer) {
		t.Errorf("RED: older dir present = %v, newer dir present = %v, want the older gone and the newer kept", exists(oldDir), exists(newer))
	}
}

func TestC2613_003_ExecutableDirWithAFileInTheHolderSnapshotStaysAndTheFreeDirGoes(t *testing.T) {
	cache := t.TempDir()
	stamp := now.Add(-30 * time.Hour)
	heldDir := writeTree(t, cache, "0a/0a01-d", map[string]int{"exe": 300, "pkg/mapped.so": 200}, stamp)
	freeDir := writeTree(t, cache, "0b/0b01-d", map[string]int{"exe": 250}, stamp)
	probe := gc.LiveHolderProbe(context.Background(), lsofListing(filepath.Join(heldDir, "pkg", "mapped.so")))

	rep := gc.TrimGoCache(cache, gc.GoCacheBounds{Now: now, UnusedFor: 24 * time.Hour}, os.RemoveAll, gc.SparingHeld(probe))

	if want := (gc.CacheTrimReport{Files: 1, Bytes: 250, RemainingBytes: 500}); !reflect.DeepEqual(rep, want) {
		t.Errorf("RED: report = %+v, want %+v: the held dir's 500 bytes remain", rep, want)
	}
	if !exists(filepath.Join(heldDir, "pkg", "mapped.so")) || exists(freeDir) {
		t.Errorf("RED: held dir present = %v, free dir present = %v, want the held dir kept and the free dir gone", exists(heldDir), exists(freeDir))
	}
}

func TestC2613_004_HeldExecutableDirIsSparedFromTheCapAndNeverCountedAsTrimmed(t *testing.T) {
	cache := t.TempDir()
	heldDir := writeTree(t, cache, "0a/0a01-d", map[string]int{"exe": 500}, now.Add(-20*time.Hour))
	writeTree(t, cache, "0b/0b01-d", map[string]int{"exe": 400}, now.Add(-10*time.Hour))
	probe := gc.LiveHolderProbe(context.Background(), lsofListing(filepath.Join(heldDir, "exe")))

	rep := gc.TrimGoCache(cache, gc.GoCacheBounds{Now: now, MaxBytes: 600}, os.RemoveAll, gc.SparingHeld(probe))

	if !exists(heldDir) || rep.CapBytes != 400 || rep.RemainingBytes != 500 {
		t.Errorf("RED: held dir present = %v, report = %+v, want the held dir kept, 400 cap bytes trimmed, 500 remaining", exists(heldDir), rep)
	}
}

func TestC2613_005_UnknownLivenessSparesTheDirAndReportsTheError(t *testing.T) {
	cache := t.TempDir()
	stale := writeTree(t, cache, "0a/0a01-d", map[string]int{"exe": 300}, now.Add(-30*time.Hour))
	unknown := func(string) (bool, error) { return false, fmt.Errorf("holder snapshot unavailable") }

	rep := gc.TrimGoCache(cache, gc.GoCacheBounds{Now: now, UnusedFor: 24 * time.Hour}, os.RemoveAll, gc.SparingHeld(unknown))

	if !exists(stale) || rep.Files != 0 || rep.RemainingBytes != 300 || len(rep.Errors) != 1 {
		t.Errorf("RED: dir present = %v, report = %+v, want nothing trimmed, 300 bytes remaining, one error", exists(stale), rep)
	}
}

func TestC2613_006_ReportBytesEqualTheSumOfTheTrimmedEntries(t *testing.T) {
	cache := t.TempDir()
	ageA := writeTree(t, cache, "0a/0a01-d", map[string]int{"exe": 300, "pkg/obj.o": 200}, now.Add(-40*time.Hour))
	ageB := writeTree(t, cache, "0b/0b01-d", map[string]int{"a/b/deep.o": 700, "top": 13}, now.Add(-35*time.Hour))
	capDir := writeTree(t, cache, "0d/0d01-d", map[string]int{"exe": 450, "x/y": 55}, now.Add(-20*time.Hour))
	kept := writeTree(t, cache, "0e/0e01-d", map[string]int{"exe": 400}, now.Add(-5*time.Hour))
	ageBytes := regularFileBytes(t, ageA) + regularFileBytes(t, ageB)
	capBytes := regularFileBytes(t, capDir)
	totalBefore := regularFileBytes(t, cache)

	rep := gc.TrimGoCache(cache, gc.GoCacheBounds{Now: now, UnusedFor: 24 * time.Hour, MaxBytes: 500}, os.RemoveAll)

	if rep.Files != 2 || rep.Bytes != ageBytes || rep.CapFiles != 1 || rep.CapBytes != capBytes {
		t.Errorf("RED: report = %+v, want 2 files / %d bytes by age and 1 file / %d bytes by cap", rep, ageBytes, capBytes)
	}
	if onDisk := regularFileBytes(t, cache); totalBefore-onDisk != rep.Bytes+rep.CapBytes || rep.RemainingBytes != onDisk {
		t.Errorf("RED: %d bytes left the disk, report claims %d, remaining %d vs on disk %d", totalBefore-onDisk, rep.Bytes+rep.CapBytes, rep.RemainingBytes, onDisk)
	}
	if !exists(kept) {
		t.Errorf("RED: the dir inside the cap was trimmed")
	}
}

type gcProject struct {
	repo  *gittest.Repo
	cache string
	home  string
	bin   string
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
	bin := filepath.Join(home, "evolve")
	build := exec.Command("go", "build", "-o", bin, "./cmd/evolve")
	build.Dir = acsassert.GoModuleDir(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("RED: go build ./cmd/evolve: %v\n%s", err, acsassert.OutputTail(string(out), 20))
	}
	return gcProject{repo: repo, cache: cache, home: home, bin: bin}
}

func (p gcProject) run(t *testing.T) string {
	t.Helper()
	cmd := exec.Command(p.bin, "gc", "--project-root", p.repo.Dir)
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

func TestC2613_007_EvolveGcTrimsAStaleExecutableDirAndNamesTheTrimmedEntry(t *testing.T) {
	p := newGCProject(t)
	stale := writeTree(t, p.cache, "0a/0a01-d", map[string]int{"exe": 300, "pkg/obj.o": 200}, longUnused)
	fresh := writeTree(t, p.cache, "0b/0b01-d", map[string]int{"exe": 100}, time.Time{})

	out := p.run(t)

	if exists(stale) || !exists(fresh) || !strings.Contains(out, "trimmed 1 go build cache file(s)") {
		t.Errorf("RED: stale present = %v, fresh present = %v, output:\n%s", exists(stale), exists(fresh), acsassert.OutputTail(out, 20))
	}
}

func TestC2613_008_EvolveGcSparesAnExecutableDirALiveProcessHolds(t *testing.T) {
	if _, err := exec.LookPath("lsof"); err != nil {
		t.Fatalf("RED: lsof is required for the live holder probe: %v", err)
	}
	p := newGCProject(t)
	held := writeTree(t, p.cache, "0a/0a01-d", map[string]int{"exe": 300, "pkg/mapped.so": 200}, longUnused)
	free := writeTree(t, p.cache, "0b/0b01-d", map[string]int{"exe": 100}, longUnused)
	f, err := os.Open(filepath.Join(held, "pkg", "mapped.so"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	out := p.run(t)

	if !exists(filepath.Join(held, "pkg", "mapped.so")) || exists(free) {
		t.Errorf("RED: held file present = %v, free dir present = %v, output:\n%s", exists(held), exists(free), acsassert.OutputTail(out, 20))
	}
}
