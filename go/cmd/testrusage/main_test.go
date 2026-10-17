package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

const helperEnv = "TESTRUSAGE_HELPER"

func TestMain(m *testing.M) {
	os.Setenv("GORACE", "atexit_sleep_ms=0")
	switch os.Getenv(helperEnv) {
	case "alloc":
		helperAlloc()
	case "spawn":
		os.Exit(0)
	case "echo":
		helperEcho()
	case "exit":
		os.Exit(7)
	case "sleep":
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func helperAlloc() {
	buf := make([]byte, 64<<20)
	for i := 0; i < len(buf); i += 4096 {
		buf[i] = 1
	}
	child := exec.Command(os.Args[0])
	child.Env = append(os.Environ(), helperEnv+"=spawn")
	if err := child.Run(); err != nil {
		os.Exit(3)
	}
	if buf[len(buf)-4096] != 1 {
		os.Exit(4)
	}
	os.Exit(0)
}

func helperEcho() {
	data := make([]byte, 5)
	if _, err := os.Stdin.Read(data); err != nil {
		os.Exit(5)
	}
	os.Stdout.Write(append([]byte("out:"), data...))
	os.Stderr.Write(append([]byte("err:"), data...))
	os.Exit(0)
}

func runWrapper(t *testing.T, mode string, stdin string, extraArgs ...string) (code int, stdout, stderr string, records []Record) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "rusage.ndjson")
	t.Setenv(helperEnv, mode)
	t.Setenv(outEnv, out)
	var so, se bytes.Buffer
	args := append([]string{os.Args[0]}, extraArgs...)
	code = run(args, strings.NewReader(stdin), &so, &se, nil)
	return code, so.String(), se.String(), readRecords(t, out)
}

func readRecords(t *testing.T, path string) []Record {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var recs []Record
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		var r Record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("bad record %q: %v", line, err)
		}
		recs = append(recs, r)
	}
	return recs
}

func TestRun_AllocAndSpawn_RecordsPeakRSSAndSystemCPU(t *testing.T) {
	code, _, stderr, recs := runWrapper(t, "alloc", "")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if len(recs) != 1 {
		t.Fatalf("records=%d", len(recs))
	}
	r := recs[0]
	if r.PeakRSSMB < 64 {
		t.Errorf("peak_rss_mb=%v want >=64", r.PeakRSSMB)
	}
	if r.SystemS <= 0 {
		t.Errorf("system_s=%v want >0", r.SystemS)
	}
	if r.WallS <= 0 || r.UserS < 0 || r.ExitCode != 0 {
		t.Errorf("record=%+v", r)
	}
	if r.Package != "cmd/testrusage" {
		t.Errorf("package=%q", r.Package)
	}
}

func TestRun_FailingChild_PassesExitCodeThrough(t *testing.T) {
	code, _, _, recs := runWrapper(t, "exit", "")
	if code != 7 {
		t.Fatalf("code=%d want 7", code)
	}
	if len(recs) != 1 || recs[0].ExitCode != 7 {
		t.Fatalf("records=%+v", recs)
	}
}

func TestRun_PassesStdioThrough(t *testing.T) {
	code, stdout, stderr, _ := runWrapper(t, "echo", "hello")
	if code != 0 || stdout != "out:hello" || stderr != "err:hello" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestRun_AppendsOneRecordPerRun(t *testing.T) {
	out := filepath.Join(t.TempDir(), "rusage.ndjson")
	t.Setenv(helperEnv, "spawn")
	t.Setenv(outEnv, out)
	for i := 0; i < 2; i++ {
		if code := run([]string{os.Args[0]}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}, nil); code != 0 {
			t.Fatalf("code=%d", code)
		}
	}
	if n := len(readRecords(t, out)); n != 2 {
		t.Fatalf("records=%d want 2", n)
	}
}

func TestRun_NoOutEnv_StillRunsChildAndWritesNoFile(t *testing.T) {
	t.Setenv(helperEnv, "exit")
	t.Setenv(outEnv, "")
	var se bytes.Buffer
	if code := run([]string{os.Args[0]}, strings.NewReader(""), &bytes.Buffer{}, &se, nil); code != 7 {
		t.Fatalf("code=%d", code)
	}
	if se.Len() != 0 {
		t.Fatalf("stderr=%q", se.String())
	}
}

func TestRun_NoArgs_UsageError(t *testing.T) {
	var se bytes.Buffer
	if code := run(nil, strings.NewReader(""), &bytes.Buffer{}, &se, nil); code != 2 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(se.String(), "usage") {
		t.Fatalf("stderr=%q", se.String())
	}
}

func TestRun_StartFailure_Exits127(t *testing.T) {
	t.Setenv(outEnv, filepath.Join(t.TempDir(), "x.ndjson"))
	var se bytes.Buffer
	if code := run([]string{"/nonexistent/test.bin"}, strings.NewReader(""), &bytes.Buffer{}, &se, nil); code != 127 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(se.String(), "testrusage:") {
		t.Fatalf("stderr=%q", se.String())
	}
}

