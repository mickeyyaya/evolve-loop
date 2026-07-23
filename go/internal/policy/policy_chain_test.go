package policy

import (
	"os"
	"path/filepath"
	"testing"
)

// TestChainConfig_DefaultsAndOverride pins the compiled-default-then-override
// contract of the `chain` block. The edge case is load-bearing: a
// non-positive max_batches must NOT resolve to a 0 cap, which would silently
// refuse to run a single batch for an operator who explicitly asked to chain.
func TestChainConfig_DefaultsAndOverride(t *testing.T) {
	t.Parallel()
	yes, no := true, false
	tests := []struct {
		name           string
		chain          *ChainPolicy
		wantEnabled    bool
		wantMaxBatches int
	}{
		{"absent block defaults off with positive cap", nil, false, DefaultChainMaxBatches},
		{"explicit enable honoured", &ChainPolicy{Enabled: &yes}, true, DefaultChainMaxBatches},
		{"explicit disable honoured", &ChainPolicy{Enabled: &no}, false, DefaultChainMaxBatches},
		{"explicit cap honoured", &ChainPolicy{Enabled: &yes, MaxBatches: 7}, true, 7},
		{"zero cap falls back to default", &ChainPolicy{Enabled: &yes, MaxBatches: 0}, true, DefaultChainMaxBatches},
		{"negative cap falls back to default", &ChainPolicy{Enabled: &yes, MaxBatches: -3}, true, DefaultChainMaxBatches},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := (Policy{Chain: tc.chain}).ChainConfig()
			if got.Enabled != tc.wantEnabled || got.MaxBatches != tc.wantMaxBatches {
				t.Fatalf("ChainConfig() = {Enabled:%v MaxBatches:%d}, want {Enabled:%v MaxBatches:%d}",
					got.Enabled, got.MaxBatches, tc.wantEnabled, tc.wantMaxBatches)
			}
			if got.MaxBatches <= 0 {
				t.Fatalf("resolved MaxBatches must always be positive, got %d", got.MaxBatches)
			}
		})
	}
}

// TestDefaultChainMaxBatches pins the compiled backstop itself: it must be a
// positive runaway bound, not an accidental 0/negative that would make every
// chained invocation a no-op.
func TestDefaultChainMaxBatches(t *testing.T) {
	t.Parallel()
	if DefaultChainMaxBatches <= 1 {
		t.Fatalf("DefaultChainMaxBatches = %d; the compiled cap must allow at least two chained batches", DefaultChainMaxBatches)
	}
}

// TestChainConfig_FromJSON exercises the block through the real loader so the
// json tags (`chain`, `enabled`, `max_batches`) are part of the contract, not
// just the Go struct.
func TestChainConfig_FromJSON(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(`{"chain":{"enabled":true,"max_batches":4}}`), 0o644); err != nil {
		t.Fatalf("write policy fixture: %v", err)
	}
	p, err := Load(path)
	if err != nil {
		t.Fatalf("load chain policy: %v", err)
	}
	got := p.ChainConfig()
	if !got.Enabled || got.MaxBatches != 4 {
		t.Fatalf("ChainConfig() from JSON = %+v, want {Enabled:true MaxBatches:4}", got)
	}
}
