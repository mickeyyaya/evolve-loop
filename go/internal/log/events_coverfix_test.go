package log

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmitAbnormal_AppendFailureNamesThePath(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "abnormal-events.jsonl")
	w := NewSidecarWriter(path)
	diskFull := errors.New("no space left on device")
	w.appendLine = func(*os.File, []byte) error { return diskFull }

	err := w.EmitAbnormal(Event{EventType: "x"})
	if !errors.Is(err, diskFull) {
		t.Fatalf("EmitAbnormal error = %v, want it to wrap the append failure", err)
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("EmitAbnormal error %q does not name the sidecar path %q", err, path)
	}
}
