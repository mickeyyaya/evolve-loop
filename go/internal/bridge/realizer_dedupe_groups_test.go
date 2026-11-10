package bridge

import (
	"reflect"
	"testing"
)

func TestDedupeLaunchFlags_GroupsFlagWithWholeValueRun(t *testing.T) {
	t.Parallel()
	in := []string{"--allowedTools", "Read", "Write", "Bash", "--disallowedTools", "Bash", "Write"}
	if got := dedupeLaunchFlags(in); !reflect.DeepEqual(got, in) {
		t.Fatalf("dedupeLaunchFlags(%v) = %v, want the input unchanged", in, got)
	}
}

func TestDedupeLaunchFlags_DropsRepeatedMultiValueGroup(t *testing.T) {
	t.Parallel()
	in := []string{"--allowedTools", "Read", "Write", "--allowedTools", "Read", "Write"}
	want := []string{"--allowedTools", "Read", "Write"}
	if got := dedupeLaunchFlags(in); !reflect.DeepEqual(got, want) {
		t.Fatalf("dedupeLaunchFlags(%v) = %v, want %v", in, got, want)
	}
}

func TestArgsContainEffort_IgnoresPlanModeKey(t *testing.T) {
	t.Parallel()
	if argsContainEffort([]string{"-c", "plan_mode_reasoning_effort=high"}) {
		t.Fatal("plan_mode_reasoning_effort= must not count as model_reasoning_effort=")
	}
}
