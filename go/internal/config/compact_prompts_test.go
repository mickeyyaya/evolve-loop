package config

import (
	"reflect"
	"testing"
)

// compact_prompts_test.go — Adversarial amplification for CompactPrompts config behavior.
//
// Probes the config pipeline for the cycle-413 Task B CompactPrompts field:
// - Default (true) when compact_prompts absent from registry
// - Explicit false overrides the default
// - Whole workflow block absent → default still applies
// - Registry entirely absent → default applies (from defaults())
//
// These are distinct from ACS C413_004/C413_005 which test field-presence
// and the explicit-true case. These tests target the silent-false regression
// path: a field that defaults true can silently become false if parse errors
// or missing keys are not handled correctly.

func TestCompactPrompts_DefaultTrue_WhenKeyAbsent(t *testing.T) {
	// workflow block present but compact_prompts key absent → default=true
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
	// no workflow block at all → default=true
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
	// compact_prompts=false must override the default-true
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
	// no registry file → defaults() must set CompactPrompts=true
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
	// Sanity: explicit true is accepted and preserved (same as default but
	// exercises the parse→populate path end-to-end).
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
