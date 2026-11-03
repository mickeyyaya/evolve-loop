package ship

import "testing"

func TestPhaseIntegrationWired_ReportsTheFleetIntegrationStage(t *testing.T) {
	t.Parallel()
	if !New(Config{Integration: &Integration{Stage: "shadow"}}).IntegrationWired() {
		t.Fatal("a ship phase built with an integration stage reports it wired")
	}
	if New(Config{}).IntegrationWired() {
		t.Fatal("a solo ship phase reports no integration stage")
	}
}
