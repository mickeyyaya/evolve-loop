package agentcmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestAgentList_NamesReadOnlySpecProfilesOnly(t *testing.T) {
	root := project(t)
	var stdout, stderr bytes.Buffer

	code := New(depsWith(nil))([]string{"list", "--project-root", root}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit %d (stderr=%s)", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "adversarial-review\tevaluate\t") {
		t.Errorf("list lacks adversarial-review with its archetype:\n%s", out)
	}
	for _, refused := range []string{"test-amplification", "\ntdd\t", "\nship\t"} {
		if strings.Contains(out, refused) {
			t.Errorf("list names %q, which a standalone run refuses:\n%s", refused, out)
		}
	}
}

func TestAgentList_JSON(t *testing.T) {
	root := project(t)
	var stdout, stderr bytes.Buffer

	code := New(depsWith(nil))([]string{"list", "--json", "--project-root", root}, nil, &stdout, &stderr)

	var got []Profile
	if code != 0 || json.Unmarshal(stdout.Bytes(), &got) != nil {
		t.Fatalf("exit %d stdout %q", code, stdout.String())
	}
	found := false
	for _, p := range got {
		found = found || (p.Name == "adversarial-review" && p.Archetype == "evaluate")
		if p.Name == "test-amplification" {
			t.Errorf("JSON list names the source-write profile")
		}
	}
	if !found {
		t.Errorf("JSON list lacks adversarial-review: %+v", got)
	}
}

func TestAgentList_Errors(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		code   int
		reason string
	}{
		{"no catalog", []string{"list", "--project-root", t.TempDir()}, 1, "phase catalog"},
		{"bad flag", []string{"list", "--bogus"}, 10, "bogus"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := New(depsWith(nil))(c.args, nil, &stdout, &stderr)
			if code != c.code || !strings.Contains(stderr.String(), c.reason) {
				t.Errorf("exit %d stderr %q, want %d and %q", code, stderr.String(), c.code, c.reason)
			}
		})
	}
}

func TestAgent_UsageAndHelp(t *testing.T) {
	cases := []struct {
		args   []string
		code   int
		stdout bool
	}{
		{nil, 10, false},
		{[]string{"bogus"}, 10, false},
		{[]string{"help"}, 0, true},
		{[]string{"--help"}, 0, true},
		{[]string{"-h"}, 0, true},
	}
	for _, c := range cases {
		var stdout, stderr bytes.Buffer
		code := New(depsWith(nil))(c.args, nil, &stdout, &stderr)
		out := stderr.String()
		if c.stdout {
			out = stdout.String()
		}
		if code != c.code || !strings.Contains(out, "evolve agent run <profile>") {
			t.Errorf("%v: exit %d out %q, want %d and the usage", c.args, code, out, c.code)
		}
	}
}

func TestNew_ZeroDepsUseTheProcessEnvironment(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "2")
	var stdout, stderr bytes.Buffer

	code := New(Deps{})([]string{"run", "adversarial-review", "--project-root", project(t)}, strings.NewReader(diff), &stdout, &stderr)

	if code != 2 || !strings.Contains(stderr.String(), "inside a phase") {
		t.Errorf("exit %d stderr %q, want the process dispatch depth to refuse the run", code, stderr.String())
	}
}
