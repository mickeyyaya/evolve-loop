package acsrunner

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ParseTestJSON consumes `go test -json` output and aggregates per-test
// verdicts into a Verdict struct. This is the pure parser; Run() in
// runner.go wraps it with go-test invocation.

func TestParseTestJSON_SimplePassFail(t *testing.T) {
	input := strings.Join([]string{
		`{"Time":"2026-05-22T10:00:00Z","Action":"run","Package":"acs/cycle-104","Test":"TestPredicateA"}`,
		`{"Time":"2026-05-22T10:00:00Z","Action":"pass","Package":"acs/cycle-104","Test":"TestPredicateA","Elapsed":0.123}`,
		`{"Time":"2026-05-22T10:00:00Z","Action":"run","Package":"acs/cycle-104","Test":"TestPredicateB"}`,
		`{"Time":"2026-05-22T10:00:00Z","Action":"output","Package":"acs/cycle-104","Test":"TestPredicateB","Output":"--- FAIL: TestPredicateB\n"}`,
		`{"Time":"2026-05-22T10:00:00Z","Action":"fail","Package":"acs/cycle-104","Test":"TestPredicateB","Elapsed":0.456}`,
	}, "\n") + "\n"
	v, err := ParseTestJSON(strings.NewReader(input), 104)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if v.Cycle != 104 {
		t.Errorf("cycle=%d, want 104", v.Cycle)
	}
	if v.Total != 2 {
		t.Errorf("total=%d, want 2", v.Total)
	}
	if v.RedCount != 1 {
		t.Errorf("redCount=%d, want 1", v.RedCount)
	}
	if len(v.Predicates) != 2 {
		t.Fatalf("predicates=%d, want 2", len(v.Predicates))
	}
	// Find each by name.
	got := map[string]Predicate{}
	for _, p := range v.Predicates {
		got[p.Name] = p
	}
	if got["TestPredicateA"].Verdict != "PASS" {
		t.Errorf("A verdict=%q", got["TestPredicateA"].Verdict)
	}
	if got["TestPredicateA"].DurationMS != 123 {
		t.Errorf("A duration_ms=%d, want 123", got["TestPredicateA"].DurationMS)
	}
	if got["TestPredicateB"].Verdict != "FAIL" {
		t.Errorf("B verdict=%q", got["TestPredicateB"].Verdict)
	}
	if !strings.Contains(got["TestPredicateB"].Output, "--- FAIL") {
		t.Errorf("B output missing FAIL line: %q", got["TestPredicateB"].Output)
	}
}

func TestParseTestJSON_SkipsPackageLevelEvents(t *testing.T) {
	// Lines without Test name are package-level (run/pass for the
	// package itself) and must not be counted.
	input := strings.Join([]string{
		`{"Action":"start","Package":"acs/cycle-1"}`,
		`{"Action":"run","Package":"acs/cycle-1","Test":"TestX"}`,
		`{"Action":"pass","Package":"acs/cycle-1","Test":"TestX","Elapsed":0.001}`,
		`{"Action":"pass","Package":"acs/cycle-1","Elapsed":0.5}`,
	}, "\n") + "\n"
	v, err := ParseTestJSON(strings.NewReader(input), 1)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if v.Total != 1 {
		t.Errorf("total=%d, want 1 (package-level pass must not count)", v.Total)
	}
}

func TestParseTestJSON_SkipAction(t *testing.T) {
	input := `{"Action":"run","Package":"x","Test":"TestS"}
{"Action":"skip","Package":"x","Test":"TestS","Elapsed":0}
`
	v, err := ParseTestJSON(strings.NewReader(input), 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if v.Total != 1 {
		t.Errorf("total=%d, want 1", v.Total)
	}
	if v.RedCount != 0 {
		t.Errorf("redCount=%d, want 0 (skip is not red)", v.RedCount)
	}
	if v.SkipCount != 1 {
		t.Errorf("skipCount=%d, want 1 (skip is counted as skip)", v.SkipCount)
	}
	if v.Predicates[0].Verdict != "SKIP" {
		t.Errorf("verdict=%q, want SKIP", v.Predicates[0].Verdict)
	}
}

func TestParseTestJSON_InvalidLineIgnored(t *testing.T) {
	// Mixed valid + invalid; the parser should skip invalid lines
	// without bailing the whole verdict.
	input := strings.Join([]string{
		`not json`,
		`{"Action":"run","Package":"x","Test":"TestA"}`,
		`{"Action":"pass","Package":"x","Test":"TestA","Elapsed":0.1}`,
		``,
	}, "\n") + "\n"
	v, err := ParseTestJSON(strings.NewReader(input), 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if v.Total != 1 || v.RedCount != 0 {
		t.Errorf("aggregate wrong: %+v", v)
	}
}

func TestVerdict_JSONShape(t *testing.T) {
	v := Verdict{
		Cycle:    42,
		Total:    3,
		RedCount: 1,
		Predicates: []Predicate{
			{Name: "TestA", Verdict: "PASS", DurationMS: 100},
			{Name: "TestB", Verdict: "FAIL", DurationMS: 200, Output: "msg"},
		},
	}
	raw, _ := json.Marshal(v)
	for _, want := range []string{
		`"cycle":42`,
		`"total":3`,
		`"red_count":1`,
		`"predicates"`,
		`"duration_ms":100`,
		`"verdict":"PASS"`,
	} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("missing %q in: %s", want, raw)
		}
	}
}

