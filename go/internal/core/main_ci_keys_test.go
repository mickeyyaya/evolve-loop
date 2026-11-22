package core

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/mainstop"
)

var (
	ciJobKey    = baseattr.Key{Package: CIPackage, Test: "build+test=failure"}
	laneTestKey = baseattr.Key{Package: "example.com/m/a", Test: "TestOne"}
)

func TestStopCIKeys_SplitAStopIntoLaneAndCIKeys(t *testing.T) {
	t.Parallel()
	mixed := mainstop.Stop{Keys: []baseattr.Key{laneTestKey, ciJobKey}}
	if !IsCIKey(ciJobKey) || IsCIKey(laneTestKey) {
		t.Errorf("IsCIKey: ci = %v, lane = %v, want true and false", IsCIKey(ciJobKey), IsCIKey(laneTestKey))
	}
	if !slices.Equal(StopLaneKeys(mixed), []baseattr.Key{laneTestKey}) || !slices.Equal(StopCIKeys(mixed), []baseattr.Key{ciJobKey}) {
		t.Errorf("lane = %v ci = %v, want one key of each kind", StopLaneKeys(mixed), StopCIKeys(mixed))
	}
	ciOnly := mainstop.Stop{Keys: []baseattr.Key{ciJobKey}}
	if StopCIOnly(mixed) || !StopCIOnly(ciOnly) || StopCIOnly(mainstop.Stop{}) {
		t.Error("StopCIOnly must hold for a held stop of CI keys alone, and for no other stop")
	}
}

func TestCIKeysStayRed_NeverRunsACIKeyAndNeverTurnsItGreen(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		keys      []baseattr.Key
		laneRed   []baseattr.Key
		wantRuns  int
		wantRed   []baseattr.Key
		wantError bool
	}{
		"a CI key alone":           {keys: []baseattr.Key{ciJobKey}, wantRuns: 0, wantRed: []baseattr.Key{ciJobKey}},
		"a lane key green":         {keys: []baseattr.Key{laneTestKey, ciJobKey}, wantRuns: 1, wantRed: []baseattr.Key{ciJobKey}},
		"a lane key red":           {keys: []baseattr.Key{laneTestKey, ciJobKey}, laneRed: []baseattr.Key{laneTestKey}, wantRuns: 1, wantRed: []baseattr.Key{laneTestKey, ciJobKey}},
		"a failed run keeps error": {keys: []baseattr.Key{laneTestKey}, wantRuns: 1, wantError: true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runs := 0
			var fn KeyRunFn = func(_ context.Context, _ string, reds baseattr.Reds) ([]baseattr.Key, error) {
				runs++
				if slices.ContainsFunc(reds.Keys, IsCIKey) {
					t.Errorf("the run got a CI key: %v", reds.Keys)
				}
				if c.wantError {
					return nil, errors.New("go test did not start")
				}
				return c.laneRed, nil
			}
			run := CIKeysStayRed(fn)

			red, err := run(context.Background(), "tip", baseattr.Reds{Keys: c.keys})

			if runs != c.wantRuns || (err != nil) != c.wantError {
				t.Errorf("runs = %d err = %v, want %d runs and error %v", runs, err, c.wantRuns, c.wantError)
			}
			if !c.wantError && !slices.Equal(red, c.wantRed) {
				t.Errorf("red = %v, want %v", red, c.wantRed)
			}
		})
	}
}
