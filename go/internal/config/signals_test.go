package config

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// Test 24 — signalCodes is the ONE projection from the legacy Warning.Code
// vocabulary onto the registered signal codes: six pairs, every value owned by
// module config with a doc that names fields.step, no phantoms.
func TestSignalCodes_TableIsTheOneProjectionWithNoPhantoms(t *testing.T) {
	want := map[string]signalcenter.Code{
		codeUnknownValue:       CodeUnknownValue,
		codeWeakSpine:          CodeWeakSpine,
		codeSpineOrder:         CodeSpineOrder,
		codeInertPhaseEnable:   CodeInertPhaseEnable,
		codeRegistryUnreadable: CodeRegistryUnreadable,
		codeRegistryMalformed:  CodeRegistryMalformed,
	}
	if len(signalCodes) != 6 {
		t.Fatalf("six codes, got %d: %v", len(signalCodes), signalCodes)
	}
	for legacy, code := range want {
		if signalCodes[legacy] != code {
			t.Errorf("signalCodes[%q] = %q, want %q", legacy, signalCodes[legacy], code)
		}
	}
	docs := map[signalcenter.Code]string{}
	for _, cd := range signalcenter.RegisteredCodes()[signalcenter.ModuleConfig] {
		docs[cd.Code] = cd.Doc
	}
	for _, code := range signalCodes {
		if m, ok := signalcenter.IsRegistered(code); !ok || m != signalcenter.ModuleConfig {
			t.Errorf("%s registered under %q (ok=%v), want module config", code, m, ok)
		}
		if !strings.Contains(docs[code], "fields.step") {
			t.Errorf("%s doc must name fields.step: %q", code, docs[code])
		}
	}
	if c := signalcenter.RegistryConflicts(); len(c) != 0 {
		t.Errorf("registry conflicts: %+v", c)
	}
	for _, phantom := range []string{"unknown-key", "deprecated-flag"} {
		if _, ok := signalCodes[phantom]; ok {
			t.Errorf("%q has no producer and must not be registered", phantom)
		}
	}
	if _, ok := signalcenter.IsRegistered("CONFIG_UNKNOWN_KEY"); ok {
		t.Error("CONFIG_UNKNOWN_KEY is a phantom")
	}
}
