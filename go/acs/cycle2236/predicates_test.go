//go:build acs

// Package cycle2236 encodes the acceptance of seat-login-l6-readers-read-the-seat-directory.
package cycle2236

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

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

const (
	seatDirLine     = "stale seat directory folder(s)"
	holderBody      = "sk-ant-oat01-ACS-HOLDER-FIXTURE"
	settingsBody    = `{"fixture":true}`
	journeyTestName = "TestSeatReadersJourney_"
)

var nonAlnum = regexp.MustCompile(`[^A-Za-z0-9]`)

var evolveBin string

var buildErr string

func TestMain(m *testing.M) {
	os.Exit(runWithBinary(m))
}

func runWithBinary(m *testing.M) int {
	dir, err := os.MkdirTemp("", "acs-cycle2236-bin-")
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

func repoFile(t *testing.T, rel string) string {
	t.Helper()
	goMod, err := moduleRoot()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(goMod), rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(raw)
}

func scratchDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "acs-cycle2236-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func present(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func folderNameOf(path string) string {
	return nonAlnum.ReplaceAllString(path, "-")
}

type seatHostFixture struct {
	t        *testing.T
	now      time.Time
	home     string
	project  string
	base     string
	seatRoot string
	tmp      string
	sessions string
}

func newSeatHostFixture(t *testing.T) seatHostFixture {
	t.Helper()
	project, err := filepath.EvalSymlinks(gittest.Fixture(t).Dir)
	if err != nil {
		t.Fatal(err)
	}
	home := scratchDir(t)
	f := seatHostFixture{
		t: t, now: time.Now(), home: home, project: project,
		base: filepath.Join(project, ".evolve", "worktrees"), seatRoot: paths.SeatRoot(home),
		tmp: scratchDir(t), sessions: scratchDir(t),
	}
	if err := os.MkdirAll(f.base, 0o755); err != nil {
		t.Fatal(err)
	}
	f.write(paths.SeatTokenPath(home), holderBody, 72*time.Hour)
	f.write(paths.SeatSettingsPath(home), settingsBody, 72*time.Hour)
	return f
}

func (f seatHostFixture) write(path, body string, age time.Duration) {
	f.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		f.t.Fatal(err)
	}
	f.age(path, age)
}

func (f seatHostFixture) age(path string, age time.Duration) {
	f.t.Helper()
	old := f.now.Add(-age)
	err := filepath.WalkDir(path, func(p string, _ os.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		return os.Chtimes(p, old, old)
	})
	if err != nil {
		f.t.Fatal(err)
	}
}

func (f seatHostFixture) liveWorktree(name string) {
	f.t.Helper()
	if err := os.MkdirAll(filepath.Join(f.base, name), 0o755); err != nil {
		f.t.Fatal(err)
	}
}

func (f seatHostFixture) projectsFolder(folder string, age time.Duration) string {
	f.t.Helper()
	dir := filepath.Join(f.seatRoot, "projects", folder)
	f.write(filepath.Join(dir, "0a1b2c3d.jsonl"), `{"type":"user"}`+"\n", age)
	f.age(dir, age)
	return dir
}

func (f seatHostFixture) worktreeFolder(name string, age time.Duration) string {
	return f.projectsFolder(folderNameOf(filepath.Join(f.base, name)), age)
}

func (f seatHostFixture) markRunLive() {
	f.t.Helper()
	runDir := filepath.Join(f.project, ".evolve", "runs", "cycle-acs-live")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: "acs-live", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		f.t.Fatal(err)
	}
}

type gcResult struct {
	report []string
	stdout string
	stderr string
	code   int
}

