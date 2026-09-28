package guards

import (
	"context"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestSplitShellCommands_AnsiCQuotingYieldsTheWordBashRuns(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{`find $'docs' -delete`, []string{"find", "docs", "-delete"}},
		{`ls $'\x64ocs'`, []string{"ls", "docs"}},
		{`ls $'\x64'ocs`, []string{"ls", "docs"}},
		{`ls $'\144ocs'`, []string{"ls", "docs"}},
		{`ls $'\u0064ocs'`, []string{"ls", "docs"}},
		{`ls $'\U00000064ocs'`, []string{"ls", "docs"}},
		{`ls $'\u0164ocs'`, []string{"ls", "\u0164ocs"}},
		{`ls $'\x64\x6f'cs`, []string{"ls", "docs"}},
		{`ls $'\8'`, []string{"ls", `\8`}},
		{`echo $"docs"`, []string{"echo", "docs"}},
		{`echo $"a b"x`, []string{"echo", "a bx"}},
		{`ls $'\X44ocs'`, []string{"ls", `\X44ocs`}},
		{`echo $'it\'s'`, []string{"echo", "it's"}},
		{`echo $'a\\b'`, []string{"echo", `a\b`}},
		{`echo $'tab\there'`, []string{"echo", "tab\there"}},
		{`echo $'\xg'`, []string{"echo", `\xg`}},
		{`echo "$'docs'"`, []string{"echo", "$'docs'"}},
		{`echo \$'docs'`, []string{"echo", "$docs"}},
		{`echo $'unterminated`, []string{"echo", "unterminated"}},
	}
	for _, tc := range cases {
		cmds := splitShellCommands(tc.in)
		if len(cmds) != 1 || !slices.Equal(cmds[0].words, tc.want) {
			t.Errorf("splitShellCommands(%q) words = %q, want %q", tc.in, commandWords(cmds), tc.want)
		}
	}
}

func TestSplitShellCommands_AnsiCQuotedSeparatorsStayInTheWord(t *testing.T) {
	cmds := splitShellCommands(`echo $'a;b|c' ; ls`)
	if len(cmds) != 2 || !slices.Equal(cmds[0].words, []string{"echo", "a;b|c"}) {
		t.Fatalf("an ANSI-C quoted separator must not end the command: %q", commandWords(cmds))
	}
}

func TestShip_Decide_AnsiCQuotedGitIsStillGit(t *testing.T) {
	s := NewShip(false)
	for _, cmd := range []string{`$'git' commit -m x`, `$'\x67it' push origin main`, `$'\U00000067'it push origin main`} {
		in := core.GuardInput{ToolName: "Bash", ToolInput: map[string]any{"command": cmd}}
		if dec := s.Decide(context.Background(), in); dec.Allow {
			t.Errorf("%q allowed: bash runs git %s", cmd, "through ANSI-C quoting")
		}
	}
}

func commandWords(cmds []shellCommand) [][]string {
	out := make([][]string, 0, len(cmds))
	for _, c := range cmds {
		out = append(out, c.words)
	}
	return out
}

func TestCandidateCommands_AWrapperMakesEveryLaterWordAPossibleProgram(t *testing.T) {
	cases := []struct {
		in    string
		names []string
	}{
		{"rm -f x", []string{"rm"}},
		{"FOO=1 BAR=2 /usr/bin/unlink x", []string{"unlink"}},
		{"MV a b", []string{"mv"}},
		{"env -u FOO unlink x", []string{"-u", "foo", "unlink", "x"}},
		{"timeout 5 rm x", []string{"5", "rm", "x"}},
		{"FOO=1", nil},
	}
	for _, tc := range cases {
		var names []string
		for _, c := range candidateCommands(splitShellCommands(tc.in)[0].words) {
			names = append(names, programName(c[0]))
		}
		if !slices.Equal(names, tc.names) {
			t.Errorf("candidateCommands(%q) programs = %q, want %q", tc.in, names, tc.names)
		}
	}
}