func TestRun_UnwritableOut_WarnsButKeepsChildExitCode(t *testing.T) {
	t.Setenv(helperEnv, "exit")
	t.Setenv(outEnv, filepath.Join(t.TempDir(), "missing-dir", "x.ndjson"))
	var se bytes.Buffer
	if code := run([]string{os.Args[0]}, strings.NewReader(""), &bytes.Buffer{}, &se, nil); code != 7 {
		t.Fatalf("code=%d want 7", code)
	}
	if !strings.Contains(se.String(), "testrusage:") {
		t.Fatalf("stderr=%q", se.String())
	}
}

func TestRun_SignaledChild_ExitsWith128PlusSignal(t *testing.T) {
	out := filepath.Join(t.TempDir(), "x.ndjson")
	t.Setenv(helperEnv, "sleep")
	t.Setenv(outEnv, out)
	sig := make(chan os.Signal, 1)
	go func() {
		time.Sleep(300 * time.Millisecond)
		sig <- syscall.SIGTERM
	}()
	code := run([]string{os.Args[0]}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}, sig)
	if code != 128+int(syscall.SIGTERM) {
		t.Fatalf("code=%d", code)
	}
	if recs := readRecords(t, out); len(recs) != 1 || recs[0].ExitCode != code {
		t.Fatalf("records=%+v", recs)
	}
}

func TestMain_UsesProcessArgsAndExits(t *testing.T) {
	oldExit, oldArgs := exitProcess, os.Args
	t.Cleanup(func() { exitProcess, os.Args = oldExit, oldArgs })
	var got = -1
	exitProcess = func(c int) { got = c }
	t.Setenv(helperEnv, "exit")
	t.Setenv(outEnv, "")
	os.Args = []string{"testrusage", oldArgs[0]}
	main()
	if got != 7 {
		t.Fatalf("exit=%d", got)
	}
}

func TestPeakRSSMB_NormalizesPerOS(t *testing.T) {
	cases := []struct {
		goos string
		raw  int64
		want float64
	}{
		{"linux", 65536, 64},
		{"darwin", 67108864, 64},
		{"darwin", 0, 0},
	}
	for _, c := range cases {
		if got := peakRSSMB(c.raw, c.goos); got != c.want {
			t.Errorf("%s %d: got %v want %v", c.goos, c.raw, got, c.want)
		}
	}
}

func TestPackageName_RelativeToModuleRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "internal", "foo")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := packageName(nested); got != "internal/foo" {
		t.Errorf("nested: %q", got)
	}
	if got := packageName(root); got != "." {
		t.Errorf("root: %q", got)
	}
	bare := t.TempDir()
	if got := packageName(bare); got != filepath.ToSlash(bare) {
		t.Errorf("no go.mod: %q", got)
	}
}

type fakeFile struct {
	writeErr, closeErr error
	closed             bool
}

func (f *fakeFile) Write(p []byte) (int, error) { return len(p), f.writeErr }
func (f *fakeFile) Close() error                { f.closed = true; return f.closeErr }

func TestAppendRecord_WriteAndCloseErrors(t *testing.T) {
	old := openAppend
	t.Cleanup(func() { openAppend = old })
	w := &fakeFile{writeErr: errors.New("disk full")}
	openAppend = func(string) (io.WriteCloser, error) { return w, nil }
	if err := appendRecord("x", Record{}); err == nil || !w.closed {
		t.Fatalf("write error: err=%v closed=%v", err, w.closed)
	}
	c := &fakeFile{closeErr: errors.New("close failed")}
	openAppend = func(string) (io.WriteCloser, error) { return c, nil }
	if err := appendRecord("x", Record{}); err == nil {
		t.Fatal("want close error")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("sink broken") }

func TestRun_StdoutCopyFailure_IsReported(t *testing.T) {
	t.Setenv(helperEnv, "echo")
	t.Setenv(outEnv, "")
	var se bytes.Buffer
	run([]string{os.Args[0]}, strings.NewReader("hello"), failingWriter{}, &se, nil)
	if !strings.Contains(se.String(), "sink broken") {
		t.Fatalf("stderr=%q", se.String())
	}
}

func TestBuildRecord_NoRusage_ZeroUsageAndNote(t *testing.T) {
	rec := buildRecord(&os.ProcessState{}, time.Second, 3)
	if rec.Note != "rusage: unavailable" || rec.UserS != 0 || rec.SystemS != 0 || rec.PeakRSSMB != 0 || rec.ExitCode != 3 || rec.WallS != 1 {
		t.Fatalf("rec=%+v", rec)
	}
}

func TestRun_WaitFailureWithoutProcessState_ReportsAndExits1(t *testing.T) {
	old := waitCmd
	t.Cleanup(func() { waitCmd = old })
	waitCmd = func(c *exec.Cmd) error {
		_ = c.Process.Kill()
		_ = (*exec.Cmd).Wait(c)
		c.ProcessState = nil
		return errors.New("wait boom")
	}
	t.Setenv(helperEnv, "sleep")
	t.Setenv(outEnv, filepath.Join(t.TempDir(), "x.ndjson"))
	var se bytes.Buffer
	code := run([]string{os.Args[0]}, strings.NewReader(""), &bytes.Buffer{}, &se, nil)
	if code != 1 || !strings.Contains(se.String(), "wait boom") {
		t.Fatalf("code=%d stderr=%q", code, se.String())
	}
}
