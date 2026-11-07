//go:build acs

// Package cycle2213 encodes the acceptance of gc-misses-seat-session-temp-dirs.
package cycle2213

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

const (
	fixtureBytes  = 1234
	sparseBytes   = 1_500_000_000
	sparseSizeTxt = "1.50 GB"
	previewLine   = "stale seat session folder(s) would be removed"
)

var nonAlnum = regexp.MustCompile(`[^A-Za-z0-9]`)

var evolveBin string

var buildErr string

func TestMain(m *testing.M) {
	os.Exit(runWithBinary(m))
}

func runWithBinary(m *testing.M) int {
	dir, err := os.MkdirTemp("", "acs-cycle2213-bin-")
	if err != nil {
		buildErr = fmt.Sprintf("mktemp for evolve build: %v", err)
		return m.Run()
	}
	defer os.RemoveAll(dir)
	goMod, err := moduleRoot()
	if err != nil {
		buildErr = err.Error()
		return m.Run()
	}
	bin := filepath.Join(dir, "evolve-under-test")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/evolve")
	cmd.Dir = goMod
	if out, berr := cmd.CombinedOutput(); berr != nil {
		buildErr = fmt.Sprintf("go build ./cmd/evolve (dir=%s): %v\n%s", goMod, berr, out)
	} else {
		evolveBin = bin
	}
	return m.Run()
}

func moduleRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	dir := wd
	for i := 0; i < 8; i++ {
		if _, serr := os.Stat(filepath.Join(dir, "go.mod")); serr == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("no go.mod found walking up from %s", wd)
}

var referenceInstant = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

type seatFixture struct {
	t        *testing.T
	now      time.Time
	project  string
	base     string
	seatRoot string
}

func newCLIFixture(t *testing.T) seatFixture {
	t.Helper()
	f := newSeatFixtureIn(t, gittest.Fixture(t).Dir)
	f.now = time.Now()
	return f
}

func newSeatFixture(t *testing.T) seatFixture {
	t.Helper()
	return newSeatFixtureIn(t, t.TempDir())
}

