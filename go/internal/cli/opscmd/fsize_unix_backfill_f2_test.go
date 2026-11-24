//go:build unix

package opscmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/testhost"
)

const (
	leaseHelperDirEnv = "EVOLVE_TEST_LEASE_HELPER_DIR"
	leaseHelperMarker = "LEASE-RESULT "
	leaseHelperRun    = "^TestHelperWriteConsoleLeaseWithNoFileGrowth$"
)

type leaseHelperResult struct {
	RC           int  `json:"rc"`
	StdoutEmpty  bool `json:"stdout_empty"`
	StderrPrefix bool `json:"stderr_prefix"`
}

func TestHelperWriteConsoleLeaseWithNoFileGrowth(t *testing.T) {
	dir := os.Getenv(leaseHelperDirEnv)
	if dir == "" {
		t.Skip("helper process body; run via TestWriteConsoleLease_AFailedWriteIsReportedAndLeavesNoLeaseOrTempFileF2")
	}
	var old syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &old); err != nil {
		t.Fatalf("no RLIMIT_FSIZE: %v", err)
	}
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Cur: 0, Max: old.Max}); err != nil {
		t.Fatalf("cannot lower RLIMIT_FSIZE: %v", err)
	}
	t.Cleanup(func() { _ = syscall.Setrlimit(syscall.RLIMIT_FSIZE, &old) })
	var out, errb bytes.Buffer
	rc := writeConsoleLease(filepath.Join(dir, "lease.json"), []string{"a.md"}, "r", time.Minute, &out, &errb)
	body, err := json.Marshal(leaseHelperResult{RC: rc, StdoutEmpty: out.Len() == 0, StderrPrefix: strings.HasPrefix(errb.String(), "evolve console-lease: ")})
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout.WriteString(leaseHelperMarker + string(body) + "\n")
}

func TestWriteConsoleLease_AFailedWriteIsReportedAndLeavesNoLeaseOrTempFileF2(t *testing.T) {
	dir := t.TempDir()
	raw, err := testhost.Command(leaseHelperRun, leaseHelperDirEnv+"="+dir).Output()
	if err != nil {
		t.Fatalf("the helper process failed: %v\n%s", err, raw)
	}
	var got leaseHelperResult
	for _, line := range strings.Split(string(raw), "\n") {
		if body, ok := strings.CutPrefix(line, leaseHelperMarker); ok {
			if err := json.Unmarshal([]byte(body), &got); err != nil {
				t.Fatalf("bad helper result %q: %v", body, err)
			}
			break
		}
	}
	entries, readErr := os.ReadDir(dir)
	if got.RC != 1 || !got.StdoutEmpty || !got.StderrPrefix || readErr != nil || len(entries) != 0 {
		t.Errorf("result %+v, dir %v (%v), want exit 1, no stdout, the error prefix and an empty dir", got, entries, readErr)
	}
}
