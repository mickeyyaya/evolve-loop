package main

import (
	"flag"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/scopedelta"
)

func TestBuildScopeDeltaReport_ClassifiesTheLanesChangeAgainstItsPinAndWave(t *testing.T) {
	t.Parallel()
	wt := gittest.Fixture(t)
	wt.Write("docs/alpha.md", "base\n")
	wt.Write("docs/beta.md", "base\n")
	wt.Git("add", "-A")
	wt.Git("commit", "-q", "-m", "base")
	base := wt.Git("rev-parse", "HEAD")
	for _, rel := range []string{"docs/alpha.md", "docs/beta.md", "docs/gamma.md"} {
		wt.Write(rel, "changed\n")
	}
	wt.Git("add", "-A")
	wt.Git("commit", "-q", "-m", "lane change")
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	writeScopeDeltaLane(t, evolveDir, "7", "g", "self", "docs/alpha.md")
	writeScopeDeltaLane(t, evolveDir, "8", "g", "sib", "docs/beta.md")
	state := `{"cycle_id":7,"goal_hash":"g","active_worktree":` + strconv.Quote(wt.Dir) + `,"worktree_base_sha":"` + base + `"}`
	if err := os.WriteFile(filepath.Join(evolveDir, "runs", "cycle-7", "cycle-state.json"), []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := buildScopeDeltaReport(evolveDir, 7, io.Discard)

	if err != nil {
		t.Fatal(err)
	}
	got := map[string]scopedelta.Class{}
	for _, r := range report.Entries {
		got[r.Path] = r.Class
	}
	want := map[string]scopedelta.Class{"docs/alpha.md": scopedelta.ClassInScope, "docs/beta.md": scopedelta.ClassCrossLane, "docs/gamma.md": scopedelta.ClassOpportunistic}
	if !maps.Equal(got, want) || report.Summary.Kept != 1 || report.Summary.Carved != 2 {
		t.Fatalf("the operator view classifies the lane's diff against its pin and its wave's siblings: entries %v, summary %+v", got, report.Summary)
	}
}

func TestBuildScopeDeltaReport_EachUnreadableInputIsAnError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, state string
		claimsFile  bool
		wantErr     string
	}{
		{"no cycle state", "", false, "read cycle 7 state"},
		{"unreadable claims", `{"cycle_id":7}`, true, "lane 7 claims"},
		{"no worktree to diff", `{"cycle_id":7}`, false, "names no active_worktree"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			evolveDir := filepath.Join(t.TempDir(), ".evolve")
			runDir := filepath.Join(evolveDir, "runs", "cycle-7")
			if err := os.MkdirAll(filepath.Join(evolveDir, "inbox", "processing"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(runDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if tc.state != "" {
				if err := os.WriteFile(filepath.Join(runDir, "cycle-state.json"), []byte(tc.state), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if tc.claimsFile {
				if err := os.WriteFile(filepath.Join(evolveDir, "inbox", "processing", "cycle-7"), []byte("not a dir"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			_, err := buildScopeDeltaReport(evolveDir, 7, io.Discard)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want one containing %q", err, tc.wantErr)
			}
		})
	}
}

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

func writeScopeDeltaLane(t *testing.T, evolveDir, cycle, goalHash, itemID, file string) {
	t.Helper()
	for dir, body := range map[string][2]string{
		filepath.Join(evolveDir, "runs", "cycle-"+cycle):                {"lane-scope.json", `{"todo_ids":["` + itemID + `"],"goal_hash":"` + goalHash + `"}`},
		filepath.Join(evolveDir, "inbox", "processing", "cycle-"+cycle): {itemID + ".json", `{"id":"` + itemID + `","files":["` + file + `"]}`},
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, body[0]), []byte(body[1]), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
