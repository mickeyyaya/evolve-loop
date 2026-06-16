package phaseconfig

import "testing"

// TestPhaseConfig_LoadDecomposition names the phaseconfig.PhaseConfig type (Load
// returns it but it is never named in a test) and pins the decomposition
// contract: one loaded config exposes an agreeing PhaseSpec view, profile name,
// in-band prompt, and swarm-worker count.
func TestPhaseConfig_LoadDecomposition(t *testing.T) {
	got, err := Load(writeCfg(t, sample))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	want := PhaseConfig{SwarmWorkers: 3}
	if got.SwarmWorkers != want.SwarmWorkers {
		t.Errorf("SwarmWorkers = %d, want %d", got.SwarmWorkers, want.SwarmWorkers)
	}
	if got.Spec().Name != "security-scan" {
		t.Errorf("Spec().Name = %q, want security-scan", got.Spec().Name)
	}
	if got.ProfileName() != "security-scanner" {
		t.Errorf("ProfileName() = %q, want security-scanner", got.ProfileName())
	}
	if body, ok := got.PromptBody(); !ok || body == "" {
		t.Errorf("PromptBody() = %q,%v; want non-empty,true (sample carries an in-band prompt)", body, ok)
	}
}
