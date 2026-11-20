package integration

import (
	"context"
	"testing"
)

func TestCheckMain_AnOlderRunThatCannotBeReadFailsTheCheck(t *testing.T) {
	t.Parallel()
	stub := runsOfTheSHA([]ciRunStub{
		{ID: 30, Status: "completed", Conclusion: "cancelled"},
		{ID: 20, Status: "completed", Conclusion: "success"},
	})
	reply := func(args []string) (string, int, error) {
		if has(args, "run", "view") && args[2] == "20" {
			return "not json\n", 0, nil
		}
		return stub(args)
	}
	ci := newGhCI(t, &ghScript{reply: reply})
	state, err := Train{MainCI: ci}.checkMain(context.Background(), sha('a'))
	if err == nil || state.State != "" {
		t.Errorf("checkMain = %+v, %v; want the unreadable older run to fail the check", state, err)
	}
}

func TestCheckMain_ANewestRunStillRunningLeavesMainPending(t *testing.T) {
	t.Parallel()
	for _, name := range mainCIWatchers {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			state, err := checkMainThrough(t, name, []ciRunStub{
				{ID: 30, Status: "in_progress"},
				{ID: 20, Status: "completed", Conclusion: "success"},
			})
			if err != nil || state.State != MainPending {
				t.Errorf("checkMain = %+v, %v; want main pending", state, err)
			}
		})
	}
}
