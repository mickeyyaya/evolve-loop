package bridge

import (
	"reflect"
	"testing"
)

// claudeperm_test.go — the claude-tmux manifest must realize EVERY valid
// claude permission mode (not just bypass+plan), so a per-phase
// EVOLVE_<AGENT>_PERMISSION_MODE override never silently no-ops. Regression
// guard for the Phase 2c gap where acceptEdits/default fell through to no flag.
func TestRealizeFor_ClaudeTmux_AllPermissionModes(t *testing.T) {
	cases := map[string][]string{
		"bypass":      {"--dangerously-skip-permissions"},
		"plan":        {"--permission-mode", "plan"},
		"acceptEdits": {"--permission-mode", "acceptEdits"},
		"default":     {"--permission-mode", "default"},
		"dontAsk":     {"--permission-mode", "dontAsk"},
		"auto":        {"--permission-mode", "auto"},
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
