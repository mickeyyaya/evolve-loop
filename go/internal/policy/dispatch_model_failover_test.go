package policy

import "testing"

func TestBridgeModelFailoverOn_OnlyTheWordOnTurnsItOn(t *testing.T) {
	for word, want := range map[string]bool{ModelFailoverStageOn: true, "": false, "off": false, "enforce": false, "On": false} {
		if got := (BridgePolicy{ModelFailover: word}).ModelFailoverOn(); got != want {
			t.Errorf("ModelFailover %q: ModelFailoverOn() = %v, want %v", word, got, want)
		}
	}
	if (Policy{}).BridgeConfig().ModelFailoverOn() {
		t.Error("an absent bridge block must leave the model failover off")
	}
}
