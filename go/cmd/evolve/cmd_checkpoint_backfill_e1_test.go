package main

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/wtcheckpoint"
)

func TestSaveLine(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		row  checkpointSaveRow
		want string
	}{
		{"a clean worktree", checkpointSaveRow{SaveResult: wtcheckpoint.SaveResult{Status: "clean", Worktree: "w"}}, "clean w"},
		{"a saved ref", checkpointSaveRow{SaveResult: wtcheckpoint.SaveResult{Status: "saved", Worktree: "w", Ref: "refs/checkpoints/w/1"}}, "saved w refs/checkpoints/w/1"},
		{"pruned and pushed", checkpointSaveRow{SaveResult: wtcheckpoint.SaveResult{Status: "saved", Worktree: "w", Ref: "r", Pruned: []string{"a", "b"}}, Pushed: true}, "saved w r (pruned 2) (pushed)"},
		{"pushed only", checkpointSaveRow{SaveResult: wtcheckpoint.SaveResult{Status: "saved", Worktree: "w", Ref: "r"}, Pushed: true}, "saved w r (pushed)"},
		{"one pruned", checkpointSaveRow{SaveResult: wtcheckpoint.SaveResult{Status: "saved", Worktree: "w", Ref: "r", Pruned: []string{"a"}}}, "saved w r (pruned 1)"},
	}
	for _, tc := range cases {
		if got := saveLine(tc.row); got != tc.want {
			t.Errorf("%s: saveLine = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestListLine(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 6, 6, 0, 1, 0, time.UTC)
	base := wtcheckpoint.Entry{Ref: "refs/checkpoints/dirty/20261006T060001Z", Time: at, Base: "0123456789abcdef", Files: 3, Added: 10, Deleted: 2}
	labelled := base
	labelled.Label = "before lunch · ü"

	if got, want := listLine(base), "refs/checkpoints/dirty/20261006T060001Z  2026-10-06T06:00:01Z  base "+shortSHA("0123456789abcdef")+"  3 file(s) +10 -2"; got != want {
		t.Errorf("listLine = %q, want %q", got, want)
	}
	if got := listLine(labelled); got != listLine(base)+"  label: before lunch · ü" {
		t.Errorf("listLine(labelled) = %q, want the label appended", got)
	}
}
