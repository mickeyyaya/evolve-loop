package bridge

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

func TestResolveAttemptTokens_TheAgyConversationStoreIsAMeasuredUsage(t *testing.T) {
	t.Parallel()
	usage := cyclestate.TokenUsage{Input: 554617, Output: 69220, CacheRead: 13904571}
	e := NewEngine(Deps{TokenResolver: func(w tokenusage.Window) (tokenusage.Result, error) {
		return tokenusage.Result{Usage: usage, Source: tokenusage.SourceAgyConversation, FillPct: 40}, nil
	}})
	start := time.Date(2026, 10, 19, 7, 7, 30, 0, time.UTC)

	got, status := e.resolveAttemptTokens(core.BridgeRequest{CLI: "agy-tmux"}, start, start.Add(16*time.Minute), "c1", 1)

	if status != llmcalls.UsageMeasured || got.Usage != usage || got.FillPct != 40 {
		t.Errorf("result = %+v status %q, want the agy usage recorded as measured", got, status)
	}
}
