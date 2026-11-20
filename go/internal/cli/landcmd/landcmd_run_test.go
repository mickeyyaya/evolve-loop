package landcmd

import (
	"bytes"
	"strings"
	"testing"
)

const (
	prMergeUsage    = "usage: evolve pr merge <n>... [--update-branch] [--wait D] [--json] [--project-root P]"
	prWindowUsage   = "usage: evolve pr window <n> | --files-from FILE [--json] [--project-root P]"
	prBoundaryUsage = "usage: evolve pr boundary [--json] [--project-root P]"
	prRefreshUsage  = "usage: evolve pr refresh [--project-root P]"
	ciClassifyUsage = "usage: evolve ci classify <run-id|run:N|pr:N|sha:H> [--json] [--rerun] [--project-root P]"
	ciWatchUsage    = "usage: evolve ci watch (--sha S | --pr N | --tag T) [--workflow W]... [--cycle N] [--project-root P]"
)

func runPR(args ...string) (code int, stdout, stderr string) {
	defer current.Store(current.Load())
	var out, errb bytes.Buffer
	code = RunPR(args, strings.NewReader(""), &out, &errb, Deps{})
	return code, out.String(), errb.String()
}

func runCI(args ...string) (code int, stdout, stderr string) {
	defer current.Store(current.Load())
	var out, errb bytes.Buffer
	code = RunCI(args, strings.NewReader(""), &out, &errb, Deps{})
	return code, out.String(), errb.String()
}

func TestLandCmdRun(t *testing.T) {
	cases := []struct {
		name       string
		run        func(args ...string) (int, string, string)
		args       []string
		wantCode   int
		wantStdout []string
		wantStderr []string
	}{
		{
			name:       "pr with no verb prints every pr usage line and exits with the usage code",
			run:        runPR,
			wantCode:   10,
			wantStderr: []string{prMergeUsage, prWindowUsage, prBoundaryUsage, prRefreshUsage},
		},
		{
			name:       "pr merge with no number is a usage error naming the missing number",
			run:        runPR,
			args:       []string{"merge"},
			wantCode:   10,
			wantStderr: []string{"evolve pr merge: name at least one PR number", prMergeUsage},
		},
		{
			name:       "pr merge refuses a non-numeric operand",
			run:        runPR,
			args:       []string{"merge", "abc"},
			wantCode:   10,
			wantStderr: []string{`evolve pr merge: PR operand "abc" must be a distinct positive number`},
		},
		{
			name:       "pr merge refuses a repeated number",
			run:        runPR,
			args:       []string{"merge", "5", "5"},
			wantCode:   10,
			wantStderr: []string{`evolve pr merge: PR operand "5" must be a distinct positive number`},
		},
		{
			name:       "pr merge refuses a negative wait before it reads the project root",
			run:        runPR,
			args:       []string{"merge", "5", "--wait", "-1s"},
			wantCode:   10,
			wantStderr: []string{`evolve pr merge: --wait "-1s" must be a non-negative duration`},
		},
		{
			name:       "pr window with neither a number nor a file is a usage error",
			run:        runPR,
			args:       []string{"window"},
			wantCode:   10,
			wantStderr: []string{"evolve pr window: name a PR number or --files-from", prWindowUsage},
		},
		{
			name:       "pr boundary refuses an operand",
			run:        runPR,
			args:       []string{"boundary", "extra"},
			wantCode:   10,
			wantStderr: []string{`evolve pr boundary: unexpected operand "extra"`, prBoundaryUsage},
		},
		{
			name:       "pr refresh refuses an operand",
			run:        runPR,
			args:       []string{"refresh", "extra"},
			wantCode:   10,
			wantStderr: []string{prRefreshUsage},
		},
		{
			name:       "ci with no verb prints the ci usage and exits with the usage code",
			run:        runCI,
			wantCode:   10,
			wantStderr: []string{ciClassifyUsage, ciWatchUsage, "evolve ci darwin-set <generate|check>", "evolve ci telemetry --jobs FILE"},
		},
		{
			name:       "ci classify with no target is a usage error",
			run:        runCI,
			args:       []string{"classify"},
			wantCode:   10,
			wantStderr: []string{"evolve ci classify: want exactly one target, got 0", ciClassifyUsage},
		},
		{
			name:       "ci classify refuses a target that is no run, pr or sha",
			run:        runCI,
			args:       []string{"classify", "nonsense"},
			wantCode:   10,
			wantStderr: []string{`evolve ci classify: ciwatch: target must be <run-id>, run:<id>, pr:<n>, sha:<hex> or a 7-40 hex sha: "nonsense"`},
		},
		{
			name:       "ci classify refuses a second target",
			run:        runCI,
			args:       []string{"classify", "pr:1", "pr:2"},
			wantCode:   10,
			wantStderr: []string{"evolve ci classify: want exactly one target, got 2"},
		},
		{
			name:       "ci watch with no target names the three choices",
			run:        runCI,
			args:       []string{"watch"},
			wantCode:   10,
			wantStderr: []string{"evolve ci watch: name exactly one of --sha, --pr or --tag, got 0", ciWatchUsage},
		},
		{
			name:       "ci watch help prints the watch usage on stdout and exits 0",
			run:        runCI,
			args:       []string{"watch", "--help"},
			wantCode:   0,
			wantStdout: []string{ciWatchUsage, "--workflow W   watch workflow W instead of the defaults (repeatable)"},
		},
		{
			name:       "ci telemetry without a jobs file is a usage error",
			run:        runCI,
			args:       []string{"telemetry"},
			wantCode:   10,
			wantStderr: []string{"evolve ci telemetry: want --jobs <file> and no operand"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, stdout, stderr := c.run(c.args...)
			if code != c.wantCode {
				t.Fatalf("exit code = %d, want %d\nstdout: %s\nstderr: %s", code, c.wantCode, stdout, stderr)
			}
			for _, want := range c.wantStdout {
				if !strings.Contains(stdout, want) {
					t.Errorf("stdout lacks %q:\n%s", want, stdout)
				}
			}
			for _, want := range c.wantStderr {
				if !strings.Contains(stderr, want) {
					t.Errorf("stderr lacks %q:\n%s", want, stderr)
				}
			}
			if c.wantCode == 0 && stderr != "" {
				t.Errorf("a successful help run wrote to stderr:\n%s", stderr)
			}
			if c.wantCode != 0 && stdout != "" {
				t.Errorf("a usage error wrote to stdout:\n%s", stdout)
			}
		})
	}
}
