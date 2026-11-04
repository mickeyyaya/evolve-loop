package bridge

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

const footerPrefixPad = "? for shortcuts                                        "

func pickedModelToken(t *testing.T, cli, tier string) string {
	t.Helper()
	token := RealizeFor(cli, LaunchIntent{ModelTier: tier, Permission: "bypass"}).modelDispatchEffect.model
	if token == "" {
		t.Fatalf("%s realizes no model for tier %s", cli, tier)
	}
	return token
}

func modelNameOfToken(token string) string {
	name, _, _ := strings.Cut(token, " (")
	return name
}

func footerShowing(name, variant string) string {
	return footerPrefixPad + name + " · " + variant
}

func mismatchFrames(t *testing.T, cli, footer string) []string {
	t.Helper()
	check, err := launchModelCheckFor(cli)
	if err != nil || !check.enabled() {
		t.Fatalf("%s declares no launch model check (%+v, %v)", cli, check, err)
	}
	return append(slices.Repeat([]string{footer}, check.waitS+2), "cleanup scrollback")
}

func requireOneModelMismatch(t *testing.T, l modelCheckLaunch, want map[string]string) {
	t.Helper()
	if l.code != ExitModelMismatch || l.err != nil {
		t.Fatalf("Launch = (%d, %v), want (%d, nil): the served model is not the catalog pick\n%s", l.code, l.err, ExitModelMismatch, l.stderr)
	}
	if l.tmux.PasteCount != 0 {
		t.Fatalf("the prompt was delivered %d time(s) to the wrong model", l.tmux.PasteCount)
	}
	if len(l.tmux.KilledSessions) != 1 {
		t.Fatalf("killed sessions %v, want the wrong-model session killed", l.tmux.KilledSessions)
	}
	got := l.mismatches()
	if len(got) != 1 {
		t.Fatalf("mismatch signals %d, want 1 (events %+v)", len(got), l.events)
	}
	for key, value := range want {
		if got[0].Fields[key] != value {
			t.Errorf("field %s = %q, want %q (fields %v)", key, got[0].Fields[key], value, got[0].Fields)
		}
	}
}

func requireDelivered(t *testing.T, l modelCheckLaunch) {
	t.Helper()
	if l.code == ExitModelMismatch || l.tmux.PasteCount != 1 {
		t.Fatalf("Launch = (%d, %v), paste count %d: the dispatched model booted, so the prompt goes out once\n%s", l.code, l.err, l.tmux.PasteCount, l.stderr)
	}
	if got := l.mismatches(); len(got) != 0 {
		t.Fatalf("a verified launch emitted %+v", got)
	}
}

func TestLaunchModelVerification_AgyClaudeBootingAnotherClaudeModelFailsOverWithTheMismatchSignal(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	pickedName := modelNameOfToken(pickedModelToken(t, "agy-claude-tmux", "deep"))
	const servedName = "Claude Sonnet 5.5"
	if pickedName == servedName {
		t.Fatalf("the deep pick is %s: this case needs a different Claude model", servedName)
	}
	footer := footerShowing(servedName, "medium")

	l := launchTargetOver(t, "agy-claude-tmux", "deep", mismatchFrames(t, "agy-claude-tmux", footer))

	requireOneModelMismatch(t, l, map[string]string{
		"expected": "claude", "observed": "claude", "label": servedName + " · medium",
		"expected_model": pickedName, "served_model": servedName, "cli": "agy-claude-tmux", "phase": "auditor",
	})
}

func TestLaunchModelVerification_AgyClaudeBootingAnOlderVersionOfThePickedModelIsAMismatch(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	pickedName := modelNameOfToken(pickedModelToken(t, "agy-claude-tmux", "deep"))
	const servedName = "Claude Opus 4.6"
	if pickedName == servedName {
		t.Fatalf("the deep pick is %s: this case needs an older version", servedName)
	}

	l := launchTargetOver(t, "agy-claude-tmux", "deep", mismatchFrames(t, "agy-claude-tmux", footerShowing(servedName, "medium")))

	requireOneModelMismatch(t, l, map[string]string{"expected_model": pickedName, "served_model": servedName})
}

func TestLaunchModelVerification_ThePickedModelAtAnotherEffortStillVerifies(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	pickedName := modelNameOfToken(pickedModelToken(t, "agy-claude-tmux", "deep"))

	l := launchTargetOver(t, "agy-claude-tmux", "deep", slices.Repeat([]string{footerShowing(pickedName, "high")}, 60))

	requireDelivered(t, l)
}

func TestLaunchModelCheckFor_AgyTmuxDeclaresTheGeminiRule(t *testing.T) {
	t.Parallel()
	check, err := launchModelCheckFor("agy-tmux")
	if err != nil {
		t.Fatal(err)
	}
	if !check.enabled() || check.family != "gemini" || check.waitS <= 0 || check.labelRegex == "" {
		t.Fatalf("agy-tmux check %+v: want the gemini family, a positive wait and the footer label regex", check)
	}
	if verified, err := VerifiesLaunchModel("agy-tmux"); err != nil || !verified {
		t.Fatalf("VerifiesLaunchModel(agy-tmux) = (%v, %v), want the driver to run the rule", verified, err)
	}
}

