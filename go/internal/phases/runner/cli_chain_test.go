package runner

import "testing"

// Chain resolution + capability-probe tests moved to internal/llmroute (the
// logic now lives there). What remains in the runner are the two dispatch-log
// helpers, tested here.

func TestSameCandidates(t *testing.T) {
	cases := []struct {
		a, b []string
		want bool
	}{
		{[]string{"a", "b"}, []string{"a", "b"}, true},
		{[]string{"a", "b"}, []string{"b", "a"}, false},
		{[]string{"a"}, []string{"a", "b"}, false},
		{nil, nil, true},
		{nil, []string{}, true},
	}
	for _, c := range cases {
		if got := sameCandidates(c.a, c.b); got != c.want {
			t.Errorf("sameCandidates(%v, %v)=%v want %v", c.a, c.b, got, c.want)
		}
	}
}

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
