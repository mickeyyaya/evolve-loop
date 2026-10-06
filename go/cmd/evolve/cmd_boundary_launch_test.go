package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestBoundaryRun_LaunchStepPassesTheRealLoopArgumentParser(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		goal          string
		extra         []string
		wantMaxCycles int
	}{
		{"goal only", "drain the console queue", nil, 0},
		{"merge and max-cycles", "drain the console queue", []string{"--merge", "12,15", "--max-cycles", "3"}, 3},
		{"goal text that looks like flags", "--resume \"quoted\" goal\nsecond line", []string{"--max-cycles", "1"}, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			plane := t.TempDir()
			if _, rc := parseLoopArgs([]string{"--detach", "--goal-text", c.goal, "--project-root", plane}, io.Discard); rc == 0 {
				t.Fatal("fixture: the real loop parser must refuse --detach without --log, else this test proves nothing")
			}
			goalFile := filepath.Join(t.TempDir(), "goal.txt")
			if err := os.WriteFile(goalFile, []byte("\n"+c.goal+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			var launched loopConfig
			var launchArgs []string
			launchRC := -1
			parseLaunchWithRealLoopParser := func(verb string, args []string, _, stderr io.Writer) int {
				if verb != "loop" {
					return 0
				}
				launchArgs = slices.Clone(args)
				launched, launchRC = parseLoopArgs(args, stderr)
				return launchRC
			}
			var stdout, stderr bytes.Buffer
			args := append([]string{"run", "--goal-text-file", goalFile, "--project-root", plane}, c.extra...)
			rc := runBoundaryWith(parseLaunchWithRealLoopParser, args, &stdout, &stderr)
			if launchRC == -1 {
				t.Fatalf("boundary run never dispatched the loop launch (rc=%d)\n%s%s", rc, stdout.String(), stderr.String())
			}
			if launchRC != 0 || rc != 0 {
				t.Fatalf("evolve loop's own parser refuses the launch argv %q: parser rc=%d, boundary rc=%d\n%s", launchArgs, launchRC, rc, stderr.String())
			}
			if !launched.Detach {
				t.Errorf("launch argv %q does not parse as a detached launch", launchArgs)
			}
			if launched.ProjectRoot != plane {
				t.Errorf("the launch runs on %q, want the resolved plane %q", launched.ProjectRoot, plane)
			}
			if launched.GoalText != strings.TrimSpace(c.goal) {
				t.Errorf("the launch's goal text is %q, want the goal file's trimmed text %q", launched.GoalText, strings.TrimSpace(c.goal))
			}
			if c.wantMaxCycles != 0 && launched.MaxCycles != c.wantMaxCycles {
				t.Errorf("the launch's max cycles is %d, want %d passed through", launched.MaxCycles, c.wantMaxCycles)
			}
			planeEvolveDir := filepath.Join(plane, ".evolve")
			if rel, err := filepath.Rel(planeEvolveDir, launched.LogPath); err != nil || rel == "." || !filepath.IsLocal(rel) {
				t.Errorf("the detached loop logs to %q, want a file under the plane's %s, not one relative to the operator's cwd", launched.LogPath, planeEvolveDir)
			}
		})
	}
}

func TestBoundaryRun_RefusedLaunchReturnsTheLoopsExitCodeAfterTheRelease(t *testing.T) {
	t.Parallel()
	fake := &fakeBoundaryVerbs{failAt: 6, code: exitUsage}
	var stdout, stderr bytes.Buffer
	rc := runBoundaryWith(fake.dispatch, []string{"run", "--merge", "12", "--goal-text-file", fakeBoundaryGoalFile(t)}, &stdout, &stderr)
	if rc != exitUsage {
		t.Errorf("rc = %d, want the refused launch's own exit code %d\n%s%s", rc, exitUsage, stdout.String(), stderr.String())
	}
	want := []string{"loop-stop", "pr", "sync-main", "gc", "loop-stop", "loop"}
	if got := fake.verbs(); !slices.Equal(got, want) {
		t.Errorf("dispatched %v, want %v ending at the refused launch", got, want)
	}
	if !strings.Contains(stderr.String(), "released") {
		t.Errorf("a refused launch must tell the operator the brake is no longer engaged\n%s", stderr.String())
	}
}
