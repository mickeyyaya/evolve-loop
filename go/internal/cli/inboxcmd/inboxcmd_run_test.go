package inboxcmd

import (
	"bytes"
	"strings"
	"testing"
)

var inboxVerbNames = []string{
	"batches", "lint", "list", "show", "rank", "add", "edit", "verify", "withdraw",
	"claim", "claims", "release", "split", "route-console", "route-lane", "consume",
	"curate", "quarantine", "ack-fingerprint", "trailer-check",
}

func runInbox(args ...string) (code int, stdout, stderr string) {
	var out, errb bytes.Buffer
	code = Run(args, strings.NewReader(""), &out, &errb, Deps{})
	return code, out.String(), errb.String()
}

func TestInboxCmdRun(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout []string
		wantStderr []string
	}{
		{
			name:       "list help prints the verb usage and exits with the usage code",
			args:       []string{"list", "--help"},
			wantCode:   10,
			wantStderr: []string{"usage: evolve inbox list [--status ready|console|waiting] [--kind K] [--route R] [--json]"},
		},
		{
			name:       "group help lists every verb on stdout and exits 0",
			args:       []string{"--help"},
			wantCode:   0,
			wantStdout: append([]string{"usage: evolve inbox <verb> ..."}, verbUsageLines()...),
		},
		{
			name:       "short help flag is the same as the long one",
			args:       []string{"-h"},
			wantCode:   0,
			wantStdout: []string{"usage: evolve inbox <verb> ..."},
		},
		{
			name:       "no verb prints the group usage on stderr and exits with the usage code",
			args:       nil,
			wantCode:   10,
			wantStderr: []string{"usage: evolve inbox <verb> ..."},
		},
		{
			name:       "an unknown verb is a usage error",
			args:       []string{"nosuchverb"},
			wantCode:   10,
			wantStderr: []string{"usage: evolve inbox <verb> ...", "evolve inbox batches"},
		},
		{
			name:       "a status outside the menu names is a usage error",
			args:       []string{"list", "--status", "bogus"},
			wantCode:   10,
			wantStderr: []string{`inbox list: --status "bogus" is not one of ready, console, waiting`},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, stdout, stderr := runInbox(c.args...)
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

func verbUsageLines() []string {
	lines := make([]string, len(inboxVerbNames))
	for i, name := range inboxVerbNames {
		lines[i] = "evolve inbox " + name
	}
	return lines
}
