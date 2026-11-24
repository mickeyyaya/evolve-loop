package heldaudit

import (
	"context"
	"testing"
)

func TestAdmitter_RefusesTheBuilderFamilyAnUnknownFamilyAndEveryTierBelowDeep(t *testing.T) {
	t.Parallel()
	admit := Admitter([]string{"claude"}, modelFamily)

	for _, tc := range []struct {
		cli, tier string
		want      bool
	}{
		{"agy", "deep", true},
		{"claude", "deep", false},
		{"claude-tmux", "deep", false},
		{"agy", "standard", false},
		{"unknown-cli", "deep", false},
	} {
		if got := admit(tc.cli, tc.tier); got != tc.want {
			t.Errorf("admit(%q, %q) = %v, want %v", tc.cli, tc.tier, got, tc.want)
		}
	}
}

func TestSeatRunner_IsTheSeamThatCarriesTheRequestAndReturnsItsOutcome(t *testing.T) {
	t.Parallel()
	var seat SeatRunner = func(_ context.Context, req SeatRequest) (SeatOutcome, error) {
		return SeatOutcome{CLI: req.Route, Tier: "deep"}, nil
	}

	got, err := seat(context.Background(), SeatRequest{Route: "agy"})

	if err != nil || got.CLI != "agy" || got.Tier != "deep" {
		t.Errorf("seat = %+v, %v; want the request echoed in the outcome", got, err)
	}
}
