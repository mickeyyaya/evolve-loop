package bridge

import (
	"reflect"
	"testing"
)

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
			"dash-leading VALUE is not recognised as a value (documented limit)",
			[]string{"--min", "-1", "--max", "-1"},
			[]string{"--min", "-1", "--max"},
		},
		{
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