func TestWriteVerdict_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	v := Verdict{
		Cycle:    7,
		Total:    2,
		RedCount: 0,
		Predicates: []Predicate{
			{Name: "TestA", Verdict: "PASS", DurationMS: 5},
			{Name: "TestB", Verdict: "PASS", DurationMS: 6},
		},
	}
	dst, err := WriteVerdict(dir, v)
	if err != nil {
		t.Fatalf("WriteVerdict: %v", err)
	}
	if !strings.HasSuffix(dst, "/runs/cycle-7/acs-verdict.json") {
		t.Errorf("dst path unexpected: %s", dst)
	}
	raw, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	var got Verdict
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Cycle != 7 || got.Total != 2 || len(got.Predicates) != 2 {
		t.Errorf("round-trip lost: %+v", got)
	}
}

func TestWriteVerdict_AtomicNoTmpLeftovers(t *testing.T) {
	dir := t.TempDir()
	v := Verdict{Cycle: 9}
	_, err := WriteVerdict(dir, v)
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "runs", "cycle-9"))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("leftover tmp: %s", e.Name())
		}
	}
}

func TestWriteVerdict_MkdirError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses permissions")
	}
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := WriteVerdict(blocker, Verdict{Cycle: 1})
	if err == nil {
		t.Fatal("expected mkdir error")
	}
}

func TestWriteVerdict_MarshalError(t *testing.T) {
	dir := t.TempDir()
	withWriteHooks(writeHooks{
		marshal: func(any) ([]byte, error) { return nil, errors.New("forced marshal fail") },
	}, func() {
		_, err := WriteVerdict(dir, Verdict{Cycle: 1})
		if err == nil {
			t.Fatal("expected marshal error")
		}
	})
}

func TestWriteVerdict_WriteError(t *testing.T) {
	dir := t.TempDir()
	withWriteHooks(writeHooks{
		write: func(*os.File, []byte) (int, error) { return 0, errors.New("forced write fail") },
	}, func() {
		_, err := WriteVerdict(dir, Verdict{Cycle: 1})
		if err == nil {
			t.Fatal("expected write error")
		}
	})
}

func TestWriteVerdict_CloseError(t *testing.T) {
	dir := t.TempDir()
	withWriteHooks(writeHooks{
		closeF: func(*os.File) error { return errors.New("forced close fail") },
	}, func() {
		_, err := WriteVerdict(dir, Verdict{Cycle: 1})
		if err == nil {
			t.Fatal("expected close error")
		}
	})
}

func TestWriteVerdict_RenameError(t *testing.T) {
	dir := t.TempDir()
	withWriteHooks(writeHooks{
		rename: func(_, _ string) error { return errors.New("forced rename fail") },
	}, func() {
		_, err := WriteVerdict(dir, Verdict{Cycle: 1})
		if err == nil {
			t.Fatal("expected rename error")
		}
	})
}

