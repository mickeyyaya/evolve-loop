package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func TestScopeDeltaCycle_ResolvesFlagOrPositionalAndRejectsTheRest(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		args    []string
		want    int
		wantErr string
	}{
		{"flag", []string{"--cycle", "7"}, 7, ""},
		{"positional", []string{"7"}, 7, ""},
		{"both forms", []string{"--cycle", "7", "8"}, 0, "both"},
		{"two positionals", []string{"7", "8"}, 0, "unexpected"},
		{"zero positional", []string{"0"}, 0, "positive"},
		{"explicit zero flag", []string{"--cycle", "0"}, 0, "positive"},
		{"missing", nil, 0, "missing cycle"},
		{"non-numeric positional", []string{"x"}, 0, "not a number"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fs := flag.NewFlagSet("t", flag.ContinueOnError)
			fs.SetOutput(io.Discard)
			cycle := fs.Int("cycle", 0, "")
			if err := fs.Parse(tc.args); err != nil {
				t.Fatal(err)
			}
			got, err := scopeDeltaCycle(fs, *cycle)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want one containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("got (%d, %v), want (%d, nil)", got, err, tc.want)
			}
		})
	}
}

func TestLaneChangedPaths_NoWorktreeIsAnError(t *testing.T) {
	t.Parallel()
	if _, err := laneChangedPaths(cyclestate.CycleState{WorktreeBaseSHA: "abc"}); err == nil {
		t.Error("a state without active_worktree must not read as an empty diff")
	}
	if _, err := laneChangedPaths(cyclestate.CycleState{ActiveWorktree: t.TempDir()}); err == nil {
		t.Error("a state without worktree_base_sha must not read as an empty diff")
	}
}

func writeScopeDeltaLane(t *testing.T, evolveDir, cycle, stateBody, itemID, file string) {
	t.Helper()
	runDir := filepath.Join(evolveDir, "runs", "cycle-"+cycle)
	claimDir := filepath.Join(evolveDir, "inbox", "processing", "cycle-"+cycle)
	for dir, files := range map[string]map[string]string{
		runDir:   {"cycle-state.json": stateBody, "lane-scope.json": `{"todo_ids":["` + itemID + `"]}`},
		claimDir: {itemID + ".json": `{"id":"` + itemID + `","files":["` + file + `"]}`},
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, body := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestSiblingLaneFiles_SameWaveOnlyAndWarnsOnCorruptState(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	writeScopeDeltaLane(t, evolveDir, "1", `{"cycle_id":1,"goal_hash":"g"}`, "self", "self.go")
	writeScopeDeltaLane(t, evolveDir, "2", `{"cycle_id":2,"goal_hash":"g"}`, "sib", "sib.go")
	writeScopeDeltaLane(t, evolveDir, "3", `{"cycle_id":3,"goal_hash":"other"}`, "far", "far.go")
	writeScopeDeltaLane(t, evolveDir, "4", `{broken`, "bad", "bad.go")
	runsDir, inboxDir := filepath.Join(evolveDir, "runs"), filepath.Join(evolveDir, "inbox")

	var stderr bytes.Buffer
	got := siblingLaneFiles(runsDir, inboxDir, 1, "g", &stderr)
	if !slices.Equal(got, []string{"sib.go"}) {
		t.Errorf("siblings = %v, want only the same-wave lane's sib.go", got)
	}
	if !strings.Contains(stderr.String(), "skip sibling cycle 4") {
		t.Errorf("a corrupt sibling state must be named, stderr = %q", stderr.String())
	}
	if got := siblingLaneFiles(runsDir, inboxDir, 1, "", io.Discard); got != nil {
		t.Errorf("an empty goal_hash matched siblings %v; it names no wave", got)
	}
}
