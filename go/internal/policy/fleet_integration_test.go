package policy

import (
	"errors"
	"strings"
	"testing"
	"time"
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

func TestPullRefWait_DefaultsAndRejectsNegative(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, json string
		want       time.Duration
		wantErr    bool
	}{
		{"absent fleet block uses default", `{}`, DefaultPullRefWaitS * time.Second, false},
		{"absent integration uses default", `{"fleet":{"count":4}}`, DefaultPullRefWaitS * time.Second, false},
		{"absent key uses default", `{"fleet":{"integration":{}}}`, DefaultPullRefWaitS * time.Second, false},
		{"zero disables wait", `{"fleet":{"integration":{"pull_ref_wait_s":0}}}`, 0, false},
		{"positive seconds", `{"fleet":{"integration":{"pull_ref_wait_s":12}}}`, 12 * time.Second, false},
		{"negative fails closed", `{"fleet":{"integration":{"pull_ref_wait_s":-1}}}`, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := loadCIWatchPolicy(t, c.json).PullRefWait()
			if (err != nil) != c.wantErr || got != c.want {
				t.Fatalf("PullRefWait() = %v, %v; want %v, err=%v", got, err, c.want, c.wantErr)
			}
			if err != nil && !errors.Is(err, errInvalidPullRefWait) {
				t.Errorf("the error must wrap errInvalidPullRefWait: %v", err)
			}
		})
	}
	secs := 9
	literal := Policy{Fleet: &FleetPolicy{Integration: &FleetIntegrationPolicy{PullRefWaitS: &secs}}}
	if got, err := literal.PullRefWait(); err != nil || got != 9*time.Second {
		t.Errorf("a literal PullRefWaitS must resolve: %v, %v", got, err)
	}
}
