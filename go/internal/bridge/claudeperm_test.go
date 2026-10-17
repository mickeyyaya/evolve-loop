package bridge

import (
	"reflect"
	"testing"
)

func TestRealizeFor_ClaudeTmux_AllPermissionModes(t *testing.T) {
	t.Parallel()
	cases := map[string][]string{
		"bypass":      {"--strict-mcp-config", "--dangerously-skip-permissions"},
		"plan":        {"--strict-mcp-config", "--permission-mode", "plan"},
		"acceptEdits": {"--strict-mcp-config", "--permission-mode", "acceptEdits"},
		"default":     {"--strict-mcp-config", "--permission-mode", "default"},
		"dontAsk":     {"--strict-mcp-config", "--permission-mode", "dontAsk"},
		"auto":        {"--strict-mcp-config", "--permission-mode", "auto"},
	}
	for mode, want := range cases {
		t.Run(mode, func(t *testing.T) {
			r := RealizeFor("claude-tmux", LaunchIntent{Permission: mode})
			if !reflect.DeepEqual(r.LaunchFlags, want) {
				t.Fatalf("permission %q realized to %v, want %v", mode, r.LaunchFlags, want)
			}
		})
	}
}
