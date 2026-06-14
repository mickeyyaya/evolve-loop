package fixtures

import (
	"fmt"
	"sync"
	"testing"
)

// Level is a test-log severity. Messages below the configured level are
// dropped before buffering.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// String renders the level prefix used in flushed output, e.g. "[INFO]".
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "LOG"
	}
}

// logTB is the minimal slice of *testing.T the logger depends on. *testing.T
// satisfies it; tests supply a stub to drive quiet/flush behavior
// deterministically. Kept unexported so the public surface is just Logger(t).
type logTB interface {
	Helper()
	Cleanup(func())
	Logf(format string, args ...any)
	Failed() bool
}

// TestLogger is a leveled, buffered, quiet-by-default test logger. It records
// nothing to test output while the test is passing; on failure (or under
// `go test -v`) it flushes the full leveled transcript so a failure is
// self-explaining. This is the debuggability layer of the test harness —
// detailed logs without the spam.
type TestLogger struct {
	tb      logTB
	mu      sync.Mutex
	level   Level
	verbose bool
	entries []string
}

// Logger returns a TestLogger bound to t. It is quiet by default and flushes
// its buffer automatically if the test fails; under `go test -v` it emits
// live. Default level is LevelDebug (everything retained).
func Logger(t *testing.T) *TestLogger {
	t.Helper()
	return newLogger(t, testing.Verbose())
}

// newLogger is the seam-injected constructor (see logTB). Registers the
// flush-on-failure cleanup so callers never have to.
func newLogger(tb logTB, verbose bool) *TestLogger {
	l := &TestLogger{tb: tb, level: LevelDebug, verbose: verbose}
	tb.Cleanup(l.flushOnFailure)
	return l
}

// SetLevel sets the minimum retained level and returns the logger for chaining.
func (l *TestLogger) SetLevel(lv Level) *TestLogger {
	l.mu.Lock()
	l.level = lv
	l.mu.Unlock()
	return l
}

// Debug/Info/Warn/Error record a printf-formatted message at that level.
func (l *TestLogger) Debug(format string, args ...any) { l.log(LevelDebug, format, args...) }
func (l *TestLogger) Info(format string, args ...any)  { l.log(LevelInfo, format, args...) }
func (l *TestLogger) Warn(format string, args ...any)  { l.log(LevelWarn, format, args...) }
func (l *TestLogger) Error(format string, args ...any) { l.log(LevelError, format, args...) }

func (l *TestLogger) log(lv Level, format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if lv < l.level {
		return
	}
	line := fmt.Sprintf("[%s] %s", lv, fmt.Sprintf(format, args...))
	if l.verbose {
		l.tb.Helper()
		l.tb.Logf("%s", line)
		return
	}
	l.entries = append(l.entries, line)
}

// Flush emits all buffered entries to the test log and clears the buffer.
// Idempotent — a second call with no new entries emits nothing.
func (l *TestLogger) Flush() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, e := range l.entries {
		l.tb.Helper()
		l.tb.Logf("%s", e)
	}
	l.entries = nil
}

// flushOnFailure is the registered cleanup: it flushes only if the test failed,
// keeping passing tests silent. It reads tb.Failed() without the mutex because
// testing runs Cleanup funcs serially, after all test goroutines have finished —
// no log() call can be in flight here.
func (l *TestLogger) flushOnFailure() {
	if l.tb.Failed() {
		l.Flush()
	}
}
