package observerengine

import "testing"

func TestAPICover_EveryExportIsNamed(t *testing.T) {
	t.Parallel()
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
	var opt Option = WithSignals(nil)
	e := New(settingsFixture(t.TempDir()), depsFixture(), opt)
	e.Start()
	var stop Stop = e.Tick()
	if stop != StopNone || e.SignalsWired() {
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
