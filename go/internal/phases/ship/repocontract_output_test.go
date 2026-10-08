package ship

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
)

func TestRepoContractGate_StderrGetsTheNotesAndThePathWhileTheScanLogGetsTheRawStream(t *testing.T) {
	repo, goDir := importerFixture(t)
	mustWrite(t, filepath.Join(goDir, "internal", "base", "base.go"), "package base\n\nconst Stop = \"new-stop\"\n")
	ws := t.TempDir()
	var stderr strings.Builder

	err := runRepoContractGate(context.Background(), "enforce", repo, ws, &stderr)

	if err == nil || !strings.Contains(err.Error(), "TestUserContract") {
		t.Fatalf("the red importer still blocks the ship and names its test: %v", err)
	}
	scanPath := filepath.Join(ws, scanLogName)
	scan, readErr := os.ReadFile(scanPath)
	if readErr != nil {
		t.Fatalf("read the scan log: %v", readErr)
	}
	for _, raw := range []string{"=== RUN   TestUserContract", "--- FAIL: TestUserContract"} {
		if strings.Contains(stderr.String(), raw) {
			t.Errorf("stderr carries the raw test line %q; it belongs only in %s:\n%s", raw, scanLogName, stderr.String())
		}
		if !strings.Contains(string(scan), raw) {
			t.Errorf("the scan log lost the raw test line %q", raw)
		}
	}
	note := "[ship] repo-contract importer backstop: go test -json"
	if !strings.Contains(stderr.String(), note) || !strings.Contains(string(scan), note) {
		t.Errorf("the [ship] note %q must reach stderr and the scan log", note)
	}
	if pointer := "[ship] repo-contract gate: full output: " + scanPath + "\n"; !strings.Contains(stderr.String(), pointer) {
		t.Errorf("stderr does not end the gate with %q:\n%s", pointer, stderr.String())
	}
}

func TestRepoContractGate_WithoutARunDirTheRawStreamStaysOnStderr(t *testing.T) {
	repo, goDir := importerFixture(t)
	mustWrite(t, filepath.Join(goDir, "internal", "base", "base.go"), "package base\n\nconst Stop = \"new-stop\"\n")
	var stderr strings.Builder

	if err := runRepoContractGate(context.Background(), "enforce", repo, "", &stderr); err == nil {
		t.Fatal("the red importer must block the ship")
	}

	if !strings.Contains(stderr.String(), "--- FAIL: TestUserContract") {
		t.Errorf("with no run dir there is no scan log, so stderr is the only copy of the failing output:\n%s", stderr.String())
	}
	if strings.Contains(stderr.String(), "full output:") {
		t.Errorf("no scan log exists, so stderr must not point at one:\n%s", stderr.String())
	}
}

func TestScanLogName_IsAToolOutputFileThatACyclePassDeletes(t *testing.T) {
	if !slices.Contains(gcpolicy.ToolOutputFiles(), scanLogName) {
		t.Errorf("gcpolicy.ToolOutputFiles() = %v does not name %q, so a PASS seal keeps the scan log", gcpolicy.ToolOutputFiles(), scanLogName)
	}
}

type wrappedWriter struct{ w io.Writer }

func (w wrappedWriter) Write(p []byte) (int, error) { return w.w.Write(p) }

func TestRunGoTestJSON_AWrappedNotesWriterNeverGetsTheRawStream(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module packlog\n\ngo 1.23\n")
	mustWrite(t, filepath.Join(dir, "packlog_test.go"), "package packlog\n\nimport \"testing\"\n\nfunc TestChatty(t *testing.T) { t.Log(\"raw chatter\") }\n")
	var stderr, raw bytes.Buffer

	o := runRepoContractPackages(context.Background(), dir, packLog{notes: wrappedWriter{&stderr}, raw: &raw}, []string{"./..."})

	if !o.green() {
		t.Fatalf("precondition: the probe package must pass: err=%v failed=%v", o.err, o.failedNames())
	}
	for _, line := range []string{"=== RUN   TestChatty", "raw chatter"} {
		if strings.Contains(stderr.String(), line) {
			t.Errorf("the wrapped notes writer got the raw line %q:\n%s", line, stderr.String())
		}
		if !strings.Contains(raw.String(), line) {
			t.Errorf("the raw writer lost the line %q", line)
		}
	}
}
