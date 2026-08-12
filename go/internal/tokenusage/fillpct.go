package tokenusage

// fillpct.go — context-fill telemetry (cycle-1444, task
// `context-fill-telemetry-record`). Fill% is a DERIVED reading off the usage
// the resolver already recovered — prompt-side tokens ÷ the driver family's
// effective window — never a second independent measurement path.
//
// Two invariants carry the whole file: an unmeasurable reading is an explicit
// negative sentinel (never 0%, never Inf/NaN, so a comparison downstream can
// never silently treat "we don't know" as "empty context"), and an unmapped CLI
// family reports window 0 rather than a guessed window (publishing a fabricated
// fill reading is worse than publishing none).

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

// FillPctUnmeasured is the reading for a launch whose context fill could not be
// derived — no configured window for the driver family, or no tier observed the
// usage. It is deliberately negative so it can never collide with a real 0–100
// percentage, and so every threshold comparison fails closed (silent) on it.
const FillPctUnmeasured = -1.0

// claudeEffectiveWindow is the conservative effective context window for the
// claude family. Deliberately below any advertised maximum: the reading that
// matters operationally is how close a launch is to degradation, not to the
// hard ceiling.
const claudeEffectiveWindow = 200_000

// PromptTokens returns the input-side token count that occupies the context
// window: fresh input plus both cache halves. Generated Output is excluded —
// it does not sit in the prompt, and counting it would make the fill WARN fire
// on long answers instead of on big prompts.
func PromptTokens(u cyclestate.TokenUsage) int {
	return u.Input + u.CacheRead + u.CacheWrite
}

// EffectiveWindow returns the effective context window for a driver family, or
// 0 when the family has no measured window. Empty means claude, matching
// isClaudeDriver — the window table must not disagree with the collector
// dispatch about who is a claude launch. Families whose real window nobody has
// measured (codex, agy, ollama, anything unknown) return 0 on purpose, which
// degrades FillPct to the sentinel rather than to a guess.
func EffectiveWindow(driver string) int {
	if isClaudeDriver(driver) {
		return claudeEffectiveWindow
	}
	return 0
}

// FillPct returns how full the context window is, as a percentage on a 0–100
// scale so a threshold expressed in percent compares directly against it. A
// zero or negative window is unmeasurable and yields FillPctUnmeasured — the
// divide-by-zero guard that keeps Inf/NaN out of every downstream comparison.
// Over-full readings are not clamped: a launch at 120% is a real signal.
func FillPct(promptTokens, window int) float64 {
	if window <= 0 {
		return FillPctUnmeasured
	}
	return float64(promptTokens) / float64(window) * 100
}

// FillWarn returns the operator-facing warning for a fill reading, or "" when
// no warning is due. It fires only STRICTLY above the threshold (a launch
// exactly at the line has not crossed it) and never on the unmeasured sentinel.
// The message names the phase: an unattributed fill WARN cannot be acted on,
// because the operator cannot tell which launch is near compaction.
func FillWarn(phase string, pct float64, thresholdPct int) string {
	if pct == FillPctUnmeasured || pct < 0 {
		return ""
	}
	if pct <= float64(thresholdPct) {
		return ""
	}
	return fmt.Sprintf("context fill %.1f%% for phase %s exceeds the %d%% warn threshold — this launch is close to compaction", pct, phase, thresholdPct)
}