func (f seatHostFixture) gc(ttlHours int, dryRun bool) gcResult {
	f.t.Helper()
	if buildErr != "" {
		f.t.Fatalf("evolve CLI did not build from this worktree: %s", buildErr)
	}
	policy := fmt.Sprintf(`{"gc":{"temp_ttl_hours":%d,"seat_session_root":%q}}`, ttlHours, f.sessions)
	if err := os.WriteFile(filepath.Join(f.project, ".evolve", "policy.json"), []byte(policy), 0o644); err != nil {
		f.t.Fatal(err)
	}
	args := []string{"gc", "--json", "--project-root", f.project}
	if dryRun {
		args = append(args, "--dry-run")
	} else {
		f.markRunLive()
	}
	cmd := exec.Command(evolveBin, args...)
	cmd.Dir = f.project
	cmd.Env = append(os.Environ(), "HOME="+f.home, "TMPDIR="+f.tmp, "GOCACHE="+f.tmp, "CLAUDE_CONFIG_DIR=")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	code := 0
	if ee, ok := runErr.(*exec.ExitError); ok {
		code = ee.ExitCode()
	}
	var parsed struct {
		Data struct {
			Report []string `json:"report"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		f.t.Fatalf("evolve gc --json printed no JSON: %v\nstdout=%s\nstderr=%s", err, stdout.String(), stderr.String())
	}
	return gcResult{report: parsed.Data.Report, stdout: stdout.String(), stderr: stderr.String(), code: code}
}

func (g gcResult) lineWith(needle string) string {
	for _, l := range g.report {
		if strings.Contains(l, needle) {
			return l
		}
	}
	return ""
}

func TestC2236_001_ASeatOnTheTokenFileIsFoundBySpendAndPostMortemUnderTheSeatRoot(t *testing.T) {
	goMod, err := moduleRoot()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", "-count=1", "-v", "-run", "^"+journeyTestName, "./internal/bridge")
	cmd.Dir = goMod
	out, runErr := cmd.CombinedOutput()
	text := string(out)
	if runErr != nil {
		t.Fatalf("RED: the seat readers journey failed: %v\n%s", runErr, text)
	}
	for _, name := range []string{
		"TestSeatReadersJourney_ASeatOnTheTokenFileIsFoundBySpendAndPostMortemUnderTheSeatRoot",
		"TestSeatReadersJourney_AStoredLoginSeatStillGivesTheSameSpendAndPostMortem",
	} {
		if !strings.Contains(text, "--- PASS: "+name) {
			t.Errorf("RED: %s did not run and pass:\n%s", name, text)
		}
	}
}

func TestC2236_002_GCRemovesOnlyTheStaleFolderOfARemovedWorktreeAndSparesAnyLiveSeatAndTheHolder(t *testing.T) {
	f := newSeatHostFixture(t)
	f.liveWorktree("cycle-live-1")
	gone := f.worktreeFolder("cycle-gone-1", 72*time.Hour)
	liveRoot := f.worktreeFolder("cycle-live-1", 72*time.Hour)
	liveChild := f.projectsFolder(folderNameOf(filepath.Join(f.base, "cycle-live-1", "go")), 72*time.Hour)
	young := f.worktreeFolder("cycle-gone-young", time.Hour)
	busy := f.worktreeFolder("cycle-gone-busy", 72*time.Hour)
	f.write(filepath.Join(busy, "streaming.jsonl"), "fresh\n", 0)
	foreign := f.projectsFolder("-Users-someone-else-project", 72*time.Hour)
	outside := scratchDir(t)
	f.write(filepath.Join(outside, "keep.txt"), "keep", 72*time.Hour)
	link := filepath.Join(f.seatRoot, "projects", folderNameOf(filepath.Join(f.base, "cycle-gone-link")))
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	plugins := filepath.Join(f.seatRoot, "plugins")
	f.write(filepath.Join(plugins, "installed.json"), "{}", 72*time.Hour)
	f.age(plugins, 72*time.Hour)

	f.gc(24, false)

	if present(gone) {
		t.Errorf("RED: the folder of a removed worktree over the age limit survived: %s", gone)
	}
	for _, kept := range []string{liveRoot, liveChild, young, busy, foreign, link, filepath.Join(outside, "keep.txt"), plugins} {
		if !present(kept) {
			t.Errorf("RED: gc removed state that a live or foreign seat owns: %s", kept)
		}
	}
	token, err := os.ReadFile(paths.SeatTokenPath(f.home))
	if err != nil || string(token) != holderBody {
		t.Errorf("RED: the token holder changed or vanished: %q %v", token, err)
	}
	settings, err := os.ReadFile(paths.SeatSettingsPath(f.home))
	if err != nil || string(settings) != settingsBody {
		t.Errorf("RED: the seat settings changed or vanished: %q %v", settings, err)
	}
}

func TestC2236_003_ADryRunCountsOnlyTheRemovedWorktreeFolderAndRemovesNothing(t *testing.T) {
	f := newSeatHostFixture(t)
	f.liveWorktree("cycle-live-2")
	live := f.worktreeFolder("cycle-live-2", 72*time.Hour)
	gone := f.worktreeFolder("cycle-gone-2", 72*time.Hour)

	run := f.gc(24, true)

	line := run.lineWith(seatDirLine)
	if line == "" {
		t.Fatalf("RED: no report line says %q:\n%s\nstderr=%s", seatDirLine, run.stdout, run.stderr)
	}
	if !strings.Contains(line, "1 "+seatDirLine) {
		t.Errorf("RED: the line must count exactly the removed worktree folder: %q", line)
	}
	if !present(live) || !present(gone) {
		t.Errorf("a preview removed a folder: live=%v gone=%v", present(live), present(gone))
	}
}

func TestC2236_004_TheAgeLimitComesFromTheGCPolicyTTL(t *testing.T) {
	f := newSeatHostFixture(t)
	f.worktreeFolder("cycle-gone-ttl", 48*time.Hour)

	long := f.gc(72, true)
	if l := long.lineWith(seatDirLine); l == "" || !strings.Contains(l, "0 "+seatDirLine) {
		t.Errorf("RED: a 48h-old folder must not count under a 72h limit, and the step must still report: %q\n%s", l, long.stdout)
	}
	short := f.gc(24, true)
	if l := short.lineWith(seatDirLine); !strings.Contains(l, "1 "+seatDirLine) {
		t.Errorf("RED: a 48h-old folder must count under a 24h limit: %q\n%s", l, short.stdout)
	}
}

func TestC2236_005_AHostWithNoSeatDirectoryReportsZeroAndGCCreatesNothing(t *testing.T) {
	f := newSeatHostFixture(t)
	if err := os.RemoveAll(f.seatRoot); err != nil {
		t.Fatal(err)
	}

	run := f.gc(24, true)

	if l := run.lineWith(seatDirLine); l == "" || !strings.Contains(l, "0 "+seatDirLine) {
		t.Errorf("RED: the seat directory step must report zero on a host with no seat directory: %q\n%s", l, run.stdout)
	}
	if strings.Contains(run.stderr, "seat directory error") {
		t.Errorf("a missing seat directory is not an error: %s", run.stderr)
	}
	if present(f.seatRoot) {
		t.Errorf("gc created the seat directory %s", f.seatRoot)
	}
}

func TestC2236_006_ThePlanRowL6AndItsStepNoLongerSayTheReadersAreMissing(t *testing.T) {
	// acs-predicate: config-check
	plan := repoFile(t, "docs/plans/seat-login-isolation-2026-11.md")
	stale := regexp.MustCompile(`(?i)(set must wait|verbs do not yet have the readers|gc and the usage probe read no transcript, so they do not change)`)
	if m := stale.FindString(plan); m != "" {
		t.Errorf("RED: the plan still says %q", m)
	}
	var row string
	for _, line := range strings.Split(plan, "\n") {
		if strings.HasPrefix(line, "| L6 |") {
			row = line
		}
	}
	if row == "" {
		t.Fatal("the plan has no L6 row")
	}
	if !regexp.MustCompile(`(?i)\b(built|landed)\b`).MatchString(row) {
		t.Errorf("RED: the L6 row does not state that the readers are built: %q", row)
	}
	if !strings.Contains(strings.ToLower(row), "gc") {
		t.Errorf("RED: the L6 row does not name gc among the readers: %q", row)
	}
}

func TestC2236_007_TheAuthUsageTextDoesNotSaySetMustWait(t *testing.T) {
	if buildErr != "" {
		t.Fatalf("evolve CLI did not build from this worktree: %s", buildErr)
	}
	cmd := exec.Command(evolveBin, "auth", "--help")
	cmd.Dir = t.TempDir()
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		t.Fatalf("evolve auth --help: %v", err)
	}
	usage := stdout.String()
	if !strings.Contains(usage, "seat-token set") {
		t.Fatalf("the usage does not describe seat-token set:\n%s", usage)
	}
	if m := regexp.MustCompile(`(?i)(must wait|not yet|until the readers|wait for the readers|before the readers)`).FindString(usage); m != "" {
		t.Errorf("the usage still tells the operator to wait: %q\n%s", m, usage)
	}
	if !strings.Contains(usage, "seat root") {
		t.Errorf("the usage does not say that the readers read the seat root:\n%s", usage)
	}
}

func TestC2236_008_TheGCPackageNoteRecordsWhySeatDirectoryStateIsOwned(t *testing.T) {
	// acs-predicate: config-check
	note := repoFile(t, "docs/architecture/packages/internal-gc.md")
	if !regexp.MustCompile(`(?i)seat directory`).MatchString(note) {
		t.Errorf("RED: the gc package note does not record the seat directory sweep")
	}
	if !strings.Contains(note, "paths.SeatRoot") && !strings.Contains(note, ".evolve-seat") {
		t.Errorf("RED: the gc package note does not name the seat directory path")
	}
}
