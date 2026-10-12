package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func boundaryGoalFile(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "goal.md")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParseBoundaryArgs_Refusals(t *testing.T) {
	t.Parallel()
	goal := boundaryGoalFile(t, "ship the wave\n")
	blank := boundaryGoalFile(t, " \n\t\n")
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"a stray operand", []string{"--goal-text-file", goal, "extra"}, `unexpected argument "extra"`},
		{"zero max cycles", []string{"--goal-text-file", goal, "--max-cycles", "0"}, `--max-cycles "0" must be a positive number`},
		{"negative max cycles", []string{"--goal-text-file", goal, "--max-cycles", "-1"}, `--max-cycles "-1" must be a positive number`},
		{"non-numeric max cycles", []string{"--goal-text-file", goal, "--max-cycles", "三"}, `--max-cycles "三" must be a positive number`},
		{"a whitespace-only goal", []string{"--goal-text-file", blank}, "--goal-text-file " + blank + " is empty"},
		{"an absent goal file", []string{"--goal-text-file", filepath.Join(t.TempDir(), "none.md")}, "--goal-text-file: open "},
		{"a repeated PR", []string{"--goal-text-file", goal, "--merge", "12,12"}, `"12" is not a distinct positive PR number`},
		{"no goal file", nil, "--goal-text-file is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseBoundaryArgs(tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("parseBoundaryArgs(%q) err = %v, want it to contain %q", tc.args, err, tc.want)
			}
		})
	}
}

func TestParseBoundaryArgs_AcceptsTheSmallestValidMaxCyclesAndTrimsTheGoal(t *testing.T) {
	t.Parallel()
	goal := boundaryGoalFile(t, "\n  ship the wave  \n")

	a, err := parseBoundaryArgs([]string{"--goal-text-file", goal, "--max-cycles", "1", "--merge", "7,9", "--dry-run"})

	if err != nil {
		t.Fatal(err)
	}
	if a.maxCycles != "1" || a.goalText != "ship the wave" || !a.dryRun || strings.Join(a.prs, ",") != "7,9" {
		t.Errorf("args = %+v, want maxCycles=1 goal trimmed dryRun prs=[7 9]", a)
	}
}

func TestRunBoundary_WithoutRunPrintsTheUsage(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer

	code := runBoundary([]string{"go"}, nil, &out, &errb)

	if code != exitUsage || out.Len() != 0 || errb.String() != boundaryUsage+"\n" {
		t.Errorf("code=%d stdout=%q stderr=%q, want the usage", code, out.String(), errb.String())
	}
}

func TestRunBoundary_ADryRunListsTheStepsWithoutDispatching(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer

	code := runBoundary([]string{"run", "--dry-run", "--goal-text-file", boundaryGoalFile(t, "g"), "--project-root", t.TempDir()}, nil, &out, &errb)

	if code != 0 {
		t.Fatalf("code = %d, want 0 (stderr=%q)", code, errb.String())
	}
	if !strings.HasPrefix(out.String(), "boundary: dry run; would run, in order:\n  1. ") {
		t.Errorf("stdout = %q, want the numbered step list", out.String())
	}
}
