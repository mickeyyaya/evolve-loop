package log

import (
	"path/filepath"
	"testing"
)

// TestSidecarWriter_NamedType names the concrete log.SidecarWriter type
// (NewSidecarWriter returns *SidecarWriter but the bare type is never named in a
// test) and pins that the constructor hands back a non-nil writer whose
// documented no-op Close returns nil and is idempotent.
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
