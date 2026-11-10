package bridge

import (
	"reflect"
	"testing"
)

func TestDedupeLaunchFlags_TwoMultiValueFlagsKeepSharedValues(t *testing.T) {
	t.Parallel()
	in := []string{"--allowedTools", "Read", "Write", "--tools", "Bash", "Read", "Write", "Grep", "Glob"}
	if got := dedupeLaunchFlags(in); !reflect.DeepEqual(got, in) {
		t.Fatalf("dedupeLaunchFlags(%v) = %v, want the input unchanged: a value shared by two flags is not a duplicate", in, got)
	}
}

func TestDedupeLaunchFlags_PreservesFlagsWithOneValueAndBooleanFlags(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"repeated boolean flag", []string{"--yolo", "--yolo", "-c", "a=1"}, []string{"--yolo", "-c", "a=1"}},
		{"repeated pair", []string{"-c", "a=1", "-c", "a=1", "-c", "b=2"}, []string{"-c", "a=1", "-c", "b=2"}},
		{"distinct pairs of one flag", []string{"-c", "a=1", "-c", "b=2"}, []string{"-c", "a=1", "-c", "b=2"}},
		{"single token", []string{"--yolo"}, []string{"--yolo"}},
	}
	for _, tc := range cases {
		if got := dedupeLaunchFlags(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: dedupeLaunchFlags(%v) = %v, want %v", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestDedupeLaunchFlags_RepeatedMultiValueGroupAfterOtherFlagCollapses(t *testing.T) {
	t.Parallel()
	in := []string{"--tools", "Bash", "Read", "--yolo", "--tools", "Bash", "Read"}
	want := []string{"--tools", "Bash", "Read", "--yolo"}
	if got := dedupeLaunchFlags(in); !reflect.DeepEqual(got, want) {
		t.Fatalf("dedupeLaunchFlags(%v) = %v, want %v", in, got, want)
	}
}
