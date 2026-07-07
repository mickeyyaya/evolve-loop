package tokenusage

// defaultresolver.go — the single shared resolver both production
// composition roots (internal/adapters/bridge.Adapter and internal/subagent's
// defaultExecAdapter) wire into their gobridge.Deps.TokenResolver field. See
// defaultresolver_test.go for the RED contract this satisfies.

// DefaultResolver returns a resolver that recovers token usage for a Window
// from the transcript tier only (S4/S5 collectors need a logPath/pane that a
// bare Window does not carry). It never errors — telemetry is best-effort —
// and falls open to SourceNone when no transcript matches.
func DefaultResolver(configRoot string) func(Window) (Result, error) {
	return func(w Window) (Result, error) {
		return Chain(TranscriptCollector(configRoot, w)), nil
	}
}
