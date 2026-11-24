//go:build unix

package commentaudit

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/testhost"
)

const (
	fsfullHelperModeEnv = "EVOLVE_TEST_FSFULL_HELPER_MODE"
	fsfullHelperDirEnv  = "EVOLVE_TEST_FSFULL_HELPER_DIR"
	fsfullHelperRun     = "^TestHelperFileWriteWithNoFileGrowth$"
	fsfullAppend        = "append"
	fsfullReplace       = "replace"
	fsfullMarker        = "FSFULL-RESULT "
	fsfullPageBody      = "package a\n// c\n"
	kindNoError         = "none"
	kindFileTooLarge    = "file-too-large"
	kindOtherError      = "other"
)

type fsfullResult struct {
	Kind        string `json:"kind"`
	FailedInDir string `json:"failed_in_dir"`
}

func classifyWriteError(err error) fsfullResult {
	var pathErr *fs.PathError
	switch {
	case err == nil:
		return fsfullResult{Kind: kindNoError}
	case errors.Is(err, syscall.EFBIG) && errors.As(err, &pathErr):
		return fsfullResult{Kind: kindFileTooLarge, FailedInDir: filepath.Dir(pathErr.Path)}
	}
	return fsfullResult{Kind: kindOtherError}
}

func TestHelperFileWriteWithNoFileGrowth(t *testing.T) {
	mode, dir := os.Getenv(fsfullHelperModeEnv), os.Getenv(fsfullHelperDirEnv)
	if mode == "" || dir == "" {
		t.Skip("helper process body; run via the F2 tests of appendFile and replaceFile")
	}
	var old syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &old); err != nil {
		t.Fatalf("no RLIMIT_FSIZE: %v", err)
	}
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Cur: 0, Max: old.Max}); err != nil {
		t.Fatalf("cannot lower RLIMIT_FSIZE: %v", err)
	}
	t.Cleanup(func() { _ = syscall.Setrlimit(syscall.RLIMIT_FSIZE, &old) })
	var err error
	if mode == fsfullAppend {
		err = appendFile(filepath.Join(dir, "page.md"), strings.Repeat("history\n", 64))
	} else {
		err = replaceFile(filepath.Join(dir, "a.go"), []byte(strings.Repeat("package a\n", 64)))
	}
	body, marshalErr := json.Marshal(classifyWriteError(err))
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	os.Stdout.WriteString(fsfullMarker + string(body) + "\n")
}

func runFileWriteWithNoFileGrowth(t *testing.T, mode, dir string) fsfullResult {
	t.Helper()
	raw, err := testhost.Command(fsfullHelperRun, fsfullHelperModeEnv+"="+mode, fsfullHelperDirEnv+"="+dir).Output()
	if err != nil {
		t.Fatalf("the helper process failed: %v\n%s", err, raw)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if body, ok := strings.CutPrefix(line, fsfullMarker); ok {
			var got fsfullResult
			if err := json.Unmarshal([]byte(body), &got); err != nil {
				t.Fatalf("bad helper result %q: %v", body, err)
			}
			return got
		}
	}
	t.Fatalf("the helper printed no result:\n%s", raw)
	return fsfullResult{}
}

func TestAppendFile_ReportsAFailedWriteAndLeavesNoDataF2(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "page.md")

	got := runFileWriteWithNoFileGrowth(t, fsfullAppend, dir)

	info, statErr := os.Stat(name)
	if got.Kind != kindFileTooLarge || statErr != nil || info.Size() != 0 {
		t.Errorf("appendFile = %+v, stat (%v, %v); want a file-too-large error and an empty page", got, info, statErr)
	}
}

func TestAppendFile_ReportsAnUnopenablePageF2(t *testing.T) {
	t.Parallel()

	err := appendFile(filepath.Join(t.TempDir(), "no", "such", "page.md"), "x")

	if !os.IsNotExist(err) {
		t.Errorf("appendFile = %v, want a not-exist error", err)
	}
}

func TestReplaceFile_ReportsAFailedWriteRemovesTheTempAndKeepsTheTargetF2(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "a.go")
	if err := os.WriteFile(target, []byte(fsfullPageBody), 0o640); err != nil {
		t.Fatal(err)
	}

	got := runFileWriteWithNoFileGrowth(t, fsfullReplace, dir)

	entries, readErr := os.ReadDir(dir)
	if got.Kind != kindFileTooLarge || got.FailedInDir != dir || readErr != nil || len(entries) != 1 || entries[0].Name() != "a.go" {
		t.Errorf("replaceFile = %+v, dir %v (%v); want a file-too-large error in the target dir and no temp file left", got, entries, readErr)
	}
	if body, _ := os.ReadFile(target); string(body) != fsfullPageBody {
		t.Errorf("the target changed to %q", body)
	}
}
