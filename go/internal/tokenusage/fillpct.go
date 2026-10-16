package tokenusage

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

const FillPctUnmeasured = -1.0

const claudeEffectiveWindow = 200_000

const promptTokensUnmeasured = -1

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

func windowOccupancy(r Result) int {
	if r.PeakPromptTokens != 0 {
		return r.PeakPromptTokens
	}
	return PromptTokens(r.Usage)
}

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

func FillPct(promptTokens, window int) float64 {
	if window <= 0 || promptTokens < 0 {
		return FillPctUnmeasured
	}
	return float64(promptTokens) / float64(window) * 100
}

func FillWarn(phase string, pct float64, thresholdPct int) string {
	if pct < 0 || pct <= float64(thresholdPct) {
		return ""
	}
	return fmt.Sprintf("context fill %.1f%% for phase %s exceeds the %d%% warn threshold — this launch is close to compaction", pct, phase, thresholdPct)
}

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
