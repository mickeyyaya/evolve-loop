package bridge

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
)

func agy1217Frame(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("panestream", "testdata", "agy-1.2.17", name))
	if err != nil {
		t.Fatalf("read agy 1.2.17 fixture %s: %v", name, err)
	}
	return string(b)
}

func agyTmuxProfile() panestream.PaneProfile {
	return paneProfileFor(tmuxLaunch{name: "agy-tmux", promptMarker: "? for shortcuts"})
}

func withoutFooter(pane string) string {
	var kept []string
	for _, line := range strings.Split(pane, "\n") {
		if strings.Contains(line, "esc to cancel") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

func TestAgyTmux1217_SpinnerTickIsNotProgress(t *testing.T) {
	pairs := [][2]string{
		{"editing-a.txt", "editing-b.txt"},
		{"generating-a.txt", "generating-b.txt"},
	}
	for _, pair := range pairs {
		lc := panestream.NewLivenessCenter()
		lc.Observe("agy", agy1217Frame(t, pair[0]), agyTmuxProfile())
		lc.Observe("agy", agy1217Frame(t, pair[1]), agyTmuxProfile())
		if lc.Changed("agy") {
			t.Errorf("%s -> %s differ only in the spinner frame, but LivenessCenter reported progress", pair[0], pair[1])
		}
	}
}

func TestAgyTmux1217_RealOutputIsProgress(t *testing.T) {
	lc := panestream.NewLivenessCenter()
	lc.Observe("agy", agy1217Frame(t, "editing-after-thought.txt"), agyTmuxProfile())
	lc.Observe("agy", agy1217Frame(t, "answer.txt"), agyTmuxProfile())
	if !lc.Changed("agy") {
		t.Error("the answer frame adds transcript lines, but LivenessCenter reported no progress")
	}
}

func TestAgyTmux1217_SpinnerLineReadsBusyWithoutTheFooter(t *testing.T) {
	for _, name := range []string{"generating-a.txt", "editing-a.txt", "editing-b.txt", "thought-summary.txt"} {
		pane := withoutFooter(agy1217Frame(t, name))
		if !panestream.PaneBusy(pane, agyTmuxProfile()) {
			t.Errorf("%s: the spinner line alone must read busy", name)
		}
	}
}

func TestAgyTmux1217_IdleAndParkedPanesAreNotBusy(t *testing.T) {
	for _, name := range []string{"idle.txt", "answer.txt", "parked-paste.txt", "parked-typed.txt"} {
		if panestream.PaneBusy(agy1217Frame(t, name), agyTmuxProfile()) {
			t.Errorf("%s: no spinner and no esc footer, but the pane read busy", name)
		}
	}
}

func TestAgyTmux1217_FrozenSpinnerWalksToHungNotConverging(t *testing.T) {
	frames := map[string][]string{
		"1.0.4 exact spinner": {legacyAgyFrame(t, "thinking.txt")},
		"1.2.17 ticking spinner": {
			agy1217Frame(t, "generating-a.txt"),
			agy1217Frame(t, "generating-b.txt"),
		},
	}
	for name, seq := range frames {
		probe := detectorFor(tmuxLaunch{name: "agy-tmux"})
		var last panestream.LivenessState
		for i := 0; i < 5; i++ {
			state, _ := probe.Assess(seq[i%len(seq)], agyTmuxProfile())
			if i > 0 && state == panestream.LivenessConverging {
				t.Errorf("%s: call %d returned Converging for a spinner with no new transcript", name, i)
			}
			last = state
		}
		if last != panestream.LivenessHung {
			t.Errorf("%s: after 5 frozen frames got %v, want Hung", name, last)
		}
	}
}

func legacyAgyFrame(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("panestream", "testdata", "agy", name))
	if err != nil {
		t.Fatalf("read agy 1.0.4 fixture %s: %v", name, err)
	}
	return string(b)
}

func TestAgyTmuxManifest_BusyLineRegexNamesTheSpinnerFrame(t *testing.T) {
	m, err := LoadManifest("agy-tmux")
	if err != nil {
		t.Fatal(err)
	}
	re, err := regexp.Compile(m.BusyLineRegex)
	if err != nil || m.BusyLineRegex == "" {
		t.Fatalf("agy-tmux busy_line_regex %q must be a non-empty valid pattern: %v", m.BusyLineRegex, err)
	}
	if re.NumSubexp() < 1 {
		t.Fatalf("busy_line_regex %q must capture the spinner frame as group 1", m.BusyLineRegex)
	}
	busy := []string{"⣾  Generating...", "⢿  Working...", "⡿  Loading...", "⣽  Editing files...", "⣷  Define $a_n$ as the number"}
	for _, line := range busy {
		if !re.MatchString(line) {
			t.Errorf("busy_line_regex must match the 1.2.17 spinner line %q", line)
		}
	}
	content := []string{"  • 5", "● Edit(/tmp/work/out.txt) (ctrl+o to expand)", "▸ Thought for 14s, 1.5k tokens", "> [Pasted text #1 +122 lines]", ">"}
	for _, line := range content {
		if re.MatchString(line) {
			t.Errorf("busy_line_regex must not match the non-spinner line %q", line)
		}
	}
}
