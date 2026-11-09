package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

var seatNonAlnum = regexp.MustCompile(`[^A-Za-z0-9]`)

type seatSessionEnv struct {
	projectRoot string
	base        string
	seatRoot    string
	liveFolder  string
	goneFolder  string
	youngFolder string
	busyFolder  string
}

func seatFolderAt(t *testing.T, root, worktreePath string, age time.Duration) string {
	t.Helper()
	dir := filepath.Join(root, seatNonAlnum.ReplaceAllString(worktreePath, "-"))
	session := filepath.Join(dir, "0a1b2c3d-session", "tasks")
	if err := os.MkdirAll(session, 0o755); err != nil {
		t.Fatal(err)
	}
	big, err := os.Create(filepath.Join(session, "big.out"))
	if err != nil {
		t.Fatal(err)
	}
	if err := big.Truncate(1_500_000_000); err != nil {
		t.Fatal(err)
	}
	big.Close()
	old := time.Now().Add(-age)
	err = filepath.WalkDir(dir, func(p string, _ os.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		return os.Chtimes(p, old, old)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func newSeatSessionEnv(t *testing.T) seatSessionEnv {
	t.Helper()
	projectRoot, _, _ := gcWorktreeEnv(t, "")
	gcFakeGoCache(t)
	t.Setenv("TMPDIR", t.TempDir())
	resolved, err := filepath.EvalSymlinks(projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	seatRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(resolved, ".evolve", "worktrees")
	if err := os.MkdirAll(filepath.Join(base, "cycle-live"), 0o755); err != nil {
		t.Fatal(err)
	}
	e := seatSessionEnv{projectRoot: projectRoot, base: base, seatRoot: seatRoot}
	e.liveFolder = seatFolderAt(t, seatRoot, filepath.Join(base, "cycle-live"), 72*time.Hour)
	e.goneFolder = seatFolderAt(t, seatRoot, filepath.Join(base, "cycle-gone"), 72*time.Hour)
	e.youngFolder = seatFolderAt(t, seatRoot, filepath.Join(base, "cycle-young"), time.Hour)
	e.busyFolder = seatFolderAt(t, seatRoot, filepath.Join(base, "cycle-busy"), 72*time.Hour)
	if err := os.WriteFile(filepath.Join(e.busyFolder, "0a1b2c3d-session", "tasks", "live.out"), []byte("streaming"), 0o644); err != nil {
		t.Fatal(err)
	}
	gcWritePolicy(t, projectRoot, fmt.Sprintf(`{"temp_ttl_hours":24,"seat_session_root":%q}`, seatRoot))
	return e
}

func TestRunGC_RemovesTheSeatSessionFolderOfARemovedWorktreeAndReportsTheSpace(t *testing.T) {
	e := newSeatSessionEnv(t)

	var stdout, stderr bytes.Buffer
	runGC([]string{"--project-root", e.projectRoot}, nil, &stdout, &stderr)

	if _, err := os.Stat(e.goneFolder); !os.IsNotExist(err) {
		t.Errorf("the folder of a removed worktree survived `evolve gc`:\n%s\nstderr=%s", stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "removed 1 stale seat session folder(s) (1.50 GB") {
		t.Errorf("the report does not give the space released:\n%s", stdout.String())
	}
}

func TestRunGC_KeepsTheSeatSessionFolderOfALiveWorktreeAndOfALiveSession(t *testing.T) {
	e := newSeatSessionEnv(t)

	var stdout, stderr bytes.Buffer
	runGC([]string{"--project-root", e.projectRoot}, nil, &stdout, &stderr)

	if _, err := os.Stat(e.goneFolder); !os.IsNotExist(err) {
		t.Fatalf("the sweep removed nothing, so the keep checks below prove nothing:\n%s", stdout.String())
	}
	for name, dir := range map[string]string{"live worktree": e.liveFolder, "young": e.youngFolder, "live session": e.busyFolder} {
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("the %s folder was removed: %v", name, err)
		}
	}
}

func TestRunGC_DryRunPreviewsTheSeatSessionSweepAndRemovesNothing(t *testing.T) {
	e := newSeatSessionEnv(t)

	var stdout, stderr bytes.Buffer
	runGC([]string{"--dry-run", "--project-root", e.projectRoot}, nil, &stdout, &stderr)

	if !strings.Contains(stdout.String(), "1 stale seat session folder(s) would be removed (1.50 GB") {
		t.Errorf("--dry-run does not preview the folder:\n%s\nstderr=%s", stdout.String(), stderr.String())
	}
	if _, err := os.Stat(e.goneFolder); err != nil {
		t.Errorf("--dry-run removed %s", e.goneFolder)
	}
}

func TestRunGC_TheSeatSessionSweepIsOffWhenNoTTLIsSet(t *testing.T) {
	e := newSeatSessionEnv(t)
	gcWritePolicy(t, e.projectRoot, fmt.Sprintf(`{"seat_session_root":%q}`, e.seatRoot))

	var stdout, stderr bytes.Buffer
	runGC([]string{"--project-root", e.projectRoot}, nil, &stdout, &stderr)

	if !strings.Contains(stdout.String(), "seat session sweep off (gc.temp_ttl_hours unset)") {
		t.Errorf("gc does not say the seat session sweep is off:\n%s", stdout.String())
	}
	if _, err := os.Stat(e.goneFolder); err != nil {
		t.Errorf("a sweep with no age limit removed %s", e.goneFolder)
	}
}

func TestRunGC_AnUnreadableSeatSessionRootIsReportedOnStderr(t *testing.T) {
	e := newSeatSessionEnv(t)
	notADir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	gcWritePolicy(t, e.projectRoot, fmt.Sprintf(`{"temp_ttl_hours":24,"seat_session_root":%q}`, notADir))

	var stdout, stderr bytes.Buffer
	runGC([]string{"--project-root", e.projectRoot}, nil, &stdout, &stderr)

	if !strings.Contains(stderr.String(), "evolve gc: seat session error: read seat session root "+notADir) {
		t.Errorf("the unreadable root is not reported:\nstderr=%s", stderr.String())
	}
}

func TestRunGC_AnEmptySeatSessionRootPolicyNamesTheDefaultRootInTheReport(t *testing.T) {
	e := newSeatSessionEnv(t)
	gcWritePolicy(t, e.projectRoot, `{"temp_ttl_hours":24}`)

	var stdout, stderr bytes.Buffer
	runGC([]string{"--dry-run", "--project-root", e.projectRoot}, nil, &stdout, &stderr)

	want := fmt.Sprintf("0 stale seat session folder(s) would be removed (0.00 GB unused > 24h in /tmp/claude-%d)", os.Getuid())
	if !strings.Contains(stdout.String(), want) {
		t.Errorf("the default root is not named:\n%s", stdout.String())
	}
}

func TestRunGC_AnUnreadableSeatDirectoryProjectsIsReportedOnStderr(t *testing.T) {
	e := newSeatSessionEnv(t)
	telemetryDir := filepath.Join("/tmp", fmt.Sprintf("evtelemetry-%d", os.Getpid()))
	t.Setenv("TEST_TELEMETRY_DIR", telemetryDir)
	t.Cleanup(func() { _ = os.RemoveAll(telemetryDir) })
	home := t.TempDir()
	t.Setenv("HOME", home)
	seatRoot := filepath.Join(home, ".evolve-seat")
	if err := os.MkdirAll(seatRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	projects := filepath.Join(seatRoot, "projects")
	if err := os.WriteFile(projects, []byte("not-a-dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	runGC([]string{"--project-root", e.projectRoot}, nil, &stdout, &stderr)

	if !strings.Contains(stderr.String(), "evolve gc: seat directory error: ") {
		t.Errorf("the unreadable seat directory projects is not reported on stderr:\nstderr=%s", stderr.String())
	}
}
