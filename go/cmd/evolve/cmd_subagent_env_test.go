package main

import "testing"

// Flag-readers for the subagent handlers, migrated onto envchain. The tests pin
// each knob's default — the guard against a default-flip when swapping the
// `!= "0"` (default-on) / `== "1"` (default-off) idioms for envchain.Bool.

func TestReadSubagentRunFlags_Defaults(t *testing.T) {
	for _, k := range []string{"EVOLVE_CACHE_PREFIX_V2", "ADVERSARIAL_AUDIT", "LEGACY_AGENT_DISPATCH"} {
		t.Setenv(k, "")
	}
	f := readSubagentRunFlags()
	if !f.cachePrefixV2 {
		t.Error("cachePrefixV2 default = false, want true (`!= \"0\"`)")
	}
	if !f.adversarialAudit {
		t.Error("adversarialAudit default = false, want true (`!= \"0\"`)")
	}
	if f.legacyAgentDispatch {
		t.Error("legacyAgentDispatch default = true, want false (`== \"1\"`)")
	}
}

func TestReadSubagentRunFlags_Toggle(t *testing.T) {
	t.Setenv("EVOLVE_CACHE_PREFIX_V2", "0")
	t.Setenv("ADVERSARIAL_AUDIT", "0")
	t.Setenv("LEGACY_AGENT_DISPATCH", "1")
	f := readSubagentRunFlags()
	if f.cachePrefixV2 || f.adversarialAudit {
		t.Error(`"0" should disable the default-on flags`)
	}
	if !f.legacyAgentDispatch {
		t.Error(`"1" should enable the default-off flag`)
	}
}

func TestReadDispatchParallelFlags_Defaults(t *testing.T) {
	for _, k := range []string{"EVOLVE_FANOUT_CONCURRENCY", "EVOLVE_FANOUT_CACHE_PREFIX", "EVOLVE_FANOUT_TRACK_WORKERS"} {
		t.Setenv(k, "")
	}
	f := readDispatchParallelFlags()
	if f.concurrency != 2 {
		t.Errorf("concurrency default = %d, want 2", f.concurrency)
	}
	if !f.cachePrefixEnabled || !f.trackWorkers {
		t.Error("cachePrefixEnabled/trackWorkers default = false, want true (`!= \"0\"`)")
	}
}

func TestReadDispatchParallelFlags_ConcurrencyGuard(t *testing.T) {
	// n <= 0 (or invalid) falls back to the default 2 (the legacy `n > 0` guard).
	t.Setenv("EVOLVE_FANOUT_CONCURRENCY", "0")
	if f := readDispatchParallelFlags(); f.concurrency != 2 {
		t.Errorf("concurrency(0) = %d, want 2 (n>0 guard)", f.concurrency)
	}
	t.Setenv("EVOLVE_FANOUT_CONCURRENCY", "-1")
	if f := readDispatchParallelFlags(); f.concurrency != 2 {
		t.Errorf("concurrency(-1) = %d, want 2 (n>0 guard)", f.concurrency)
	}
	t.Setenv("EVOLVE_FANOUT_CONCURRENCY", "garbage")
	if f := readDispatchParallelFlags(); f.concurrency != 2 {
		t.Errorf("concurrency(garbage) = %d, want 2 (fallback)", f.concurrency)
	}
	t.Setenv("EVOLVE_FANOUT_CONCURRENCY", "8")
	if f := readDispatchParallelFlags(); f.concurrency != 8 {
		t.Errorf("concurrency(8) = %d, want 8", f.concurrency)
	}
}
