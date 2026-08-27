package bridge

import (
	"reflect"
	"testing"
)

// realizer_dedupe_pairs_test.go — dedupeLaunchFlags must dedupe flag-VALUE
// PAIRS as units, not individual tokens.
//
// The function's doc comment used to state a contract it did not implement:
// "a flag with distinct values (e.g. -m gpt-5.4 vs -m gpt-5.5) is correctly
// kept twice because the token values differ" and "flag-value pairs that
// legitimately repeat should NOT be deduped this way". Token-wise dedupe does
// neither: the repeated FLAG token is dropped while both value tokens survive,
// silently reassembling `-m a -m b` into `-m a b` — a different command line,
// not a deduplicated one.
//
// Latent until 2026-08-27, when codex's effort param needed two `-c` overrides
// (model_reasoning_effort and plan_mode_reasoning_effort). The realized argv
// came out as `-c model_reasoning_effort=high plan_mode_reasoning_effort=high`,
// passing the second key as a bare positional. Caught by an existing argv pin
// rather than in production, which is the argument for pinning argv exactly.
func TestDedupeLaunchFlags_KeepsDistinctValuesForRepeatedFlag(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{
			"two -c overrides with different keys both survive intact",
			[]string{"-c", "model_reasoning_effort=high", "-c", "plan_mode_reasoning_effort=high"},
			[]string{"-c", "model_reasoning_effort=high", "-c", "plan_mode_reasoning_effort=high"},
		},
		{
			"the doc comment's own example: -m with distinct values",
			[]string{"-m", "gpt-5.4", "-m", "gpt-5.5"},
			[]string{"-m", "gpt-5.4", "-m", "gpt-5.5"},
		},
		{
			"an identical pair IS still deduped (the original purpose)",
			[]string{"-c", "a=1", "-c", "a=1"},
			[]string{"-c", "a=1"},
		},
		{
			"boolean flags still dedupe by token (the cycle-124 case)",
			[]string{"--yolo", "--dangerously-skip-permissions", "--yolo"},
			[]string{"--yolo", "--dangerously-skip-permissions"},
		},
		{
			"mixed: default_args boolean + param pair, order preserved",
			[]string{"--yolo", "-m", "gpt-5.6-terra", "--yolo", "-c", "x=1"},
			[]string{"--yolo", "-m", "gpt-5.6-terra", "-c", "x=1"},
		},
		{
			// KNOWN LIMITATION, pinned so it cannot regress silently: pairing
			// keys on "next token does not start with -", not on flag arity, so
			// a value that itself looks like a flag is not recognised and the
			// pair degrades to the old token-wise behaviour. Unreachable today
			// (no manifest or tracked profile emits a `-`-leading value); this
			// row is the tripwire for the day one does.
			"dash-leading VALUE is not recognised as a value (documented limit)",
			[]string{"--min", "-1", "--max", "-1"},
			[]string{"--min", "-1", "--max"},
		},
		{
			// The empty-token guard, pinned for the same reason: an empty token
			// is manifest noise, not a value, so it stays standalone.
			"empty token does not pair; duplicates collapse",
			[]string{"", "--a", ""},
			[]string{"", "--a"},
		},
		{"empty", nil, nil},
		{"single", []string{"--yolo"}, []string{"--yolo"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dedupeLaunchFlags(tc.in)
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("dedupeLaunchFlags(%v)\n  = %v\n  want %v", tc.in, got, tc.want)
			}
		})
	}
}
