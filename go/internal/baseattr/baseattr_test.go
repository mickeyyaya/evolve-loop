package baseattr

import "testing"

func TestReds_AttributableNeedsNamedKeysOnly(t *testing.T) {
	t.Parallel()
	k := []Key{{"p", "TestA"}}
	for _, tc := range []struct {
		name string
		r    Reds
		want bool
	}{
		{"named keys", Reds{Keys: k}, true},
		{"unnamed", Reds{Keys: k, Unnamed: true}, false},
		{"no keys", Reds{}, false},
	} {
		if got := tc.r.Attributable(); got != tc.want {
			t.Errorf("%s: Attributable = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestVerdict_BlamedAndMainGreen(t *testing.T) {
	t.Parallel()
	k := []Key{{"p", "TestA"}}
	for _, tc := range []struct {
		name         string
		v            Verdict
		blamed, mgrn bool
	}{
		{"empty", Verdict{}, false, false},
		{"lane only", Verdict{LaneOnly: k}, true, false},
		{"base red, main green", Verdict{BaseRed: k, Main: "abc"}, false, true},
		{"base red, main red", Verdict{BaseRed: k, MainRed: k, Main: "abc"}, false, false},
		{"base red, main unknown", Verdict{BaseRed: k}, false, false},
		{"mixed", Verdict{LaneOnly: k, BaseRed: k, Main: "abc"}, true, true},
		{"unjudged, no key", Verdict{Unjudged: true}, true, false},
	} {
		if got := tc.v.Blamed(); got != tc.blamed {
			t.Errorf("%s: Blamed = %v, want %v", tc.name, got, tc.blamed)
		}
		if got := tc.v.MainGreen(); got != tc.mgrn {
			t.Errorf("%s: MainGreen = %v, want %v", tc.name, got, tc.mgrn)
		}
	}
}
