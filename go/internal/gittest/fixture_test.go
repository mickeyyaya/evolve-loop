package gittest

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// recordingTB captures Errorf so a teardown failure can be asserted on without
// failing the test that provokes it.
type recordingTB struct {
	testing.TB
	mu   sync.Mutex
	errs []string
}

func (r *recordingTB) Helper() {}

func (r *recordingTB) Errorf(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errs = append(r.errs, fmt.Sprintf(format, args...))
}

// lockedDir returns a directory RemoveAll cannot finish: a read-only
// subdirectory holding a file. unlock makes it removable again.
func lockedDir(t *testing.T) (dir string, unlock func()) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	dir = filepath.Join(t.TempDir(), "repo")
	sub := filepath.Join(dir, "held")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sub, 0o555); err != nil {
		t.Fatal(err)
	}
	unlock = func() { _ = os.Chmod(sub, 0o755) }
	t.Cleanup(unlock)
	return dir, unlock
}

func TestRemoveWithRetry_OutlastsATransientHold(t *testing.T) {
	dir, unlock := lockedDir(t)
	go func() {
		time.Sleep(3 * teardownBackoff)
		unlock()
	}()
	rec := &recordingTB{TB: t}
	removeWithRetry(rec, dir, teardownAttempts)
	if len(rec.errs) != 0 {
		t.Fatalf("a hold released mid-retry still failed the teardown: %v", rec.errs)
	}
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		t.Errorf("%s survived the retried removal (Lstat err=%v)", dir, err)
	}
}

func TestRemoveWithRetry_ExhaustionFailsWithPathAndDiagnostic(t *testing.T) {
	dir, _ := lockedDir(t)
	t.Setenv("PATH", t.TempDir()) // no lsof: the fallback must be stated
	rec := &recordingTB{TB: t}
	removeWithRetry(rec, dir, 2)
	if len(rec.errs) != 1 {
		t.Fatalf("exhausted teardown reported %d errors, want 1: %v", len(rec.errs), rec.errs)
	}
	msg := rec.errs[0]
	for _, want := range []string{dir, "after 2 attempts", "diagnostic unavailable"} {
		if !strings.Contains(msg, want) {
			t.Errorf("teardown failure lacks %q:\n%s", want, msg)
		}
	}
}

func TestParseLsof_DistinctCommandAndPID(t *testing.T) {
	out := "COMMAND   PID USER   FD   TYPE DEVICE SIZE/OFF NODE NAME\n" +
		"sleep   4242 me    cwd    DIR   1,16       64    9 /tmp/x/repo/held\n" +
		"git     7 me      3r   REG   1,16       10    8 /tmp/x/repo/.git/index\n" +
		"git     7 me      4r   REG   1,16       10    8 /tmp/x/repo/.git/HEAD\n"
	want := []string{"sleep (PID 4242)", "git (PID 7)"}
	if got := parseLsof(out); !reflect.DeepEqual(got, want) {
		t.Errorf("parseLsof = %v, want %v", got, want)
	}
	if got := parseLsof(""); len(got) != 0 {
		t.Errorf("parseLsof(\"\") = %v, want none", got)
	}
}
