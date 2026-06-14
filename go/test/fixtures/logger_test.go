package fixtures

// White-box: drives the unexported logTB seam + newLogger so the
// quiet/flush/level behavior is verified deterministically without a real
// failing test. The public Logger(t) constructor is smoke-tested too.

import (
	"fmt"
	"strings"
	"testing"
)

// stubTB is a controllable logTB for asserting flush/quiet behavior.
type stubTB struct {
	logs     []string
	cleanups []func()
	failed   bool
}

func (s *stubTB) Helper()           {}
func (s *stubTB) Cleanup(fn func()) { s.cleanups = append(s.cleanups, fn) }
func (s *stubTB) Logf(format string, args ...any) {
	s.logs = append(s.logs, fmt.Sprintf(format, args...))
}
func (s *stubTB) Failed() bool { return s.failed }
func (s *stubTB) runCleanups() {
	for i := len(s.cleanups) - 1; i >= 0; i-- {
		s.cleanups[i]()
	}
}

func TestLogger_QuietByDefault_NoOutputWhenPassing(t *testing.T) {
	s := &stubTB{}
	l := newLogger(s, false /* not verbose */)
	l.Info("setup %s", "ok")
	l.Debug("detail")
	if len(s.logs) != 0 {
		t.Fatalf("logs = %v, want none before failure (quiet by default)", s.logs)
	}
	s.runCleanups() // test "passed"
	if len(s.logs) != 0 {
		t.Fatalf("logs = %v, want none on a passing test", s.logs)
	}
}

func TestLogger_FlushesBufferOnFailure(t *testing.T) {
	s := &stubTB{}
	l := newLogger(s, false)
	l.Info("step a")
	l.Warn("step b")
	s.failed = true
	s.runCleanups()
	joined := strings.Join(s.logs, "\n")
	if !strings.Contains(joined, "[INFO] step a") || !strings.Contains(joined, "[WARN] step b") {
		t.Fatalf("on failure, logs = %v, want buffered entries with level prefixes", s.logs)
	}
}

func TestLogger_VerboseEmitsLive(t *testing.T) {
	s := &stubTB{}
	l := newLogger(s, true /* verbose */)
	l.Info("live %d", 1)
	if len(s.logs) != 1 || !strings.Contains(s.logs[0], "[INFO] live 1") {
		t.Fatalf("verbose logs = %v, want immediate emit", s.logs)
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	s := &stubTB{}
	l := newLogger(s, false).SetLevel(LevelWarn)
	l.Debug("d")
	l.Info("i")
	l.Warn("w")
	l.Error("e")
	s.failed = true
	s.runCleanups()
	joined := strings.Join(s.logs, "\n")
	if strings.Contains(joined, "[DEBUG]") || strings.Contains(joined, "[INFO]") {
		t.Fatalf("logs = %v, want DEBUG/INFO filtered out at LevelWarn", s.logs)
	}
	if !strings.Contains(joined, "[WARN] w") || !strings.Contains(joined, "[ERROR] e") {
		t.Fatalf("logs = %v, want WARN+ERROR retained", s.logs)
	}
}

func TestLogger_ManualFlushClearsBuffer(t *testing.T) {
	s := &stubTB{}
	l := newLogger(s, false)
	l.Info("once")
	l.Flush()
	if len(s.logs) != 1 {
		t.Fatalf("after Flush, logs = %v, want 1", s.logs)
	}
	l.Flush() // buffer cleared → no duplicate
	if len(s.logs) != 1 {
		t.Fatalf("after second Flush, logs = %v, want still 1 (buffer cleared)", s.logs)
	}
}

func TestLogger_PublicConstructorSmoke(t *testing.T) {
	l := Logger(t) // real *testing.T
	if l == nil {
		t.Fatal("Logger(t) = nil")
	}
	l.Info("constructed %d", 1)
	l.Debug("no panic")
}
