package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderTripwires_TheHeadlineNamesTheCollectorRuleNotTheClaudeFamily(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer

	renderTripwires(&out, TokensReport{TripwireCount: 2, Tripwires: []TripwireEvent{{Cycle: 5, CLI: "claude-tmux"}, {Cycle: 6, CLI: "agy-tmux"}}})

	if text := out.String(); strings.Contains(text, "non-claude") || !strings.Contains(text, "2 success launch(es) of a family with its own usage collector resolved to source=none") {
		t.Errorf("headline = %q; the tripwire counts claude and agy launches whose collector found nothing, so it must not say non-claude", text)
	}
}
