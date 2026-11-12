package baseattr

import (
	"reflect"
	"slices"
	"testing"
)

func debugMap(t *testing.T, kv []string) map[string]string {
	t.Helper()
	if len(kv)%2 != 0 {
		t.Fatalf("Debug = %v, want pairs", kv)
	}
	m := map[string]string{}
	for i := 0; i < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	return m
}

func TestVerdictDebug_CarriesTheBaseRedAndTheMainRedKeysApart(t *testing.T) {
	t.Parallel()
	dup := Key{Package: "m/internal/dup", Test: "TestCount"}
	size := Key{Package: "m/internal/size", Test: "TestSize"}
	sent := Verdict{BaseRed: []Key{dup, size}, MainRed: []Key{size}, Base: "7aa5456e2", Main: "9a2bbe0e1"}

	got := VerdictOfDebug(debugMap(t, sent.Debug()))

	if !reflect.DeepEqual(got, sent) {
		t.Errorf("VerdictOfDebug(Debug) = %+v, want %+v", got, sent)
	}
	if got := debugMap(t, sent.Debug())["keys"]; got != "m/internal/dup.TestCount, m/internal/size.TestSize" {
		t.Errorf("keys = %q, want the base-red names that the ship signal and the operator read", got)
	}
}

func TestVerdictOfDebug_AnUnjudgedMainHasNoMainRedKeyAndNoMainCommit(t *testing.T) {
	t.Parallel()
	sent := Verdict{BaseRed: []Key{{Package: "m/a", Test: "TestOne"}}, Base: "7aa5456e2"}

	got := VerdictOfDebug(debugMap(t, sent.Debug()))

	if got.MainRed != nil || got.Main != "" || len(got.BaseRed) != 1 {
		t.Errorf("VerdictOfDebug = %+v, want one base-red key, no main-red key and no main commit", got)
	}
	if zero := VerdictOfDebug(nil); !reflect.DeepEqual(zero, Verdict{}) {
		t.Errorf("VerdictOfDebug(nil) = %+v, want the zero verdict", zero)
	}
}

func TestParseNames_IsTheInverseOfNames(t *testing.T) {
	t.Parallel()
	keys := []Key{{Package: "m/a", Test: "TestOne"}, {Package: "m/b", Test: "TestTwo"}}
	if got := ParseNames(Names(keys)); !slices.Equal(got, keys) {
		t.Errorf("ParseNames(Names) = %v, want %v", got, keys)
	}
	if got := ParseNames(""); got != nil {
		t.Errorf("ParseNames of no name = %v, want nil", got)
	}
	if got := ParseNames("not a test name"); got != nil {
		t.Errorf("ParseNames of a text with no test = %v, want nil", got)
	}
}
