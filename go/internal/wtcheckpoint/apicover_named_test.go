package wtcheckpoint_test

import (
	"encoding/json"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/wtcheckpoint"
)

func TestJSONShapes_AreTheWordsTheCLIPrints(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		value any
		want  string
	}{
		{wtcheckpoint.Pruned{Ref: "refs/checkpoints/a/20261006T060000Z", Reason: wtcheckpoint.PruneReason("landed")},
			`{"ref":"refs/checkpoints/a/20261006T060000Z","reason":"landed"}`},
		{wtcheckpoint.SaveResult{Worktree: "a", Dir: "/hub/dev/a", Status: wtcheckpoint.StatusClean},
			`{"worktree":"a","dir":"/hub/dev/a","status":"clean"}`},
		{wtcheckpoint.Worktree{Dir: "/hub/dev/a", Name: "a"}, `{"dir":"/hub/dev/a","worktree":"a"}`},
	} {
		got, err := json.Marshal(tc.value)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != tc.want {
			t.Errorf("json(%T) = %s, want %s", tc.value, got, tc.want)
		}
	}
	if wtcheckpoint.PruneLanded != wtcheckpoint.PruneReason("landed") || wtcheckpoint.PruneRetention != wtcheckpoint.PruneReason("retention") {
		t.Error("the prune reasons drifted from the words the CLI prints")
	}
}
