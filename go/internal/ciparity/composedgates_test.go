package ciparity

import (
	"reflect"
	"testing"
)

// TestMissingComposedGates_FullNativeGateSet names MissingComposedGates and
// RequiredComposedGates (cycle-786): the composed-tree gate contract for the
// trivial-rebase carry-forward — absent keys and non-"pass" values both keep
// the fast path closed; only the complete green set returns nil.
func TestMissingComposedGates_FullNativeGateSet(t *testing.T) {
	green := map[string]string{}
	for _, g := range RequiredComposedGates {
		green[g] = "pass"
	}
	if got := MissingComposedGates(green); got != nil {
		t.Fatalf("full green gate set must return nil, got %v", got)
	}

	failed := map[string]string{"compile": "pass", "test": "fail", "acs": "pass", "apicover": "pass"}
	if got := MissingComposedGates(failed); !reflect.DeepEqual(got, []string{"test"}) {
		t.Fatalf("test=fail must be reported missing, got %v", got)
	}

	// An entry recording only a subset must report every absent gate — a
	// writer cannot narrow the native gate set by omission.
	partial := map[string]string{"compile": "pass"}
	if got := MissingComposedGates(partial); len(got) != len(RequiredComposedGates)-1 {
		t.Fatalf("subset gate map must report all absent gates, got %v", got)
	}

	if got := MissingComposedGates(nil); len(got) != len(RequiredComposedGates) {
		t.Fatalf("nil gate map must report the full required set, got %v", got)
	}
}

// TestGateStatuses_ProjectsStatusOnly names GateOutcome and GateStatuses
// (composed-gate-decline-coded-signal, cycle 1772): a gate runner widened to
// carry each gate's captured output tail must still be able to derive the
// status-only map MissingComposedGates and the ledger's GateResults expect.
func TestGateStatuses_ProjectsStatusOnly(t *testing.T) {
	outcomes := map[string]GateOutcome{
		"compile": {Status: "pass"},
		"test":    {Status: "fail", Tail: "boom"},
	}
	got := GateStatuses(outcomes)
	want := map[string]string{"compile": "pass", "test": "fail"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GateStatuses(%+v) = %v, want %v", outcomes, got, want)
	}
	if got := GateStatuses(nil); len(got) != 0 {
		t.Fatalf("GateStatuses(nil) = %v, want empty", got)
	}
}
