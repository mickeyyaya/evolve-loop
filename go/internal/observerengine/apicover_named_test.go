package observerengine

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
