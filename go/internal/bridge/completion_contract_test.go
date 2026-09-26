package bridge

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestCompletionContractName_IsTheStrategyKey(t *testing.T) {
	for mode, want := range map[string]string{
		"":                 completionArtifact,
		completionArtifact: "artifact",
		completionStdout:   "stdout",
		completionGit:      "git",
		"bogus-typo":       "bogus-typo",
	} {
		if got := completionContractName(mode); got != want {
			t.Errorf("completionContractName(%q) = %q, want %q", mode, got, want)
		}
	}
	cfg := &Config{Artifact: "/tmp/x"}
	lp := tmuxLaunch{promptMarker: "❯"}
	if _, ok := newCompletionDetector(completionContractName(""), cfg, Deps{}, lp, artifactBaseline{}).(*artifactDetector); !ok {
		t.Error("the default's name selects the artifact detector")
	}
	if _, ok := newCompletionDetector(completionStdout, cfg, Deps{}, lp, artifactBaseline{}).(*stdoutDetector); !ok {
		t.Error("completionStdout selects the stdout detector")
	}
	if _, ok := newCompletionDetector(completionGit, cfg, Deps{Stderr: io.Discard}.withDefaults(), lp, artifactBaseline{}).(*gitEvidenceDetector); !ok {
		t.Error("completionGit selects the git-evidence detector")
	}
}

// No non-test source outside completion.go assigns the artifact literal or
// compares against the stdout literal; engine.go's readResult and
// driver_tmux_repl.go's DONE line project completionContractName /
// completionStdout instead.
func TestCompletionContractVocabulary_SpelledOnce(t *testing.T) {
	for needle, want := range map[string][]string{
		`= "artifact"`:  {"internal/bridge/completion.go"},
		`== "stdout"`:   nil,
		`case "stdout"`: nil,
	} {
		got := nonTestSourcesMentioning(t, needle)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s is spelled by %v, want exactly %v", needle, got, want)
		}
	}
}

func TestRunTmuxREPL_DoneLineNamesTheContract(t *testing.T) {
	for mode, want := range map[string]string{"": "artifact", "bogus-typo": "bogus-typo"} {
		cfg := fixtureConfig(t)
		cfg.Completion = mode
		base := &FakeTmuxController{CaptureFrames: []string{"❯", "working ❯", "working ❯", "final scrollback", "cleanup scrollback"}}
		tm := &artifactOnPasteTmux{FakeTmuxController: base, artifact: cfg.Artifact}
		deps := fixtureDeps(tm)
		var stderr strings.Builder
		deps.Stderr = &stderr
		code, err := runTmuxREPL(context.Background(), cfg, deps, tmuxLaunch{
			name: "claude-tmux", session: "fixture-done-line", launchCmd: "claude", promptMarker: "❯", bootIntervalS: 1,
		})
		if err != nil || code != ExitOK {
			t.Fatalf("mode %q: runTmuxREPL = (%d,%v), want ExitOK,nil", mode, code, err)
		}
		if line := "[claude-tmux] DONE: " + want + " completion verdict = SUCCESS\n"; !strings.Contains(stderr.String(), line) {
			t.Errorf("mode %q: stderr lacks %q:\n%s", mode, line, stderr.String())
		}
	}
}
