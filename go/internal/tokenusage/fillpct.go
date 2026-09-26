package tokenusage

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

// FillPctUnmeasured marks a fill that could not be derived; it is negative so no real reading or threshold check matches it.
const FillPctUnmeasured = -1.0

// claudeEffectiveWindow sits below every advertised maximum: the useful signal is nearness to degradation, not the ceiling.
const claudeEffectiveWindow = 200_000

// promptTokensUnmeasured is negative so FillPct turns it into FillPctUnmeasured.
const promptTokensUnmeasured = -1

// PromptTokens sums the counters that occupy the window (input and both cache halves), or returns a negative total on a negative counter or overflow.
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

// windowOccupancy returns the fullest single turn when the tier reports turns, else the tier's one-envelope total.
// Summing turns would count the accumulated context once per turn; a negative peak passes through to the sentinel.
func windowOccupancy(r Result) int {
	if r.PeakPromptTokens != 0 {
		return r.PeakPromptTokens
	}
	return PromptTokens(r.Usage)
}

// EffectiveWindow returns a driver family's effective context window, or 0 when nobody has measured it.
func EffectiveWindow(driver string) int {
	if isClaudeDriver(driver) {
		return claudeEffectiveWindow
	}
	// Exact identities only: a prefix match would give an unmeasured lookalike CLI a guessed window.
	switch driver {
	case "codex", "codex-tmux", "agy", "agy-tmux":
		return claudeEffectiveWindow
	}
	return 0
}

// FillPct returns promptTokens as an unclamped percentage of window, or FillPctUnmeasured for a non-positive window or negative count.
func FillPct(promptTokens, window int) float64 {
	if window <= 0 || promptTokens < 0 {
		return FillPctUnmeasured
	}
	return float64(promptTokens) / float64(window) * 100
}

// FillWarn returns a phase-named warning for a reading strictly above thresholdPct, and "" otherwise or for any negative reading.
func FillWarn(phase string, pct float64, thresholdPct int) string {
	if pct == FillPctUnmeasured || pct < 0 {
		return ""
	}
	if pct <= float64(thresholdPct) {
		return ""
	}
	return fmt.Sprintf("context fill %.1f%% for phase %s exceeds the %d%% warn threshold — this launch is close to compaction", pct, phase, thresholdPct)
}

// FillWarnWithContributors appends usage's prompt-side counters, largest first, to FillWarn's message; negative counters drop the breakdown.
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
