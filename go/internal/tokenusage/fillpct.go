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
	"math"
	"sort"
	"strings"
	"unicode"

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

// promptTokensUnmeasured is the prompt-side total for a launch whose counters
// cannot be summed honestly — a negative counter, or a sum that would overflow
// int. Negative on purpose, in the same vocabulary as FillPctUnmeasured, so
// FillPct's negative guard turns it into the documented sentinel instead of a
// fabricated reading. Unexported: no caller needs to distinguish "unmeasurable
// prompt total" from "unmeasurable fill".
const promptTokensUnmeasured = -1

// PromptTokens returns the input-side token count that occupies the context
// window: fresh input plus both cache halves. Generated Output is excluded —
// it does not sit in the prompt, and counting it would make the fill WARN fire
// on long answers instead of on big prompts.
//
// The three counters are driver-controlled (they arrive as JSON off a CLI's own
// usage report), so the sum is guarded rather than trusted: any negative
// counter, or an addition that would wrap, returns a negative total. Wrapping
// silently is the worse failure — it publishes a fabricated percentage that
// FillWarn's "any negative is unmeasured" rule then swallows, so the launch
// whose telemetry is bogus is exactly the one that raises no warning
// (cycle-1444 audit M1).
func PromptTokens(u cyclestate.TokenUsage) int {
	total := 0
	for _, n := range [...]int{u.Input, u.CacheRead, u.CacheWrite} {
		if n < 0 || total > math.MaxInt-n {
			return promptTokensUnmeasured
		}
		total += n
	}
	return total
}

// windowOccupancy returns the prompt-side token count to measure a Result
// against its window: the fullest single observed turn when the tier broke the
// launch down per turn (transcript), otherwise that tier's whole-launch
// prompt-side total. The distinction is the cycle-1455 defect: a per-turn tier's
// summed Usage counts the same accumulated context once per turn and reads as
// multiples of 100% (566.9% observed live), while the events/scrollback tiers
// report ONE result envelope, whose total already is a single reading.
//
// A negative peak (turns were expected, none was observed) is passed through
// rather than repaired, so FillPct's negative guard turns it into the documented
// sentinel — nothing observed the context is not the same as the context being
// empty.
func windowOccupancy(r Result) int {
	if r.PeakPromptTokens != 0 {
		return r.PeakPromptTokens
	}
	return PromptTokens(r.Usage)
}

// EffectiveWindow returns the effective context window for a driver family, or
// 0 when the family has no measured window. Empty means claude, matching
// isClaudeDriver — the window table must not disagree with the collector
// dispatch about who is a claude launch. Codex and agy have documented
// advertised windows, so their exact production identities use the same
// conservative 200K effective window as Claude. Families whose real window
// nobody has measured (ollama, anything unknown) return 0 on purpose, which
// degrades FillPct to the sentinel rather than to a guess.
func EffectiveWindow(driver string) int {
	if isClaudeDriver(driver) {
		return claudeEffectiveWindow
	}
	switch driver {
	case "codex", "codex-tmux", "agy", "agy-tmux":
		return claudeEffectiveWindow
	}
	return 0
}

// FillPct returns how full the context window is, as a percentage on a 0–100
// scale so a threshold expressed in percent compares directly against it. A
// zero or negative window is unmeasurable and yields FillPctUnmeasured — the
// divide-by-zero guard that keeps Inf/NaN out of every downstream comparison.
// Over-full readings are not clamped: a launch at 120% is a real signal.
//
// A negative prompt-side total is likewise unmeasurable — it is either the
// guard value PromptTokens returns for a wrapped/negative sum or a nonsense
// count from elsewhere — and degrades to the sentinel rather than to a
// fabricated negative percentage.
func FillPct(promptTokens, window int) float64 {
	if window <= 0 || promptTokens < 0 {
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

// FillWarnWithContributors adds an ordered prompt-side breakdown to a fill
// warning. Missing or invalid contributor data leaves the base warning intact;
// output tokens are excluded because they do not occupy the prompt window.
func FillWarnWithContributors(phase string, pct float64, thresholdPct int, usage cyclestate.TokenUsage) string {
	phase = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, phase)
	base := FillWarn(phase, pct, thresholdPct)
	if base == "" {
		return ""
	}

	contributors := []struct {
		name  string
		value int
	}{
		{"input", usage.Input},
		{"cache_read", usage.CacheRead},
		{"cache_write", usage.CacheWrite},
	}
	for _, contributor := range contributors {
		if contributor.value < 0 {
			return base
		}
	}
	sort.SliceStable(contributors, func(i, j int) bool {
		return contributors[i].value > contributors[j].value
	})

	parts := make([]string, 0, len(contributors))
	for _, contributor := range contributors {
		if contributor.value > 0 {
			parts = append(parts, fmt.Sprintf("%s=%d", contributor.name, contributor.value))
		}
	}
	if len(parts) == 0 {
		return base
	}
	return base + "; contributors (largest first): " + strings.Join(parts, ", ")
}
