package incidentkb_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/incidentkb"
)

func writeRecordFile(t *testing.T, root, id, doc string) string {
	t.Helper()
	path := incidentkb.RecordPath(root, id)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRenderRecord_WritesTheRenderedBytesAndASecondCallChangesNothing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := writeRecordFile(t, root, "2026-11-19-sample", sampleRecordDoc())
	if problems, _, err := incidentkb.Check(root, "2026-11-19-sample"); err != nil || len(problems) == 0 {
		t.Fatalf("control: an unrendered record must fail Check; problems=%q err=%v", problems, err)
	}
	if err := incidentkb.RenderRecord(root, "2026-11-19-sample"); err != nil {
		t.Fatalf("RenderRecord: %v", err)
	}
	first, _ := os.ReadFile(path)
	if problems, _, err := incidentkb.Check(root, "2026-11-19-sample"); err != nil || len(problems) != 0 {
		t.Fatalf("after RenderRecord, Check = %q err=%v, want none", problems, err)
	}
	if err := incidentkb.RenderRecord(root, "2026-11-19-sample"); err != nil {
		t.Fatalf("second RenderRecord: %v", err)
	}
	if second, _ := os.ReadFile(path); !bytes.Equal(first, second) {
		t.Errorf("a second RenderRecord changed the bytes")
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Errorf("RenderRecord left %d files in the incident directory, want only the record", len(entries))
	}
}

func TestRenderRecord_RefusesWhatItCannotRenderAndLeavesTheFileAlone(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := incidentkb.RenderRecord(root, "bad id"); err == nil {
		t.Errorf("an invalid id: err = nil, want a refusal")
	}
	outside := writeRecordFile(t, root, "REGRESSION-COVERAGE-INDEX", sampleRecordDoc())
	if err := incidentkb.RenderRecord(root, "REGRESSION-COVERAGE-INDEX"); err == nil {
		t.Errorf("an existing file outside the id namespace: err = nil, want a refusal")
	}
	if got, _ := os.ReadFile(outside); string(got) != sampleRecordDoc() {
		t.Errorf("a refused id changed the file")
	}
	if err := incidentkb.RenderRecord(root, "2026-11-19-absent"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a missing record: err = %v, want os.ErrNotExist", err)
	}
	noMarkers := strings.ReplaceAll(sampleRecordDoc(), "<!-- GENERATED:actions BEGIN -->\n<!-- GENERATED:actions END -->\n", "")
	path := writeRecordFile(t, root, "2026-11-19-sample", noMarkers)
	if err := incidentkb.RenderRecord(root, "2026-11-19-sample"); !errors.Is(err, incidentkb.ErrMissingMarker) {
		t.Errorf("a record without a marker: err = %v, want ErrMissingMarker", err)
	}
	if got, _ := os.ReadFile(path); string(got) != noMarkers {
		t.Errorf("a refused render changed the record")
	}
}

func TestRenderRecord_AWriteFailureIsReportedAndKeepsTheOriginal(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := writeRecordFile(t, root, "2026-11-19-sample", sampleRecordDoc())
	dir := filepath.Dir(path)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o750) })
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory modes")
	}
	if err := incidentkb.RenderRecord(root, "2026-11-19-sample"); err == nil {
		t.Fatalf("a read-only directory must make RenderRecord fail")
	}
	if got, _ := os.ReadFile(path); string(got) != sampleRecordDoc() {
		t.Errorf("a failed write changed the record")
	}
}
