package main

import "testing"

// fanoutEnvConfig reads the EVOLVE_FANOUT_* knobs through envchain. These tests
// pin the truthy/falsy/default parsing of each migrated key — the guard against
// a default-flip (e.g. a default-on `!= "0"` flag becoming default-off) during
// the os.Getenv -> envchain migration.

func TestFanoutEnvConfig_Defaults(t *testing.T) {
	// All knobs unset -> documented defaults.
	for _, k := range []string{
		"EVOLVE_FANOUT_CONCURRENCY", "EVOLVE_FANOUT_TIMEOUT",
		"EVOLVE_FANOUT_CANCEL_ON_CONSENSUS", "EVOLVE_FANOUT_CONSENSUS_K",
		"EVOLVE_FANOUT_CONSENSUS_POLL_S", "EVOLVE_FANOUT_TRACK_WORKERS",
	} {
		t.Setenv(k, "")
	}
	c := fanoutEnvConfig()
	if c.Concurrency != 0 || c.TimeoutSecs != 0 || c.ConsensusK != 0 || c.ConsensusPollSecs != 0 {
		t.Errorf("int defaults = {%d,%d,%d,%d}, want all 0", c.Concurrency, c.TimeoutSecs, c.ConsensusK, c.ConsensusPollSecs)
	}
	if c.CancelOnConsensus {
		t.Errorf("CancelOnConsensus default = true, want false (default-off `== \"1\"` flag)")
	}
	if !c.TrackWorkers {
		t.Errorf("TrackWorkers default = false, want true (default-on flag)")
	}
}

func TestFanoutEnvConfig_Parsing(t *testing.T) {
	t.Setenv("EVOLVE_FANOUT_CONCURRENCY", "4")
	t.Setenv("EVOLVE_FANOUT_TIMEOUT", "600")
	t.Setenv("EVOLVE_FANOUT_CONSENSUS_K", "2")
	t.Setenv("EVOLVE_FANOUT_CONSENSUS_POLL_S", "5")
	t.Setenv("EVOLVE_FANOUT_CANCEL_ON_CONSENSUS", "1") // truthy
	t.Setenv("EVOLVE_FANOUT_TRACK_WORKERS", "0")       // falsy -> disables a default-on flag
	c := fanoutEnvConfig()
	if c.Concurrency != 4 || c.TimeoutSecs != 600 || c.ConsensusK != 2 || c.ConsensusPollSecs != 5 {
		t.Errorf("ints = {%d,%d,%d,%d}, want {4,600,2,5}", c.Concurrency, c.TimeoutSecs, c.ConsensusK, c.ConsensusPollSecs)
	}
	if !c.CancelOnConsensus {
		t.Errorf("CancelOnConsensus(\"1\") = false, want true")
	}
	if c.TrackWorkers {
		t.Errorf("TrackWorkers(\"0\") = true, want false")
	}
}

func TestFanoutEnvConfig_InvalidIntFallsBack(t *testing.T) {
	t.Setenv("EVOLVE_FANOUT_CONCURRENCY", "not-a-number")
	if c := fanoutEnvConfig(); c.Concurrency != 0 {
		t.Errorf("Concurrency(garbage) = %d, want 0 (fallback)", c.Concurrency)
	}
}
