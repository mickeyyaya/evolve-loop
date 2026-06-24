package policy_test

// FanoutPolicy — the typed parameters that replaced EVOLVE_FANOUT_*. The
// accessor encodes the non-obvious rules: Concurrency overrides only when >=1
// (0/negative → default 2, NOT 1); int fields pass 0 through as a downstream
// sentinel; *bool fields preserve an explicit false vs nil→true.

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fanoutdispatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// fanoutWant is the resolved expectation (pointers flattened to bools).
type fanoutWant struct {
	concurrency        int
	timeoutSecs        int
	cancelOnConsensus  bool
	consensusK         int
	consensusPollSecs  int
	trackWorkers       bool
	cachePrefixEnabled bool
	testExecutor       string
}

func assertFanout(t *testing.T, got policy.FanoutPolicy, want fanoutWant) {
	t.Helper()
	if got.Concurrency != want.concurrency {
		t.Errorf("Concurrency = %d, want %d", got.Concurrency, want.concurrency)
	}
	if got.TimeoutSecs != want.timeoutSecs {
		t.Errorf("TimeoutSecs = %d, want %d", got.TimeoutSecs, want.timeoutSecs)
	}
	if got.CancelOnConsensus != want.cancelOnConsensus {
		t.Errorf("CancelOnConsensus = %v, want %v", got.CancelOnConsensus, want.cancelOnConsensus)
	}
	if got.ConsensusK != want.consensusK {
		t.Errorf("ConsensusK = %d, want %d", got.ConsensusK, want.consensusK)
	}
	if got.ConsensusPollSecs != want.consensusPollSecs {
		t.Errorf("ConsensusPollSecs = %d, want %d", got.ConsensusPollSecs, want.consensusPollSecs)
	}
	if v := derefBool(t, "TrackWorkers", got.TrackWorkers); v != want.trackWorkers {
		t.Errorf("TrackWorkers = %v, want %v", v, want.trackWorkers)
	}
	if v := derefBool(t, "CachePrefixEnabled", got.CachePrefixEnabled); v != want.cachePrefixEnabled {
		t.Errorf("CachePrefixEnabled = %v, want %v", v, want.cachePrefixEnabled)
	}
	if got.TestExecutor != want.testExecutor {
		t.Errorf("TestExecutor = %q, want %q", got.TestExecutor, want.testExecutor)
	}
}

func TestFanoutConfig_Resolution(t *testing.T) {
	defaults := fanoutWant{concurrency: 2, trackWorkers: true, cachePrefixEnabled: true}
	cases := []struct {
		name string
		pol  policy.Policy
		want fanoutWant
	}{
		{"absent-defaults", policy.Policy{}, defaults},
		{"empty-block-defaults", policy.Policy{Fanout: &policy.FanoutPolicy{}}, defaults},
		{"concurrency-zero-falls-to-default", policy.Policy{Fanout: &policy.FanoutPolicy{Concurrency: 0}}, defaults},
		{"concurrency-negative-falls-to-default", policy.Policy{Fanout: &policy.FanoutPolicy{Concurrency: -3}}, defaults},
		{"concurrency-one-boundary", policy.Policy{Fanout: &policy.FanoutPolicy{Concurrency: 1}}, fanoutWant{concurrency: 1, trackWorkers: true, cachePrefixEnabled: true}},
		{"concurrency-five", policy.Policy{Fanout: &policy.FanoutPolicy{Concurrency: 5}}, fanoutWant{concurrency: 5, trackWorkers: true, cachePrefixEnabled: true}},
		{"int-sentinels-passthrough", policy.Policy{Fanout: &policy.FanoutPolicy{TimeoutSecs: 30, ConsensusK: 3, ConsensusPollSecs: 2, CancelOnConsensus: true}}, fanoutWant{concurrency: 2, timeoutSecs: 30, consensusK: 3, consensusPollSecs: 2, cancelOnConsensus: true, trackWorkers: true, cachePrefixEnabled: true}},
		{"track-workers-explicit-false", policy.Policy{Fanout: &policy.FanoutPolicy{TrackWorkers: boolPtr(false)}}, fanoutWant{concurrency: 2, trackWorkers: false, cachePrefixEnabled: true}},
		{"cache-prefix-explicit-false", policy.Policy{Fanout: &policy.FanoutPolicy{CachePrefixEnabled: boolPtr(false)}}, fanoutWant{concurrency: 2, trackWorkers: true, cachePrefixEnabled: false}},
		{"test-executor", policy.Policy{Fanout: &policy.FanoutPolicy{TestExecutor: "harness"}}, fanoutWant{concurrency: 2, trackWorkers: true, cachePrefixEnabled: true, testExecutor: "harness"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertFanout(t, tc.pol.FanoutConfig(), tc.want)
		})
	}
}

func TestLoad_FanoutBlock(t *testing.T) {
	t.Run("explicit-false-survives-json-round-trip", func(t *testing.T) {
		// track_workers:false stays false; cache_prefix_enabled:false stays false.
		json := `{"fanout":{"concurrency":4,"timeout_secs":30,"cancel_on_consensus":true,` +
			`"consensus_k":3,"consensus_poll_secs":2,"track_workers":false,` +
			`"cache_prefix_enabled":false,"test_executor":"harness"}}`
		pol, err := policy.Load(writeTempPolicy(t, json))
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		assertFanout(t, pol.FanoutConfig(), fanoutWant{
			concurrency: 4, timeoutSecs: 30, cancelOnConsensus: true, consensusK: 3,
			consensusPollSecs: 2, trackWorkers: false, cachePrefixEnabled: false, testExecutor: "harness",
		})
	})

	t.Run("concurrency-zero-in-json-clamps-to-default", func(t *testing.T) {
		// concurrency:0 via JSON still clamps up to the default 2.
		pol, err := policy.Load(writeTempPolicy(t, `{"fanout":{"concurrency":0}}`))
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if got := pol.FanoutConfig().Concurrency; got != 2 {
			t.Errorf("concurrency:0 in JSON → Concurrency = %d, want 2", got)
		}
	})
}

// TestFanoutConfig_WiringToDispatch documents the cmd_fanout_dispatch mapping:
// FanoutConfig() feeds fanoutdispatch.Config, dereferencing *TrackWorkers. The
// accessor's never-nil guarantee is what makes that deref panic-free.
func TestFanoutConfig_WiringToDispatch(t *testing.T) {
	fc := policy.Policy{}.FanoutConfig() // absent policy → resolved defaults
	cfg := fanoutdispatch.Config{
		Concurrency:       fc.Concurrency,
		TimeoutSecs:       fc.TimeoutSecs,
		CancelOnConsensus: fc.CancelOnConsensus,
		ConsensusK:        fc.ConsensusK,
		ConsensusPollSecs: fc.ConsensusPollSecs,
		TrackWorkers:      derefBool(t, "TrackWorkers", fc.TrackWorkers),
	}
	// fanoutdispatch.Config has a func field (Now) so it is not ==-comparable;
	// assert the six default-resolved wired fields explicitly.
	if cfg.Concurrency != 2 || cfg.TimeoutSecs != 0 || cfg.CancelOnConsensus ||
		cfg.ConsensusK != 0 || cfg.ConsensusPollSecs != 0 || !cfg.TrackWorkers {
		t.Errorf("wired fanoutdispatch.Config = %+v, want default-resolved {Concurrency:2, TrackWorkers:true, rest zero}", cfg)
	}
}
