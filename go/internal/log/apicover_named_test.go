package log

import (
	"path/filepath"
	"testing"
)

func TestSidecarWriter_NamedType(t *testing.T) {
	var w *SidecarWriter = NewSidecarWriter(filepath.Join(t.TempDir(), "abnormal-events.jsonl"))
	if w == nil {
		t.Fatal("NewSidecarWriter must return a non-nil *SidecarWriter")
	}
	if err := w.Close(); err != nil {
		t.Errorf("Close() = %v, want nil (documented no-op)", err)
	}
	if err := w.Close(); err != nil {
		t.Errorf("second Close() = %v, want nil (idempotent)", err)
	}
}
