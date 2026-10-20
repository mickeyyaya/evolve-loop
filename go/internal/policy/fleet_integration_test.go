package policy

import (
	"strings"
	"testing"
)

func TestFleetIntegrationStage_DefaultsShadowAndRejectsUnknownWords(t *testing.T) {
	cases := []struct {
		name, json, want string
		wantErr          bool
	}{
		{"absent fleet block is shadow", `{}`, FleetIntegrationShadow, false},
		{"absent integration is shadow", `{"fleet":{"count":4}}`, FleetIntegrationShadow, false},
		{"empty stage is shadow", `{"fleet":{"integration":{}}}`, FleetIntegrationShadow, false},
		{"shadow", `{"fleet":{"integration":{"stage":"shadow"}}}`, FleetIntegrationShadow, false},
		{"enforce", `{"fleet":{"integration":{"stage":"enforce"}}}`, FleetIntegrationEnforce, false},
		{"unknown word fails closed", `{"fleet":{"integration":{"stage":"on"}}}`, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := loadCIWatchPolicy(t, c.json).FleetIntegrationStage()
			if (err != nil) != c.wantErr || got != c.want {
				t.Fatalf("FleetIntegrationStage() = %q, %v; want %q, err=%v", got, err, c.want, c.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "fleet.integration.stage") {
				t.Errorf("the error must name the key: %v", err)
			}
		})
	}
	literal := Policy{Fleet: &FleetPolicy{Integration: &FleetIntegrationPolicy{Stage: FleetIntegrationEnforce}}}
	if got, err := literal.FleetIntegrationStage(); err != nil || got != FleetIntegrationEnforce {
		t.Errorf("a literal FleetIntegrationPolicy must resolve: %q, %v", got, err)
	}
}
