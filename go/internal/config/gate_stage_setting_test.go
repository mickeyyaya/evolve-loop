package config_test

import (
	"encoding/json"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func TestGateStageSetting_RoundTripsEveryWordGateStageAccepts(t *testing.T) {
	t.Parallel()

	for _, word := range []string{"0", "off", "shadow", "enforce"} {
		want, ok := config.GateStage(word)
		if !ok {
			t.Fatalf("GateStage refuses %q", word)
		}
		var setting config.GateStageSetting
		if err := json.Unmarshal([]byte(`"`+word+`"`), &setting); err != nil {
			t.Fatalf("decode %q: %v", word, err)
		}
		raw, err := json.Marshal(setting)
		if err != nil {
			t.Fatal(err)
		}
		var again config.GateStageSetting
		if err := json.Unmarshal(raw, &again); err != nil {
			t.Fatalf("re-decode %s: %v", raw, err)
		}
		if got := setting.Stage(); got != want {
			t.Errorf("%q decodes to %v, GateStage says %v", word, got, want)
		}
		if got := again.Stage(); got != want {
			t.Errorf("%q does not round-trip: %s", word, raw)
		}
	}
}

func TestGateStageSetting_RefusesWhatGateStageRefuses(t *testing.T) {
	t.Parallel()

	for _, word := range []string{"advisory", "on", "Enforce"} {
		var setting config.GateStageSetting
		if err := json.Unmarshal([]byte(`"`+word+`"`), &setting); err == nil {
			t.Errorf("%q must be refused", word)
		}
	}
	var notText config.GateStageSetting
	if err := json.Unmarshal([]byte(`3`), &notText); err == nil {
		t.Error("a number must be refused")
	}
}

func TestGateStageSetting_UnsetIsOff(t *testing.T) {
	t.Parallel()

	var setting config.GateStageSetting
	if setting.Stage() != config.StageOff {
		t.Error("the zero setting is off")
	}
}

func TestStage_KeepsItsOwnTextAndHasNoJSONMethods(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(config.StageAdvisory)
	if err != nil || string(raw) != "2" {
		t.Fatalf("Stage marshals as its int: %s %v", raw, err)
	}
	var back config.Stage
	if err := json.Unmarshal([]byte("2"), &back); err != nil || back != config.StageAdvisory {
		t.Fatalf("Stage round-trips advisory: %v %v", back, err)
	}
	if config.StageAdvisory.String() != "advisory" {
		t.Errorf("advisory text changed: %s", config.StageAdvisory)
	}
}
