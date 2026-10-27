package log

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
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

func TestSelectionFactory_NamedExports(t *testing.T) {
	var sel Selection = Selection{Run: "r"}
	var buf bytes.Buffer
	var sinks Sinks = Sinks{JSON: &buf, Level: "info"}
	NewSelectionLogger(sinks).InfoContext(WithSelection(context.Background(), sel), "named")
	if !strings.Contains(buf.String(), `"run":"r"`) {
		t.Errorf("selection run missing: %q", buf.String())
	}
}
