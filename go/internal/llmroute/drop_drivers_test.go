package llmroute

import (
	"reflect"
	"testing"
)

func TestDropDrivers_RemovesTheCandidatesWhoseDriverIsDroppedAndNamesThem(t *testing.T) {
	t.Parallel()
	p := Plan{Candidates: []string{"claude-cloud", "claude-tmux", "agy-tmux"}, Model: "sonnet"}

	got, dropped := DropDrivers(p, func(driver string) bool { return driver == "claude-cloud" })

	if !reflect.DeepEqual(got.Candidates, []string{"claude-tmux", "agy-tmux"}) || !reflect.DeepEqual(dropped, []string{"claude-cloud"}) || got.Model != "sonnet" {
		t.Fatalf("DropDrivers = %+v dropped %v, want the cloud seat removed and the rest of the plan kept", got, dropped)
	}
	if !reflect.DeepEqual(p.Candidates, []string{"claude-cloud", "claude-tmux", "agy-tmux"}) {
		t.Errorf("DropDrivers changed the input plan: %v", p.Candidates)
	}
	same, none := DropDrivers(p, func(string) bool { return false })
	if !reflect.DeepEqual(same, p) || none != nil {
		t.Errorf("DropDrivers with nothing to drop = %+v %v, want the plan unchanged and no names", same, none)
	}
}