func newSeatFixtureIn(t *testing.T, projectDir string) seatFixture {
	t.Helper()
	project, err := filepath.EvalSymlinks(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	seatRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(project, ".evolve", "worktrees")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	return seatFixture{t: t, now: referenceInstant, project: project, base: base, seatRoot: seatRoot}
}

func folderNameOf(path string) string {
	return nonAlnum.ReplaceAllString(path, "-")
}

func (f seatFixture) liveWorktree(name string) string {
	f.t.Helper()
	dir := filepath.Join(f.base, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package x\n"), 0o644); err != nil {
		f.t.Fatal(err)
	}
	return dir
}

func (f seatFixture) seatFolder(name string, age time.Duration) string {
	f.t.Helper()
	dir := filepath.Join(f.seatRoot, name)
	session := filepath.Join(dir, "0a1b2c3d-session", "tasks")
	if err := os.MkdirAll(session, 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(session, "out.txt"), bytes.Repeat([]byte("x"), fixtureBytes), 0o644); err != nil {
		f.t.Fatal(err)
	}
	f.age(dir, age)
	return dir
}

func (f seatFixture) age(dir string, age time.Duration) {
	f.t.Helper()
	old := f.now.Add(-age)
	err := filepath.WalkDir(dir, func(p string, _ os.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		return os.Chtimes(p, old, old)
	})
	if err != nil {
		f.t.Fatal(err)
	}
}

func (f seatFixture) sparse(dir string) {
	f.t.Helper()
	big, err := os.Create(filepath.Join(dir, "0a1b2c3d-session", "tasks", "big.out"))
	if err != nil {
		f.t.Fatal(err)
	}
	defer big.Close()
	if err := big.Truncate(sparseBytes); err != nil {
		f.t.Fatal(err)
	}
	f.age(dir, 48*time.Hour)
}

func (f seatFixture) removedWorktreeFolder(name string, age time.Duration) string {
	return f.seatFolder(folderNameOf(filepath.Join(f.base, name)), age)
}

func (f seatFixture) reap(cutoffAge time.Duration) gc.TempReapReport {
	return gc.ReapSeatSessions(f.seatRoot, f.base, f.now.Add(-cutoffAge), os.RemoveAll)
}

func present(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func TestC2213_001_ARemovedWorktreeFolderOverTheAgeLimitIsRemovedAndItsBytesCounted(t *testing.T) {
	f := newSeatFixture(t)
	gone := f.removedWorktreeFolder("cycle-gone-1", 48*time.Hour)

	rep := f.reap(24 * time.Hour)

	if present(gone) {
		t.Errorf("RED: the folder of a removed worktree over the age limit survived: %s", gone)
	}
	if want := (gc.TempReapReport{Entries: 1, Bytes: fixtureBytes}); fmt.Sprint(rep) != fmt.Sprint(want) {
		t.Errorf("RED: report = %+v, want %+v", rep, want)
	}
}

func TestC2213_002_AFolderOfALiveWorktreeIsKeptEvenWhenOld(t *testing.T) {
	f := newSeatFixture(t)
	f.liveWorktree("cycle-live-1")
	atRoot := f.seatFolder(folderNameOf(filepath.Join(f.base, "cycle-live-1")), 72*time.Hour)
	inSubdir := f.seatFolder(folderNameOf(filepath.Join(f.base, "cycle-live-1", "go")), 72*time.Hour)
	gone := f.removedWorktreeFolder("cycle-gone-2", 72*time.Hour)

	rep := f.reap(24 * time.Hour)

	for _, kept := range []string{atRoot, inSubdir} {
		if !present(kept) {
			t.Errorf("RED: the folder of a live worktree was removed: %s", kept)
		}
	}
	if present(gone) {
		t.Errorf("RED: the sweep removed nothing: the folder of the removed worktree survived: %s", gone)
	}
	if rep.Entries != 1 {
		t.Errorf("RED: entries = %d, want 1 (only the removed worktree)", rep.Entries)
	}
}

func TestC2213_003_AWorktreeWhoseNameStartsWithAnotherLiveNameIsNotKeptByThatPrefix(t *testing.T) {
	f := newSeatFixture(t)
	f.liveWorktree("cycle-1")
	gone := f.removedWorktreeFolder("cycle-10", 72*time.Hour)

	f.reap(24 * time.Hour)

	if present(gone) {
		t.Errorf("RED: cycle-10 was kept because cycle-1 is live: %s", gone)
	}
}

func TestC2213_004_AFolderWithAFreshFileInsideIsALiveSessionAndIsKept(t *testing.T) {
	f := newSeatFixture(t)
	busy := f.removedWorktreeFolder("cycle-gone-busy", 72*time.Hour)
	fresh := filepath.Join(busy, "0a1b2c3d-session", "tasks", "live.out")
	if err := os.WriteFile(fresh, []byte("streaming"), 0o644); err != nil {
		t.Fatal(err)
	}
	idle := f.removedWorktreeFolder("cycle-gone-idle", 72*time.Hour)

	rep := f.reap(24 * time.Hour)

	if !present(busy) {
		t.Errorf("RED: a session writing a fresh file was removed: %s", busy)
	}
	if present(idle) {
		t.Errorf("RED: the idle sibling survived, so the sweep removed nothing: %s", idle)
	}
	if rep.Entries != 1 {
		t.Errorf("RED: entries = %d, want 1", rep.Entries)
	}
}

func TestC2213_005_YoungForeignAndSymlinkedFoldersAreKept(t *testing.T) {
	f := newSeatFixture(t)
	young := f.removedWorktreeFolder("cycle-gone-young", time.Hour)
	foreign := f.seatFolder("-Users-someone-else-project", 72*time.Hour)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := f.now.Add(-72 * time.Hour)
	if err := os.Chtimes(outside, old, old); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(f.seatRoot, folderNameOf(filepath.Join(f.base, "cycle-gone-link")))
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	stale := f.removedWorktreeFolder("cycle-gone-stale", 72*time.Hour)

	rep := f.reap(24 * time.Hour)

	for _, kept := range []string{young, foreign, link, filepath.Join(outside, "keep.txt")} {
		if !present(kept) {
			t.Errorf("RED: %s was removed", kept)
		}
	}
	if present(stale) {
		t.Errorf("RED: the stale sibling survived, so the sweep removed nothing: %s", stale)
	}
	if rep.Entries != 1 {
		t.Errorf("RED: entries = %d, want 1", rep.Entries)
	}
}

func TestC2213_006_APreviewCountsTheFolderAndRemovesNothing(t *testing.T) {
	f := newSeatFixture(t)
	gone := f.removedWorktreeFolder("cycle-gone-preview", 72*time.Hour)

	rep := gc.ReapSeatSessions(f.seatRoot, f.base, f.now.Add(-24*time.Hour), func(string) error { return nil })

	if rep.Entries != 1 || rep.Bytes != fixtureBytes {
		t.Errorf("RED: preview report = %+v, want 1 entry of %d bytes", rep, fixtureBytes)
	}
	if !present(gone) {
		t.Errorf("preview removed %s", gone)
	}
}

func TestC2213_007_AnEmptyRootReportsNothing(t *testing.T) {
	f := newSeatFixture(t)

	rep := f.reap(24 * time.Hour)

	if rep.Entries != 0 || rep.Bytes != 0 || len(rep.Errors) != 0 {
		t.Errorf("RED: empty root report = %+v, want zero", rep)
	}
}

type cliRun struct {
	report []string
	stdout string
	stderr string
}

func gcDryRunJSON(t *testing.T, f seatFixture, ttlHours int) cliRun {
	t.Helper()
	if buildErr != "" {
		t.Fatalf("evolve CLI did not build from this worktree: %s", buildErr)
	}
	policy := fmt.Sprintf(`{"gc":{"temp_ttl_hours":%d,"seat_session_root":%q}}`, ttlHours, f.seatRoot)
	if err := os.WriteFile(filepath.Join(f.project, ".evolve", "policy.json"), []byte(policy), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(evolveBin, "gc", "--dry-run", "--json", "--project-root", f.project)
	cmd.Dir = f.project
	cmd.Env = append(os.Environ(), "TMPDIR="+t.TempDir(), "GOCACHE="+t.TempDir())
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	_ = cmd.Run()
	var parsed struct {
		Data struct {
			Report []string `json:"report"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		t.Fatalf("evolve gc --json printed no JSON: %v\nstdout=%s\nstderr=%s", err, stdout.String(), stderr.String())
	}
	return cliRun{report: parsed.Data.Report, stdout: stdout.String(), stderr: stderr.String()}
}

func (c cliRun) lineWith(needle string) string {
	for _, l := range c.report {
		if strings.Contains(l, needle) {
			return l
		}
	}
	return ""
}

func TestC2213_008_EvolveGCReportsTheSeatSessionBytesItWouldRelease(t *testing.T) {
	f := newCLIFixture(t)
	f.liveWorktree("cycle-live-cli")
	live := f.seatFolder(folderNameOf(filepath.Join(f.base, "cycle-live-cli")), 72*time.Hour)
	gone := f.removedWorktreeFolder("cycle-gone-cli", 72*time.Hour)
	f.sparse(gone)

	run := gcDryRunJSON(t, f, 24)

	line := run.lineWith(previewLine)
	if line == "" {
		t.Fatalf("RED: no report line says %q:\n%s\nstderr=%s", previewLine, run.stdout, run.stderr)
	}
	if !strings.Contains(line, "1 stale seat session folder(s)") {
		t.Errorf("RED: the live worktree folder was counted or the removed one was not: %q", line)
	}
	if !strings.Contains(line, sparseSizeTxt) {
		t.Errorf("RED: the line does not give the space released (%s): %q", sparseSizeTxt, line)
	}
	if !present(live) || !present(gone) {
		t.Errorf("preview removed a folder: live=%v gone=%v", present(live), present(gone))
	}
}

func TestC2213_009_TheAgeLimitComesFromTheGCPolicyTTL(t *testing.T) {
	f := newCLIFixture(t)
	gone := f.removedWorktreeFolder("cycle-gone-ttl", 48*time.Hour)
	f.sparse(gone)

	longTTL := gcDryRunJSON(t, f, 72)
	if l := longTTL.lineWith(previewLine); l != "" && strings.Contains(l, "1 stale seat session folder(s)") {
		t.Errorf("a 48h-old folder counted under a 72h limit: %q", l)
	}

	shortTTL := gcDryRunJSON(t, f, 24)
	if l := shortTTL.lineWith(previewLine); !strings.Contains(l, "1 stale seat session folder(s)") {
		t.Errorf("RED: a 48h-old folder not counted under a 24h limit: %q\n%s", l, shortTTL.stdout)
	}
}
