package baseattr

import (
	"context"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

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

func TestNew_SetsTheMainRefAndTheScratchDefaults(t *testing.T) {
	t.Parallel()
	released := false
	run := func(context.Context, string, Reds) ([]Key, error) { return nil, nil }
	git := gitexec.Default(t.TempDir())
	a := New(git, run, 3*time.Second, func(context.Context) func() { return func() { released = true } })
	if a.MainRef != "main" || a.Scratch != DefaultScratch || DefaultScratch != ".evolve/ephemeral/base-red" {
		t.Errorf("defaults = %q %q, want main and .evolve/ephemeral/base-red", a.MainRef, a.Scratch)
	}
	if a.Git.Dir != git.Dir || a.Run == nil || a.Timeout != 3*time.Second {
		t.Errorf("New dropped an input: %+v", a)
	}
	a.Serialize(context.Background())()
	if !released {
		t.Error("New dropped the serialize hook")
	}
}

func TestNames_JoinsEachKeyAsPackageDotTest(t *testing.T) {
	t.Parallel()
	got := Names([]Key{{Package: "m/a", Test: "TestOne"}, {Package: "m/b", Test: "TestTwo"}})
	if want := "m/a.TestOne, m/b.TestTwo"; got != want {
		t.Errorf("Names = %q, want %q", got, want)
	}
	if got := Names(nil); got != "" {
		t.Errorf("Names(nil) = %q, want the empty string", got)
	}
}
