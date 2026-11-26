package cycleblocker

import "testing"

func TestExports_AreNamedForTheAPICoverGate(t *testing.T) {
	t.Parallel()
	var in Input = Input{CheckTags: []string{"ste-lint"}}
	var got Attribution = Attribute(in, 50)
	if got.Blocker != CheckPrefix+"ste-lint" {
		t.Errorf("blocker %q; want the check prefix %q before the tag", got.Blocker, CheckPrefix)
	}
	if got = Attribute(Input{Class: "build|gate-block"}, 50); got.Blocker != ClassPrefix+"build|gate-block" {
		t.Errorf("blocker %q; want the class prefix %q before the class", got.Blocker, ClassPrefix)
	}
	if got = Attribute(Input{}, 50); got.Blocker != None || Waiting != "waiting" {
		t.Errorf("blocker %q, waiting %q; want none and the waiting name", got.Blocker, Waiting)
	}
}
