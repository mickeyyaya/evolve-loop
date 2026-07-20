package modelcatalog

import (
	"reflect"
	"testing"
)

// dispatchmodelchain_test.go — the within-tier model-failover chain accessor:
// the ordered concrete-model preference for (cli, tier), live-gated, deduped.

func TestDispatchModelChain(t *testing.T) {
	cat := Catalog{CLIs: map[string]CLIEntry{
		"claude": {
			Source:        SourceLive,
			TierModels:    map[string]string{"deep": "fable", "balanced": "sonnet"},
			TierFallbacks: map[string][]string{"deep": {"fable", "opus", "sonnet"}},
		},
		"detectonly": {
			Source:        SourceDetect, // not trustworthy for dispatch
			TierModels:    map[string]string{"deep": "x"},
			TierFallbacks: map[string][]string{"deep": {"x", "y"}},
		},
	}}

	// Live: primary first, then chain, DEDUPED (fable is both the primary and
	// the chain head — it must appear once).
	if got, want := cat.DispatchModelChain("claude", "deep"), []string{"fable", "opus", "sonnet"}; !reflect.DeepEqual(got, want) {
		t.Errorf("DispatchModelChain(claude,deep) = %v, want %v", got, want)
	}
	// A tier with only a primary (no fallback chain) → single-element chain.
	if got, want := cat.DispatchModelChain("claude", "balanced"), []string{"sonnet"}; !reflect.DeepEqual(got, want) {
		t.Errorf("DispatchModelChain(claude,balanced) = %v, want %v", got, want)
	}
	// A tier naming no models → empty (caller falls back to the manifest).
	if got := cat.DispatchModelChain("claude", "fast"); len(got) != 0 {
		t.Errorf("DispatchModelChain(claude,fast) = %v, want empty", got)
	}
	// Non-live entry → nil (same trust gate as DispatchModel; must not drive dispatch).
	if got := cat.DispatchModelChain("detectonly", "deep"); got != nil {
		t.Errorf("non-live entry must return nil, got %v", got)
	}
	// Unknown CLI → nil.
	if got := cat.DispatchModelChain("nope", "deep"); got != nil {
		t.Errorf("unknown cli must return nil, got %v", got)
	}
}

// TestDispatchModelChain_FallbackOnlyWhenPrimaryAbsent: when TierModels[tier] is
// empty, the chain still comes from TierFallbacks[tier] (mirrors modelForTier's
// primary-else-chain resolution, but returns the WHOLE chain).
func TestDispatchModelChain_FallbackOnlyWhenPrimaryAbsent(t *testing.T) {
	cat := Catalog{CLIs: map[string]CLIEntry{
		"claude": {
			Source:        SourceLive,
			TierFallbacks: map[string][]string{"deep": {"opus", "sonnet"}},
		},
	}}
	if got, want := cat.DispatchModelChain("claude", "deep"), []string{"opus", "sonnet"}; !reflect.DeepEqual(got, want) {
		t.Errorf("chain with no primary = %v, want %v", got, want)
	}
}
