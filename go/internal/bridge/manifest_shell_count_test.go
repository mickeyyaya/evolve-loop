package bridge

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
)

func TestManifestShellCountRegex_ReadsEachCLIFooter(t *testing.T) {
	t.Parallel()
	claude := paneProfileFor(tmuxLaunch{name: "claude-tmux"})
	agy := paneProfileFor(tmuxLaunch{name: "agy-tmux"})
	cases := []struct {
		name string
		pane string
		p    panestream.PaneProfile
		want int
	}{
		{"claude shells", "● working\n❯\n  ⏵⏵ bypass permissions on (shift+tab to cycle) · 2 shells · ↓ to manage\n", claude, 2},
		{"claude one shell", "● working\n❯\n  ⏵⏵ bypass permissions on (shift+tab to cycle) · 1 shell\n", claude, 1},
		{"agy task(s) mid footer", "● go test\n>\n? for shortcuts  auto · Gemini 4 Hydrogen · high · 1 task(s) · /tasks\n", agy, 1},
		{"agy tasks while busy", "● go test\n>\nesc to cancel  auto · Gemini 4 Hydrogen · high · 3 tasks · /tasks\n", agy, 3},
		{"agy singular task", "● go test\n>\n? for shortcuts  auto · high · 2 task · /tasks\n", agy, 2},
		{"agy task(s) at line end", "● go test\n>\n? for shortcuts  auto · Gemini 4 Hydrogen · high · 1 task(s)   \n", agy, 1},
		{"agy no task", "● go test\n>\n? for shortcuts  auto · Gemini 4 Hydrogen · high\n", agy, 0},
		{"agy word that starts with task", "● go test\n>\n? for shortcuts  auto · high · 1 taskmaster\n", agy, 0},
		{"agy count without a footer dot", "● go test\n>\n? for shortcuts  ran 4 tasks\n", agy, 0},
		{"an agy footer read with the claude manifest", "? for shortcuts  auto · high · 1 task(s) · /tasks\n", claude, 0},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := panestream.RunningShells(tc.pane, tc.p); got != tc.want {
				t.Errorf("RunningShells = %d, want %d for pane:\n%s", got, tc.want, tc.pane)
			}
		})
	}
}
