package bridge

import (
	"strings"
	"testing"
)

func TestDriverFor_BareAndDriverNames(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"claude", "claude-tmux"},
		{"codex", "codex"},
		{"agy", "agy"},
		{"claude-tmux", "claude-tmux"},
		{"claude-p", "claude-p"},
		{"codex-tmux", "codex-tmux"},
		{"agy-tmux", "agy-tmux"},
		{"no-such-cli", "no-such-cli"},
		{"", ""},
	}
	for _, tc := range cases {
		got, err := DriverFor(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("DriverFor(%q)=%q, %v, want %q", tc.in, got, err, tc.want)
		}
		if tc.in != "no-such-cli" && tc.in != "" {
			if _, ok := LookupDriver(got); !ok {
				t.Errorf("DriverFor(%q)=%q is not a registered driver", tc.in, tc.want)
			}
		}
	}
}

func TestDriverFor_TheBareNameGeminiIsAnErrorThatNamesTheAgyDrivers(t *testing.T) {
	t.Parallel()
	got, err := DriverFor("gemini")
	if err == nil || got != "" {
		t.Fatalf("DriverFor(gemini) = %q, %v: the bare name must not route to another family", got, err)
	}
	for _, driver := range []string{"agy-tmux", "agy-claude-tmux"} {
		if !strings.Contains(err.Error(), driver) {
			t.Errorf("the error %q does not name %s", err, driver)
		}
	}
}
