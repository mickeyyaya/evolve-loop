package gc

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSeatSessionNamesAndRoot(t *testing.T) {
	if got := SeatSessionFolderName("/a/.b/c-d"); got != "-a--b-c-d" {
		t.Errorf("folder name = %q", got)
	}
	if root := DefaultSeatSessionRoot(); filepath.Dir(root) != "/tmp" || filepath.Base(root)[:7] != "claude-" {
		t.Errorf("default root = %q", root)
	}
}

func TestReapSeatSessions_AMissingRootIsNotAnError(t *testing.T) {
	rep := ReapSeatSessions(filepath.Join(t.TempDir(), "none"), t.TempDir(), time.Now(), os.RemoveAll)
	if rep.Entries != 0 || len(rep.Errors) != 0 {
		t.Errorf("report = %+v", rep)
	}
}

func TestReapSeatSessions_ARefusedRemovalIsReportedAndNotCounted(t *testing.T) {
	root, base := t.TempDir(), filepath.Join(t.TempDir(), "wt")
	dir := filepath.Join(root, SeatSessionFolderName(filepath.Join(base, "gone")))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(dir, old, old); err != nil {
		t.Fatal(err)
	}
	rep := ReapSeatSessions(root, base, time.Now().Add(-time.Hour), func(string) error { return os.ErrPermission })
	if rep.Entries != 0 || len(rep.Errors) != 1 {
		t.Errorf("report = %+v", rep)
	}
}

func seatDir(t *testing.T, root, name string, size int, age time.Duration) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(dir, "s"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "s", "f"), make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-age)
	for _, p := range []string{filepath.Join(dir, "s", "f"), filepath.Join(dir, "s"), dir} {
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestReapSeatSessions_RemovesOnlyTheStaleFolderOfARemovedWorktree(t *testing.T) {
	root, base := t.TempDir(), filepath.Join(t.TempDir(), "wt")
	if err := os.MkdirAll(filepath.Join(base, "live"), 0o755); err != nil {
		t.Fatal(err)
	}
	gone := seatDir(t, root, SeatSessionFolderName(filepath.Join(base, "gone")), 4096, 72*time.Hour)
	live := seatDir(t, root, SeatSessionFolderName(filepath.Join(base, "live")), 10, 72*time.Hour)
	liveChild := seatDir(t, root, SeatSessionFolderName(filepath.Join(base, "live"))+"-sub", 10, 72*time.Hour)
	young := seatDir(t, root, SeatSessionFolderName(filepath.Join(base, "young")), 10, time.Minute)
	foreign := seatDir(t, root, "unrelated", 10, 72*time.Hour)
	if err := os.WriteFile(filepath.Join(root, "plain-file"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	rep := ReapSeatSessions(root, base, time.Now().Add(-time.Hour), os.RemoveAll)

	if rep.Entries != 1 || rep.Bytes < 4096 || len(rep.Errors) != 0 {
		t.Errorf("report = %+v", rep)
	}
	if _, err := os.Stat(gone); !os.IsNotExist(err) {
		t.Errorf("the removed worktree folder survived: %v", err)
	}
	for _, kept := range []string{live, liveChild, young, foreign} {
		if _, err := os.Stat(kept); err != nil {
			t.Errorf("%s was removed: %v", kept, err)
		}
	}
}

func TestReapSeatSessions_ASymlinkedFolderIsKept(t *testing.T) {
	root, base := t.TempDir(), filepath.Join(t.TempDir(), "wt")
	target := seatDir(t, t.TempDir(), "t", 10, 72*time.Hour)
	link := filepath.Join(root, SeatSessionFolderName(filepath.Join(base, "gone")))
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	rep := ReapSeatSessions(root, base, time.Now().Add(-time.Hour), os.RemoveAll)

	if rep.Entries != 0 {
		t.Errorf("report = %+v", rep)
	}
	if _, err := os.Stat(target); err != nil {
		t.Errorf("the link target was removed: %v", err)
	}
}

func TestReapSeatSessions_ARootThatIsAFileIsReported(t *testing.T) {
	file := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	rep := ReapSeatSessions(file, t.TempDir(), time.Now(), os.RemoveAll)
	if len(rep.Errors) != 1 {
		t.Errorf("report = %+v", rep)
	}
}

func TestReapSeatSessions_AFolderWithAnUnreadableEntryIsKept(t *testing.T) {
	root, base := t.TempDir(), filepath.Join(t.TempDir(), "wt")
	dir := seatDir(t, root, SeatSessionFolderName(filepath.Join(base, "gone")), 10, 72*time.Hour)
	sub := filepath.Join(dir, "s")
	if err := os.Chmod(sub, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(sub, 0o755) })

	rep := ReapSeatSessions(root, base, time.Now().Add(-time.Hour), os.RemoveAll)

	if rep.Entries != 0 {
		t.Errorf("report = %+v", rep)
	}
}

func TestReapSeatDirectory_RemovesOnlyTheStaleFolderOfARemovedWorktreeUnderProjects(t *testing.T) {
	seatRoot, base := t.TempDir(), filepath.Join(t.TempDir(), "wt")
	projects := filepath.Join(seatRoot, "projects")
	if err := os.MkdirAll(filepath.Join(base, "live"), 0o755); err != nil {
		t.Fatal(err)
	}
	gone := seatDir(t, projects, SeatSessionFolderName(filepath.Join(base, "gone")), 4096, 72*time.Hour)
	live := seatDir(t, projects, SeatSessionFolderName(filepath.Join(base, "live")), 10, 72*time.Hour)

	rep := ReapSeatDirectory(seatRoot, base, time.Now().Add(-time.Hour), os.RemoveAll)

	if rep.Entries != 1 || rep.Bytes < 4096 || len(rep.Errors) != 0 {
		t.Errorf("report = %+v", rep)
	}
	if _, err := os.Stat(gone); !os.IsNotExist(err) {
		t.Errorf("the removed worktree folder survived: %v", err)
	}
	if _, err := os.Stat(live); err != nil {
		t.Errorf("live folder was removed: %v", err)
	}
}

func TestReapSeatDirectory_AnEmptyRootOrMissingProjectsReportsZero(t *testing.T) {
	base := filepath.Join(t.TempDir(), "wt")
	repEmpty := ReapSeatDirectory("", base, time.Now(), os.RemoveAll)
	if repEmpty.Entries != 0 || len(repEmpty.Errors) != 0 {
		t.Errorf("report empty = %+v", repEmpty)
	}
	repMissing := ReapSeatDirectory(t.TempDir(), base, time.Now(), os.RemoveAll)
	if repMissing.Entries != 0 || len(repMissing.Errors) != 0 {
		t.Errorf("report missing = %+v", repMissing)
	}
}

func TestReapSeatDirectory_AnUnreadableProjectsIsReported(t *testing.T) {
	seatRoot, base := t.TempDir(), filepath.Join(t.TempDir(), "wt")
	file := filepath.Join(seatRoot, "projects")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	rep := ReapSeatDirectory(seatRoot, base, time.Now(), os.RemoveAll)
	if len(rep.Errors) != 1 {
		t.Errorf("report = %+v", rep)
	}
}
