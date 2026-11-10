package modelcatalog

import (
	"reflect"
	"testing"
)

func TestPinTiers_AnOfferedPinMovesItsTierAndAnUnofferedPinIsReported(t *testing.T) {
	t.Parallel()
	tiers := map[string]string{"deep": "opus", "top": "opus", "fast": "haiku"}
	pins := map[string]string{"top": "fable", "deep": "fable", "fast": "spark"}

	pinned, unmet := PinTiers(tiers, pins, []string{"fable", "opus", "haiku"})

	want := map[string]string{"deep": "fable", "top": "fable", "fast": "haiku"}
	if !reflect.DeepEqual(pinned, want) || !reflect.DeepEqual(unmet, []string{"fast=spark"}) {
		t.Errorf("PinTiers = %v, %v; want %v and the unmet fast=spark", pinned, unmet, want)
	}
	if tiers["deep"] != "opus" {
		t.Errorf("the input map moved to %q; a reused prior holds this map", tiers["deep"])
	}
}

func TestPinTiers_UnmetPinsAreInTierOrder(t *testing.T) {
	t.Parallel()
	_, unmet := PinTiers(nil, map[string]string{"top": "fable", "deep": "fable"}, nil)
	if !reflect.DeepEqual(unmet, []string{"deep=fable", "top=fable"}) {
		t.Errorf("unmet = %v, want the tiers in sorted order", unmet)
	}
}

func TestPinTiers_NoPinReturnsTheTiersOfTheCaller(t *testing.T) {
	t.Parallel()
	tiers := map[string]string{"deep": "opus"}
	pinned, unmet := PinTiers(tiers, nil, []string{"opus"})
	pinned["deep"] = "same map"
	if tiers["deep"] != "same map" || unmet != nil {
		t.Errorf("PinTiers with no pin = %v, %v; want the map of the caller and no unmet pin", pinned, unmet)
	}
}

func TestPinTiers_APinOnAnAbsentTierMapStillPins(t *testing.T) {
	t.Parallel()
	pinned, unmet := PinTiers(nil, map[string]string{"deep": "fable"}, []string{"fable"})
	if pinned["deep"] != "fable" || len(unmet) != 0 {
		t.Errorf("PinTiers over a nil map = %v, %v; want deep on fable", pinned, unmet)
	}
}

func TestPinPairs_GivesEachPinAsTierEqualsModelInTierOrder(t *testing.T) {
	t.Parallel()
	got := PinPairs(map[string]string{"top": "fable", "deep": "opus"})
	if !reflect.DeepEqual(got, []string{"deep=opus", "top=fable"}) {
		t.Errorf("PinPairs = %v, want deep=opus then top=fable", got)
	}
	if got := PinPairs(nil); got == nil || len(got) != 0 {
		t.Errorf("PinPairs(nil) = %#v, want an empty list", got)
	}
}
