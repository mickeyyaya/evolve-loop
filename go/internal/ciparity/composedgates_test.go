package ciparity

import (
	"reflect"
	"testing"
)

func TestMissingComposedGates_FullNativeGateSet(t *testing.T) {
	t.Parallel()
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

	partial := map[string]string{"compile": "pass"}
	if got := MissingComposedGates(partial); len(got) != len(RequiredComposedGates)-1 {
		t.Fatalf("subset gate map must report all absent gates, got %v", got)
	}

	if got := MissingComposedGates(nil); len(got) != len(RequiredComposedGates) {
		t.Fatalf("nil gate map must report the full required set, got %v", got)
	}
}

func TestGateStatuses_ProjectsStatusOnly(t *testing.T) {
	t.Parallel()
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
