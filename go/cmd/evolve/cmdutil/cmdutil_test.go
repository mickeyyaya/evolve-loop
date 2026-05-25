package cmdutil

import "testing"

func TestHasHelp(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want bool
	}{
		{"nil_args", nil, false},
		{"empty_args", []string{}, false},
		{"positionals_only", []string{"foo", "bar"}, false},
		{"unrelated_flag", []string{"--verbose"}, false},
		{"short_help", []string{"-h"}, true},
		{"long_help", []string{"--help"}, true},
		{"short_help_among_others", []string{"foo", "-h", "bar"}, true},
		{"long_help_among_others", []string{"--verbose", "--help"}, true},
		// "-help" (single dash) is NOT recognized — matches the existing
		// convention in every cmd_*.go file. Documented invariant.
		{"single_dash_help_not_matched", []string{"-help"}, false},
		// "--h" likewise.
		{"double_dash_h_not_matched", []string{"--h"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := HasHelp(tc.in); got != tc.want {
				t.Errorf("HasHelp(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
