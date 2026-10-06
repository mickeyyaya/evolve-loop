package runner

import "testing"

func TestJoinAttempts(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{"codex-tmux=80"}, "codex-tmux=80"},
		{[]string{"codex-tmux=80", "claude-tmux=0"}, "codex-tmux=80 -> claude-tmux=0"},
	}
	for _, c := range cases {
		if got := joinAttempts(c.in); got != c.want {
			t.Errorf("joinAttempts(%v)=%q want %q", c.in, got, c.want)
		}
	}
}
