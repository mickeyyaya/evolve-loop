package inboxmover

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsClaimRefusal_NamesExactlyTheThreeRefusalsThroughAnyWrap(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"delivered", ErrDelivered, true},
		{"wrapped delivered", fmt.Errorf("item a: %w", ErrDelivered), true},
		{"console routed", fmt.Errorf("item a: %w", ErrConsoleRouted), true},
		{"waiting on a dependency", fmt.Errorf("item a: %w", ErrWaitingOnDependency), true},
		{"joined with a refusal", errors.Join(errors.New("disk"), ErrDelivered), true},
		{"not found", ErrNotFound, false},
		{"move failed", fmt.Errorf("x: %w", ErrMvFailed), false},
		{"an unclassified error", errors.New("disk on fire"), false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsClaimRefusal(tc.err); got != tc.want {
				t.Errorf("IsClaimRefusal(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