func TestRun_EmptyPackageNoTests(t *testing.T) {
	// Create a throwaway module with no test files; go test should
	// exit 0 with zero tests emitted, exercising Run's happy path
	// without actually executing any user-supplied test code.
	mod := t.TempDir()
	if err := os.WriteFile(filepath.Join(mod, "go.mod"), []byte("module example.test\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mod, "x.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// We need to cd into the module so go test resolves it. Use a
	// subshell-equivalent — exec.Cmd.Dir.
	cwd, _ := os.Getwd()
	defer func() { _ = os.Chdir(cwd) }()
	if err := os.Chdir(mod); err != nil {
		t.Fatal(err)
	}
	v, err := Run(context.Background(), 1, "./...")
	if err != nil {
		t.Fatalf("Run on empty pkg: %v", err)
	}
	if v.Total != 0 {
		t.Errorf("empty package total=%d, want 0", v.Total)
	}
}

func TestRun_CompileFailureBubblesUp(t *testing.T) {
	mod := t.TempDir()
	_ = os.WriteFile(filepath.Join(mod, "go.mod"), []byte("module example.test\n\ngo 1.23\n"), 0o644)
	// Intentionally broken test file — go test will fail with no
	// tests reported, so red_count stays 0 and Run must surface a
	// non-nil error.
	_ = os.WriteFile(filepath.Join(mod, "broken_test.go"), []byte("package x\nfunc TestBroken(t Bogus) {}\n"), 0o644)
	cwd, _ := os.Getwd()
	defer func() { _ = os.Chdir(cwd) }()
	_ = os.Chdir(mod)
	_, err := Run(context.Background(), 1, "./...")
	if err == nil {
		t.Error("expected compile-failure error")
	}
}

type closingReader struct{ *strings.Reader }

func (c *closingReader) Close() error { return nil }

func TestRun_CommanderStartError(t *testing.T) {
	withCommander(func(ctx context.Context, args ...string) (io.ReadCloser, func() error, error) {
		return nil, nil, errors.New("forced start fail")
	}, func() {
		_, err := Run(context.Background(), 1, "./...")
		if err == nil {
			t.Fatal("expected start error")
		}
	})
}

// scanErrReader returns a single oversize line so bufio.Scanner's
// internal MaxScanTokenSize trips and surfaces a scan error — drives
// ParseTestJSON's scanner.Err() return branch.
type scanErrReader struct {
	b   []byte
	off int
}

func (s *scanErrReader) Read(p []byte) (int, error) {
	if s.off >= len(s.b) {
		return 0, io.EOF
	}
	n := copy(p, s.b[s.off:])
	s.off += n
	return n, nil
}
func (s *scanErrReader) Close() error { return nil }

func TestParseTestJSON_ScannerError(t *testing.T) {
	// 2MB single line — exceeds the 1MB scanner buffer max.
	big := make([]byte, 2*1024*1024)
	for i := range big {
		big[i] = 'x'
	}
	_, err := ParseTestJSON(&scanErrReader{b: big}, 0)
	if err == nil {
		t.Fatal("expected scanner buffer overflow error")
	}
}

func TestRun_ParseErrorPropagates(t *testing.T) {
	big := make([]byte, 2*1024*1024)
	for i := range big {
		big[i] = 'x'
	}
	withCommander(func(ctx context.Context, args ...string) (io.ReadCloser, func() error, error) {
		return &scanErrReader{b: big}, func() error { return nil }, nil
	}, func() {
		_, err := Run(context.Background(), 0, "./...")
		if err == nil {
			t.Fatal("expected parse error to propagate")
		}
	})
}

func TestRun_CommanderInjectsPassFail(t *testing.T) {
	withCommander(func(ctx context.Context, args ...string) (io.ReadCloser, func() error, error) {
		body := `{"Action":"run","Package":"p","Test":"TestA"}
{"Action":"pass","Package":"p","Test":"TestA","Elapsed":0.001}
{"Action":"run","Package":"p","Test":"TestB"}
{"Action":"fail","Package":"p","Test":"TestB","Elapsed":0.002}
`
		return &closingReader{strings.NewReader(body)}, func() error { return errors.New("exit 1 simulated") }, nil
	}, func() {
		v, err := Run(context.Background(), 42, "./...")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if v.RedCount != 1 || v.Total != 2 || v.Cycle != 42 {
			t.Errorf("bad verdict: %+v", v)
		}
	})
}

func TestRun_NonExistentPackageErrors(t *testing.T) {
	mod := t.TempDir()
	_ = os.WriteFile(filepath.Join(mod, "go.mod"), []byte("module example.test\n\ngo 1.23\n"), 0o644)
	cwd, _ := os.Getwd()
	defer func() { _ = os.Chdir(cwd) }()
	_ = os.Chdir(mod)
	_, err := Run(context.Background(), 1, "./doesnotexist/...")
	if err == nil {
		t.Error("expected error for missing pkg")
	}
}

func TestParseTestJSON_OutputAccumulates(t *testing.T) {
	input := strings.Join([]string{
		`{"Action":"run","Package":"x","Test":"T"}`,
		`{"Action":"output","Package":"x","Test":"T","Output":"line1\n"}`,
		`{"Action":"output","Package":"x","Test":"T","Output":"line2\n"}`,
		`{"Action":"fail","Package":"x","Test":"T","Elapsed":0.001}`,
	}, "\n") + "\n"
	v, _ := ParseTestJSON(strings.NewReader(input), 0)
	if len(v.Predicates) != 1 {
		t.Fatalf("preds=%d", len(v.Predicates))
	}
	if !strings.Contains(v.Predicates[0].Output, "line1") || !strings.Contains(v.Predicates[0].Output, "line2") {
		t.Errorf("output not accumulated: %q", v.Predicates[0].Output)
	}
}
