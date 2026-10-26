package policy

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

func reachablePolicyStructs() []reflect.Type {
	var out []reflect.Type
	seen := map[reflect.Type]bool{}
	var walk func(reflect.Type)
	walk = func(t reflect.Type) {
		for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Map {
			t = t.Elem()
		}
		if shape, ok := keyShapes[t]; ok {
			t = shape
			walk(t)
			return
		}
		if t.Kind() != reflect.Struct || seen[t] {
			return
		}
		seen[t] = true
		out = append(out, t)
		for i := range t.NumField() {
			walk(t.Field(i).Type)
		}
	}
	walk(reflect.TypeFor[Policy]())
	return out
}

func TestKeyWalk_EveryReachableFieldHasTheExactJSONTagTheWalkMatches(t *testing.T) {
	for _, st := range reachablePolicyStructs() {
		for i := range st.NumField() {
			f := st.Field(i)
			tag := f.Tag.Get("json")
			if f.Anonymous || (f.IsExported() && (tag == "" || tag == "-")) {
				t.Errorf("%s.%s: the key walk needs a named json tag on each exported field and no embedded struct (tag %q)", st, f.Name, tag)
			}
		}
	}
}

func TestKeyWalk_EachCustomDecoderIsAReviewedShape(t *testing.T) {
	unmarshaler := reflect.TypeFor[json.Unmarshaler]()
	var got []string
	for _, st := range reachablePolicyStructs() {
		if reflect.PointerTo(st).Implements(unmarshaler) {
			got = append(got, st.Name())
		}
	}
	slices.Sort(got)
	want := []string{"AgentRule", "AttemptPostmortemPolicy", "CLIRouting", "CheckpointPolicy", "CodeIndexPolicy", "ConvergencePolicy", "DocsFloorPolicy", "FleetOrchestratorPolicy", "HeldProposalsPolicy", "InboxCuratorPolicy", "InboxPriorityFactors", "InboxPriorityPolicy", "PracticeWatchPolicy", "RegressionTIAPolicy", "ResumeConfig", "RetryPolicy", "TierRule"}
	if !slices.Equal(got, want) {
		t.Fatalf("a custom UnmarshalJSON whose wire keys differ from its json tags needs a keyShapes entry; review the new set %v against %v", got, want)
	}
}
