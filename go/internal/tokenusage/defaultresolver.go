package tokenusage

// defaultresolver.go — the single shared resolver both production
// composition roots (internal/adapters/bridge.Adapter and internal/subagent's
// defaultExecAdapter) wire into their gobridge.Deps.TokenResolver field. See
// defaultresolver_test.go for the RED contract this satisfies.

// DefaultResolver returns a resolver that recovers token usage for a Window
// through the full fidelity chain: transcript > eventsResult (w.EventsLogPath)
// > scrollbackPeak (w.Scrollback). It never errors — telemetry is best-effort —
// and falls open to SourceNone when no tier has data.
func DefaultResolver(configRoot string) func(Window) (Result, error) {
	return func(w Window) (Result, error) {
		return Chain(
			TranscriptCollector(configRoot, w),
			EventsResultCollector(w.EventsLogPath),
			ScrollbackPeakCollector(w.Scrollback),
		), nil
	}
}
