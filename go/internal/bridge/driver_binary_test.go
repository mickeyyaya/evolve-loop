package bridge

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
)

func useTargetOver(t *testing.T, target, base string) {
	t.Helper()
	dir := t.TempDir()
	writeManifestFile(t, dir, target, `{"cli":"`+target+`","base":"`+base+`"}`)
	useBridgeManifestDir(t, dir)
}

func TestPaneProfileFor_AgyClaudeTmuxReadsTheAgyBinarysPaneWithTheAgyDetector(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	agyMarker := "? for shortcuts"
	target := paneProfileFor(tmuxLaunch{name: "agy-claude-tmux", promptMarker: agyMarker})
	agy := paneProfileFor(tmuxLaunch{name: "agy-tmux", promptMarker: agyMarker})

	if target != agy {
		t.Errorf("agy-claude-tmux pane profile = %+v, want agy-tmux's %+v: the target overrides no pane field, one binary renders one pane", target, agy)
	}
	if target.Name != "agy" || target.BoundaryMarker != ">" || !target.BoundaryExact {
		t.Errorf("agy-claude-tmux pane profile = %+v, want agy's exact \">\" input boundary, not the %q footer", target, agyMarker)
	}
	if _, isAgy := detectorFor(tmuxLaunch{name: "agy-claude-tmux", promptMarker: agyMarker}).(*panestream.AgyDetector); !isAgy {
		t.Errorf("agy-claude-tmux liveness detector is not *panestream.AgyDetector: agy's generating signal would go unread")
	}
}

func TestDriverBinary_KeysByTheManifestBinaryAndTheStemOnlyWithoutAManifest(t *testing.T) {
	cases := map[string]string{
		"agy-claude-tmux": "agy",
		"agy-tmux":        "agy",
		"claude-p":        "claude",
		"codex":           "codex",
		"ollama-tmux":     "ollama",
		"itest-tmux":      "itest",
	}
	for driver, want := range cases {
		if got := driverBinary(driver); got != want {
			t.Errorf("driverBinary(%q) = %q, want %q", driver, got, want)
		}
	}
}

func TestAutoResponderTick_ReadsBusyFromTheProfileOfTheBinaryATargetDrives(t *testing.T) {
	useTargetOver(t, "fake-ollama-tmux", "ollama-tmux")
	pane := "Which absolute path should I write the deliverable to?\n>>> \n"
	deps := Deps{Tmux: &fakeTmux{paneSeq: []string{pane}}, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil)}.withDefaults()
	ar := newAutoResponder("fake-ollama-tmux", t.TempDir(), deps, false, 0)
	ar.prompts = escalatePrompt

	if _, rc := ar.tick(context.Background(), "s"); rc != 0 {
		t.Errorf("tick rc = %d, want 0: a target on the ollama binary is busy while ollama's idle placeholder is absent, so the escalate rule must wait", rc)
	}
}

func TestDefaultWallCorroborator_ProbesTheRecipeOfTheBinaryATargetDrives(t *testing.T) {
	useTargetOver(t, "fake-claude-tmux", "claude-tmux")
	var probed []string
	run := func(_ context.Context, bin, _ string, _ []string, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
		probed = append(probed, bin)
		return 0, nil
	}

	walled := DefaultWallCorroborator(run, io.Discard)(context.Background(), "fake-claude-tmux")

	if walled || len(probed) != 1 || probed[0] != "claude" {
		t.Errorf("corroborate(fake-claude-tmux) = walled %v after probing %v, want the claude binary's probe to answer and clear the wall", walled, probed)
	}
}

func TestAgyTmuxLaunch_AgyClaudeTmuxLaunchesWithAgyTmuxsPaneContract(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	agy := agyTmuxLaunch("agy-tmux", &Config{}, Deps{}, "s", false)
	target := agyTmuxLaunch("agy-claude-tmux", &Config{}, Deps{}, "s", false)

	if target.name != "agy-claude-tmux" || target.promptMarker != agy.promptMarker || target.inputLineMarker != agy.inputLineMarker || target.inputLineMarker != ">" {
		t.Errorf("agy-claude-tmux launch = name %q prompt %q input %q, want its own name with agy-tmux's %q prompt and %q input-line marker", target.name, target.promptMarker, target.inputLineMarker, agy.promptMarker, ">")
	}
	profile := paneProfileFor(target)
	if profile != paneProfileFor(agy) || profile.ModelLabelRegex == "" || profile.BusyLineRegex == "" || profile.TokenLineRegex == "" {
		t.Errorf("agy-claude-tmux pane profile = %+v, want agy-tmux's, with its busy, token and model-label rules", profile)
	}
}
