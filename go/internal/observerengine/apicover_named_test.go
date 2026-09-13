package observerengine

// apicover_named_test.go — every export of the leaf named AND exercised by the
// package's own tests (the 100 % API gate counts package-local tests only):
// Paths/PathsFor + the three suffixes + the two scopes (TestPathsFor_…),
// Settings/Deps/Engine/Option/New/WithSignals/SignalsWired (the fixtures,
// TestWithSignals_…), Start/Stop/StopNone/StopEOFGrace/Tick/Shutdown/
// WriteReport (the tick, sequence and report tests), Reporter/NewReporter/
// Wired/Report (TestReporter_…), the eight codes (the registry test). This
// file pins the exported SET positionally so a new export must be named here.

import "testing"

func TestAPICover_EveryExportIsNamed(t *testing.T) {
	t.Parallel()
	var (
		_ Paths     = PathsFor("", "")
		_ Settings  = settingsFixture(t.TempDir())
		_ Deps      = depsFixture()
		_ Option    = WithSignals(nil)
		_ *Engine   = New(settingsFixture(t.TempDir()), depsFixture())
		_ Stop      = StopNone
		_ Stop      = StopEOFGrace
		_ *Reporter = NewReporter(nil)
	)
	for _, s := range []string{StdoutSuffix, EventsSuffix, ReportSuffix, ScopePhase, ScopeCycle} {
		if s == "" {
			t.Error("empty layout literal")
		}
	}
	for _, c := range []string{string(CodeNudgeAppendFailed), string(CodeReportWriteFailed), string(CodeStallKillSent), string(CodeKillFailed), string(CodeEventAppendFailed), string(CodeStdoutTailFailed), string(CodeEventsSinkOpenFailed), string(CodeWatcherLeaked)} {
		if c == "" {
			t.Error("empty code")
		}
	}
	e := New(settingsFixture(t.TempDir()), depsFixture())
	e.Start()
	if e.Tick() != StopNone || e.SignalsWired() {
		t.Error("a fresh engine ticks quietly and is unwired")
	}
	e.Shutdown("stop-timer")
	if err := e.WriteReport(); err != nil {
		t.Error(err)
	}
	NewReporter(nil).Report("Engine.Tick", 0, "", CodeKillFailed, "", nil)
	if NewReporter(nil).Wired() {
		t.Error("nil reporter is unwired")
	}
}
