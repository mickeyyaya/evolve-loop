package config

import (
	"reflect"
	"testing"
)

func TestCompactPrompts_DefaultTrue_WhenKeyAbsent(t *testing.T) {
	reg := writeRegistry(t, `{
	  "config": {"dynamic_routing": "advisory", "workflow": {}},
	  "phases": []
	}`)
	cfg, warns := Load(reg, map[string]string{})
	for _, w := range warns {
		t.Logf("warn: %s: %s", w.Code, w.Message)
	}
	field := reflect.ValueOf(cfg).FieldByName("CompactPrompts")
	if !field.IsValid() {
		t.Fatalf("CompactPrompts field absent from RoutingConfig")
	}
	if !field.Bool() {
		t.Errorf("CompactPrompts = false when key absent, want true (default-on)")
	}
}

func TestCompactPrompts_DefaultTrue_WhenWorkflowBlockAbsent(t *testing.T) {
	reg := writeRegistry(t, `{
	  "config": {"dynamic_routing": "advisory"},
	  "phases": []
	}`)
	cfg, warns := Load(reg, map[string]string{})
	for _, w := range warns {
		t.Logf("warn: %s: %s", w.Code, w.Message)
	}
	field := reflect.ValueOf(cfg).FieldByName("CompactPrompts")
	if !field.IsValid() {
		t.Fatalf("CompactPrompts field absent from RoutingConfig")
	}
	if !field.Bool() {
		t.Errorf("CompactPrompts = false when workflow block absent, want true (default-on)")
	}
}

func TestCompactPrompts_ExplicitFalse_Overrides(t *testing.T) {
	reg := writeRegistry(t, `{
	  "config": {"dynamic_routing": "advisory", "workflow": {"compact_prompts": false}},
	  "phases": []
	}`)
	cfg, warns := Load(reg, map[string]string{})
	for _, w := range warns {
		t.Logf("warn: %s: %s", w.Code, w.Message)
	}
	field := reflect.ValueOf(cfg).FieldByName("CompactPrompts")
	if !field.IsValid() {
		t.Fatalf("CompactPrompts field absent from RoutingConfig")
	}
	if field.Bool() {
		t.Errorf("CompactPrompts = true when registry sets compact_prompts=false, want false")
	}
}

func TestCompactPrompts_DefaultTrue_WhenRegistryMissing(t *testing.T) {
	cfg, warns := Load("nonexistent-path/registry.json", map[string]string{})
	for _, w := range warns {
		t.Logf("warn: %s: %s", w.Code, w.Message)
	}
	field := reflect.ValueOf(cfg).FieldByName("CompactPrompts")
	if !field.IsValid() {
		t.Fatalf("CompactPrompts field absent from RoutingConfig")
	}
	if !field.Bool() {
		t.Errorf("CompactPrompts = false on missing registry, want true (defaults() must set true)")
	}
}

func TestCompactPrompts_ExplicitTrue_RoundTrips(t *testing.T) {
	reg := writeRegistry(t, `{
	  "config": {"dynamic_routing": "advisory", "workflow": {"compact_prompts": true}},
	  "phases": []
	}`)
	cfg, warns := Load(reg, map[string]string{})
	for _, w := range warns {
		t.Logf("warn: %s: %s", w.Code, w.Message)
	}
	field := reflect.ValueOf(cfg).FieldByName("CompactPrompts")
	if !field.IsValid() {
		t.Fatalf("CompactPrompts field absent from RoutingConfig")
	}
	if !field.Bool() {
		t.Errorf("CompactPrompts = false when registry sets compact_prompts=true, want true")
	}
}
