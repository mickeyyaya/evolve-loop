package config

import (
	"reflect"
	"testing"
)

func TestLoad_DeepBuildRiskClassesAreNormalizedFromTheRegistry(t *testing.T) {
	reg := writeRegistry(t, `{"config":{"deep_build_risk":{"sec":[" Secret ","","TOKEN"],"empty":["  "]}},"phases":[]}`)
	cfg, ws := Load(reg)
	if len(ws) != 0 {
		t.Fatalf("unexpected warnings: %+v", ws)
	}
	want := map[string][]string{"sec": {"secret", "token"}}
	if !reflect.DeepEqual(cfg.DeepBuildRisk, want) {
		t.Errorf("DeepBuildRisk = %+v, want %+v", cfg.DeepBuildRisk, want)
	}
}

func TestLoad_NoDeepBuildRiskBlockLeavesTheRuleOff(t *testing.T) {
	cfg, _ := Load(writeRegistry(t, `{"config":{},"phases":[]}`))
	if cfg.DeepBuildRisk != nil {
		t.Errorf("DeepBuildRisk = %+v, want nil", cfg.DeepBuildRisk)
	}
}
