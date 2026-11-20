package integration

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"testing"
)

type ciRunStub struct {
	ID         int
	Status     string
	Conclusion string
}

func runsOfTheSHA(runs []ciRunStub) func([]string) (string, int, error) {
	return func(args []string) (string, int, error) {
		switch {
		case has(args, "run", "list"):
			limit := len(runs)
			for i, a := range args {
				if a == "--limit" && i+1 < len(args) {
					if n, err := strconv.Atoi(args[i+1]); err == nil && n < limit {
						limit = n
					}
				}
			}
			listed := make([]map[string]any, 0, limit)
			for _, r := range runs[:limit] {
				listed = append(listed, map[string]any{"databaseId": r.ID, "status": r.Status, "conclusion": r.Conclusion})
			}
			out, _ := json.Marshal(listed)
			return string(out) + "\n", 0, nil
		case has(args, "run", "view"):
			for _, r := range runs {
				if args[2] == strconv.Itoa(r.ID) {
					out, _ := json.Marshal(map[string]string{"status": r.Status, "conclusion": r.Conclusion})
					return string(out) + "\n", 0, nil
				}
			}
		}
		return "", 0, nil
	}
}

func checkMainThrough(t *testing.T, name string, runs []ciRunStub) (MainState, error) {
	t.Helper()
	ci := newGhCI(t, &ghScript{reply: runsOfTheSHA(runs)})
	var watcher MainCI = ci
	if name == "peek" {
		watcher = PeekCI{CI: ci}
	}
	return Train{MainCI: watcher}.checkMain(context.Background(), sha('a'))
}

var mainCIWatchers = []string{"watch", "peek"}

func TestCheckMain_AGreenRunWinsOverANewerCancelledRun(t *testing.T) {
	t.Parallel()
	for _, name := range mainCIWatchers {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			state, err := checkMainThrough(t, name, []ciRunStub{
				{ID: 30, Status: "completed", Conclusion: "cancelled"},
				{ID: 20, Status: "completed", Conclusion: "success"},
			})
			if err != nil || state.State != MainGreen || state.SHA != sha('a') {
				t.Errorf("checkMain = %+v, %v; want main green at %s", state, err, sha('a'))
			}
		})
	}
}

func TestCheckMain_ARedRunOfTheSHAStaysRed(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		runs []ciRunStub
	}{
		{"newest run red", []ciRunStub{{ID: 30, Status: "completed", Conclusion: "failure"}, {ID: 20, Status: "completed", Conclusion: "success"}}},
		{"older run red", []ciRunStub{{ID: 30, Status: "completed", Conclusion: "success"}, {ID: 20, Status: "completed", Conclusion: "failure"}}},
		{"red beside a cancelled run", []ciRunStub{{ID: 30, Status: "completed", Conclusion: "cancelled"}, {ID: 20, Status: "completed", Conclusion: "failure"}}},
	} {
		for _, name := range mainCIWatchers {
			t.Run(c.name+"/"+name, func(t *testing.T) {
				t.Parallel()
				state, err := checkMainThrough(t, name, c.runs)
				var refusal *MainRefusal
				if !errors.As(err, &refusal) || refusal.Reason != string(Red) || state.State != "" {
					t.Errorf("checkMain = %+v, %v; want a main refusal for red", state, err)
				}
			})
		}
	}
}

func TestCheckMain_OnlyCancelledRunsLeaveMainNotGreen(t *testing.T) {
	t.Parallel()
	for _, name := range mainCIWatchers {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			state, err := checkMainThrough(t, name, []ciRunStub{
				{ID: 30, Status: "completed", Conclusion: "cancelled"},
				{ID: 20, Status: "completed", Conclusion: "cancelled"},
			})
			var refusal *MainRefusal
			if !errors.As(err, &refusal) || refusal.Reason != "cancelled" || state.State != "" {
				t.Errorf("checkMain = %+v, %v; want a main refusal for cancelled", state, err)
			}
		})
	}
}

func TestCheckMain_NoRunOfTheSHAIsStillNoRun(t *testing.T) {
	t.Parallel()
	for _, name := range mainCIWatchers {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := checkMainThrough(t, name, nil)
			var refusal *MainRefusal
			if !errors.As(err, &refusal) || refusal.Reason != "no CI run" {
				t.Errorf("checkMain = %v; want a main refusal for no CI run", err)
			}
		})
	}
}
