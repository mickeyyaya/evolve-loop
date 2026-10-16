package phasetiming

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func TestSummary_TokenRollupAndCacheHitRatio(t *testing.T) {
	entries := []Entry{
		{Phase: "scout", Archetype: "generative", Verdict: "PASS",
			Tokens: cyclestate.TokenUsage{Input: 600, Output: 100, CacheRead: 400, CacheWrite: 20}},
		{Phase: "build", Archetype: "generative", Verdict: "PASS",
			Tokens: cyclestate.TokenUsage{Input: 300, Output: 50, CacheRead: 100, CacheWrite: 10}},
		{Phase: "audit", Archetype: "checking", Verdict: "FAIL",
			Tokens: cyclestate.TokenUsage{Input: 100, Output: 30, CacheRead: 0, CacheWrite: 0}},
	}

	s := Rollup(entries)

	wantTotal := cyclestate.TokenUsage{Input: 1000, Output: 180, CacheRead: 500, CacheWrite: 30}
	if s.TotalTokens != wantTotal {
		t.Fatalf("TotalTokens = %+v, want %+v", s.TotalTokens, wantTotal)
	}
	wantGen := cyclestate.TokenUsage{Input: 900, Output: 150, CacheRead: 500, CacheWrite: 30}
	if s.TokensByArchetype["generative"] != wantGen {
		t.Fatalf("TokensByArchetype[generative] = %+v, want %+v", s.TokensByArchetype["generative"], wantGen)
	}
	wantWaste := cyclestate.TokenUsage{Input: 100, Output: 30, CacheRead: 0, CacheWrite: 0}
	if s.WastedTokens != wantWaste {
		t.Fatalf("WastedTokens = %+v, want %+v", s.WastedTokens, wantWaste)
	}
	if got := s.CacheHitRatio; got < 0.3333 || got > 0.3334 {
		t.Fatalf("CacheHitRatio = %v, want ~0.3333", got)
	}
}

func TestSummary_TokenRollupZeroInput(t *testing.T) {
	s := Rollup([]Entry{{Phase: "scout", Verdict: "PASS"}})
	if s.CacheHitRatio != 0 {
		t.Fatalf("CacheHitRatio = %v, want 0", s.CacheHitRatio)
	}
}
