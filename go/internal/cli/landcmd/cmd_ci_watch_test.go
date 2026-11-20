package landcmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestParseCIWatchArgs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		args      []string
		want      ciWatchArgs
		workflows []string
		wantErr   string
	}{
		{"sha", []string{"--sha", "abc1234"}, ciWatchArgs{sha: "abc1234"}, []string{"required.yml"}, ""},
		{"pr with cycle", []string{"--pr", "7", "--cycle", "42"}, ciWatchArgs{pr: "7", cycle: 42}, []string{"required.yml"}, ""},
		{"tag adds release", []string{"--tag=v1.2.3"}, ciWatchArgs{tag: "v1.2.3"}, []string{"required.yml", "release.yml"}, ""},
		{"repeated workflow replaces defaults", []string{"--tag", "v1", "--workflow", "go.yml", "--workflow=lint.yml"},
			ciWatchArgs{tag: "v1"}, []string{"go.yml", "lint.yml"}, ""},
		{"no target", nil, ciWatchArgs{}, nil, "exactly one"},
		{"two targets", []string{"--sha", "abc1234", "--tag", "v1"}, ciWatchArgs{}, nil, "exactly one"},
		{"bad sha", []string{"--sha", "xyz"}, ciWatchArgs{}, nil, "hex commit SHA"},
		{"zero pr", []string{"--pr", "0"}, ciWatchArgs{}, nil, "positive PR number"},
		{"flag-looking tag", []string{"--tag", "--cycle"}, ciWatchArgs{}, nil, "not a tag name"},
		{"negative cycle", []string{"--sha", "abc1234", "--cycle", "-1"}, ciWatchArgs{}, nil, "non-negative integer"},
		{"blank workflow", []string{"--sha", "abc1234", "--workflow", " "}, ciWatchArgs{}, nil, "workflow file name"},
		{"stray operand", []string{"--sha", "abc1234", "extra"}, ciWatchArgs{}, nil, "unexpected operand"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseCIWatchArgs(tc.args)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.sha != tc.want.sha || got.pr != tc.want.pr || got.tag != tc.want.tag || got.cycle != tc.want.cycle {
				t.Errorf("parsed %+v, want %+v", got, tc.want)
			}
			if w := got.watchedWorkflows(); !slices.Equal(w, tc.workflows) {
				t.Errorf("workflows = %v, want %v", w, tc.workflows)
			}
		})
	}
}

func TestRunCI_RoutesWatchAndPrintsItsUsage(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := runCICmd([]string{"watch", "--help"}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("ci watch --help exit = %d, stderr %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "--workflow W") {
		t.Errorf("ci watch --help printed %q", stdout.String())
	}
	stdout.Reset()
	if code := runCICmd([]string{"watch"}, nil, &stdout, &stderr); code != exitUsage || !strings.Contains(stderr.String(), "ci watch") {
		t.Errorf("bare ci watch exit = %d, stderr %q", code, stderr.String())
	}
	stderr.Reset()
	if code := runCICmd([]string{"bogus"}, nil, &stdout, &stderr); code != exitUsage || !strings.Contains(stderr.String(), "ci watch") {
		t.Errorf("unknown ci subcommand exit = %d, usage %q does not list ci watch", code, stderr.String())
	}
}

func TestRunCIWatch_InvalidTargetExitsUsage(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := runCIWatch([]string{"--sha", "not-hex"}, &stdout, &stderr); code != exitUsage {
		t.Fatalf("code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr.String(), ciWatchPrefix) {
		t.Errorf("stderr %q lacks prefix %q", stderr.String(), ciWatchPrefix)
	}
}

func TestRunCIWatch_NonexistentRootExitsIO(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := runCIWatch([]string{"--sha", "1234567", "--project-root", "/nonexistent-path-never-exists"}, &stdout, &stderr); code != exitIO {
		t.Fatalf("code = %d, want %d", code, exitIO)
	}
	if !strings.Contains(stderr.String(), ciWatchPrefix) {
		t.Errorf("stderr %q lacks prefix %q", stderr.String(), ciWatchPrefix)
	}
}

func TestClassifyRedRuns_AnUnloadablePolicyClassifiesNothing(t *testing.T) {
	bin := t.TempDir()
	log := filepath.Join(bin, "gh.log")
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte("#!/bin/sh\necho \"$*\" >> '"+log+"'\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{"ci_watch":`), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer

	classifyRedRuns(context.Background(), root, []redWorkflow{{workflow: "required.yml", runID: 5}}, &out, &errb)

	if out.Len() != 0 || !strings.HasPrefix(errb.String(), ciWatchPrefix+"classify: policy: parse ") {
		t.Errorf("stdout=%q stderr=%q, want only the classify policy error", out.String(), errb.String())
	}
	if raw, _ := os.ReadFile(log); len(raw) != 0 {
		t.Errorf("gh calls = %q, want none", string(raw))
	}
}