func TestLaunchModelVerification_AgyTmuxBootingClaudeFailsOverWithTheMismatchSignal(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	pickedName := modelNameOfToken(pickedModelToken(t, "agy-tmux", "deep"))
	footer := footerShowing("Claude Opus 5.5", "high")

	l := launchTargetOver(t, "agy-tmux", "deep", mismatchFrames(t, "agy-tmux", footer))

	requireOneModelMismatch(t, l, map[string]string{
		"expected": "gemini", "observed": "claude", "label": "Claude Opus 5.5 · high",
		"expected_model": pickedName, "cli": "agy-tmux", "phase": "auditor",
	})
}

func TestLaunchModelVerification_AgyTmuxBootingAnotherGeminiModelIsAMismatch(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	pickedName := modelNameOfToken(pickedModelToken(t, "agy-tmux", "deep"))
	const servedName = "Gemini 3.8 Flash"
	if pickedName == servedName {
		t.Fatalf("the deep pick is %s: this case needs a different Gemini model", servedName)
	}

	l := launchTargetOver(t, "agy-tmux", "deep", mismatchFrames(t, "agy-tmux", footerShowing(servedName, "medium")))

	requireOneModelMismatch(t, l, map[string]string{"expected_model": pickedName, "served_model": servedName, "observed": "gemini"})
}

func TestLaunchModelVerification_AgyTmuxBootingItsPickedGeminiDeliversThePrompt(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	pickedName := modelNameOfToken(pickedModelToken(t, "agy-tmux", "deep"))

	l := launchTargetOver(t, "agy-tmux", "deep", slices.Repeat([]string{footerShowing(pickedName, "medium")}, 60))

	requireDelivered(t, l)
}

type servedTry struct {
	events []signalcenter.Event
}

func (s servedTry) mismatches() []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range s.events {
		if e.Code == CodeDispatchModelMismatch {
			out = append(out, e)
		}
	}
	return out
}

func launchClaudeTmuxServedBy(t *testing.T, model string, servedModel string) servedTry {
	t.Helper()
	ws := t.TempDir()
	prof := writeProfile(t, ws, "served-model", "")
	artifact := filepath.Join(ws, "artifact.md")
	tmux := &artifactOnPasteTmux{FakeTmuxController: &FakeTmuxController{CaptureFrames: slices.Repeat([]string{"working ❯"}, 20)}, artifact: artifact}
	center := signalcenter.New()
	eng := NewEngine(Deps{
		Tmux: tmux, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil), Signals: center,
		TokenResolver: func(tokenusage.Window) (tokenusage.Result, error) {
			return tokenusage.Result{Source: tokenusage.SourceTranscript, ServedModel: servedModel}, nil
		},
	})
	if _, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: prof, Model: model, Prompt: "work",
		Workspace: ws, Worktree: ws, ProjectRoot: ws, ArtifactPath: artifact, Agent: "builder", Attempt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	center.Flush()
	return servedTry{events: center.Recent()}
}

func TestServedModel_AClaudeTmuxTranscriptModelOfAnotherTierEmitsTheMismatchSignal(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	picked := pickedModelToken(t, "claude-tmux", "deep")

	got := launchClaudeTmuxServedBy(t, "deep", "claude-sonnet-5-5").mismatches()

	if len(got) != 1 {
		t.Fatalf("mismatch signals %d, want 1: the transcript says sonnet served a %q dispatch (%+v)", len(got), picked, got)
	}
	want := map[string]string{"expected_model": picked, "served_model": "claude-sonnet-5-5", "cli": "claude-tmux"}
	for key, value := range want {
		if got[0].Fields[key] != value {
			t.Errorf("field %s = %q, want %q (fields %v)", key, got[0].Fields[key], value, got[0].Fields)
		}
	}
	if got[0].Phase != "builder" && got[0].Fields["agent"] != "builder" {
		t.Errorf("signal names neither phase nor agent builder: %+v", got[0])
	}
}

func TestServedModel_AClaudeTmuxTranscriptModelOfThePickedTierEmitsNothing(t *testing.T) {
	injectCatalogDir(t, t.TempDir())

	got := launchClaudeTmuxServedBy(t, "deep", "claude-opus-5-5").mismatches()

	if len(got) != 0 {
		t.Fatalf("an opus dispatch served by claude-opus-5-5 emitted %+v", got)
	}
}

func TestServedModel_AClaudeTmuxWithNoTranscriptModelEmitsNothing(t *testing.T) {
	injectCatalogDir(t, t.TempDir())

	got := launchClaudeTmuxServedBy(t, "deep", "").mismatches()

	if len(got) != 0 {
		t.Fatalf("a launch whose transcript reports no model emitted %+v: an unread model is no evidence", got)
	}
}

func TestServedModel_AClaudeTmuxFullIdPickServedByAnOlderVersionEmitsTheMismatchSignal(t *testing.T) {
	injectCatalogDir(t, t.TempDir())

	got := launchClaudeTmuxServedBy(t, "claude-opus-5-5", "claude-opus-4-6").mismatches()

	if len(got) != 1 || got[0].Fields["served_model"] != "claude-opus-4-6" || got[0].Fields["expected_model"] != "claude-opus-5-5" {
		t.Fatalf("mismatch signals %+v, want one naming expected claude-opus-5-5 and served claude-opus-4-6", got)
	}
}

func TestServedModel_AClaudeTmuxFullIdPickServedByItselfEmitsNothing(t *testing.T) {
	injectCatalogDir(t, t.TempDir())

	got := launchClaudeTmuxServedBy(t, "claude-opus-5-5", "claude-opus-5-5").mismatches()

	if len(got) != 0 {
		t.Fatalf("a full-id pick served by itself emitted %+v", got)
	}
}
