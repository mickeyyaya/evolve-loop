package gc

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
)

func TestTailFiles_EverythingAfterDoubleDashIsAFile(t *testing.T) {
	t.Parallel()
	got := tailFiles([]string{"-n", "5", "a.log", "--", "-b.log", "--"})

	if want := []string{"a.log", "-b.log", "--"}; !reflect.DeepEqual(got, want) {
		t.Errorf("tailFiles = %q, want %q", got, want)
	}
}

func TestCollect_AnUnglobbableHomeIsSkipped(t *testing.T) {
	t.Parallel()
	evolveDir := filepath.Join(t.TempDir(), "a[")
	if err := os.MkdirAll(filepath.Join(evolveDir, "dispatch-logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "dispatch-logs", "x.log"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := logPlanner{}.collect(evolveDir, []gcpolicy.LogHome{{Dir: "dispatch-logs", Glob: "*.log"}}, "")

	if got != nil {
		t.Errorf("collect = %+v, want nothing from a home whose pattern does not parse", got)
	}
}

func TestLogEntryAt_AVanishedPathIsNoEntry(t *testing.T) {
	t.Parallel()
	for _, isDir := range []bool{false, true} {
		if e, ok := logEntryAt(filepath.Join(t.TempDir(), "gone.log"), isDir); ok || e != (logEntry{}) {
			t.Errorf("logEntryAt(gone, %v) = %+v, %v, want no entry", isDir, e, ok)
		}
	}
}
