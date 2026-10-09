package filter

import (
	"errors"
	"reflect"
	"testing"
)

var catalogChannels = []string{"ci.release", "ci.required", "cycle", "errors", "loop", "signals"}

func TestResolveChannels_WildcardsFollowTheNATSRules(t *testing.T) {
	cases := []struct {
		selectors []string
		want      []string
	}{
		{[]string{"loop"}, []string{"loop"}},
		{[]string{"ci.>"}, []string{"ci.release", "ci.required"}},
		{[]string{"ci.*"}, []string{"ci.release", "ci.required"}},
		{[]string{"*"}, []string{"cycle", "errors", "loop", "signals"}},
		{[]string{">"}, catalogChannels},
		{[]string{"loop", "*", "ci.required"}, []string{"ci.required", "cycle", "errors", "loop", "signals"}},
	}
	for _, tc := range cases {
		got, err := ResolveChannels(tc.selectors, catalogChannels)
		if err != nil {
			t.Fatalf("ResolveChannels(%q): %v", tc.selectors, err)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("ResolveChannels(%q) = %q, want %q", tc.selectors, got, tc.want)
		}
	}
}

func TestResolveChannels_ASelectorWithNoChannelIsRefused(t *testing.T) {
	for _, sel := range []string{"ship", "ci", "loop.>", "*.*.*"} {
		_, err := ResolveChannels([]string{"loop", sel}, catalogChannels)
		if !errors.Is(err, ErrRefused) {
			t.Errorf("ResolveChannels(%q) error = %v, want ErrRefused", sel, err)
		}
	}
}

func TestResolveChannels_AMalformedSelectorIsAUsageError(t *testing.T) {
	for _, sel := range []string{"", "Loop", "ci..x", ">.ci", "ci.>.x", "c*", "ci.", "a b"} {
		_, err := ResolveChannels([]string{sel}, catalogChannels)
		if !errors.Is(err, ErrUsage) {
			t.Errorf("ResolveChannels(%q) error = %v, want ErrUsage", sel, err)
		}
	}
	if _, err := ResolveChannels(nil, catalogChannels); !errors.Is(err, ErrUsage) {
		t.Errorf("an empty selection error = %v, want ErrUsage", err)
	}
}

func TestValidChannelName_AcceptsDottedTokensOnly(t *testing.T) {
	cases := map[string]bool{
		"loop": true, "ci.required": true, "a_b-1.c2": true,
		"": false, "Loop": false, "ci.": false, ".ci": false, "ci..x": false,
		"ci.*": false, "ci.>": false, "*": false, "a b": false,
	}
	for name, want := range cases {
		if got := ValidChannelName(name); got != want {
			t.Errorf("ValidChannelName(%q) = %v, want %v", name, got, want)
		}
	}
}
