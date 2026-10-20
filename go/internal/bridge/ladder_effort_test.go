package bridge

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func ladderLaunchLine(keys []string, binary string) string {
	for _, k := range keys {
		if strings.Contains(k, " "+binary+" ") || strings.HasPrefix(k, binary+" ") {
			return k
		}
	}
	return ""
}

func TestLadderEffort_AClaudeLaunchAtBalancedSendsTheLadderEffort(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	prof := writeProfile(t, ws, "ladder-effort", "")
	if def := LaunchEffort("claude-tmux", "balanced", policy.EffortTable{}, "ladder-effort"); def == "high" {
		t.Fatalf("the balanced default is already high, so the test proves nothing")
	}
	artifact := filepath.Join(ws, "artifact.md")
	fake := &FakeTmuxController{CaptureFrames: slices.Repeat([]string{"working ❯"}, 20)}
	eng := NewEngine(Deps{Tmux: &artifactOnPasteTmux{FakeTmuxController: fake, artifact: artifact}, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil)})

	if _, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: prof, Model: "balanced", Effort: "high", Prompt: "work",
		Workspace: ws, Worktree: ws, ProjectRoot: ws, ArtifactPath: artifact, Agent: "builder",
	}); err != nil {
		t.Fatal(err)
	}
	if line := ladderLaunchLine(fake.SentKeys, "claude"); !strings.Contains(line, "--effort high") {
		t.Fatalf("launch line %q, want --effort high from the ladder (keys %q)", line, fake.SentKeys)
	}
}

func TestLadderEffort_TheLaunchArgsCarryOnlyASetLadderEffort(t *testing.T) {
	t.Parallel()
	with := launchArgs(core.BridgeRequest{CLI: "claude-tmux", Effort: "high"}, "p", "o", "e", Deps{})
	without := launchArgs(core.BridgeRequest{CLI: "claude-tmux"}, "p", "o", "e", Deps{})

	if !slices.Contains(with, "--effort=high") {
		t.Fatalf("args %q, want --effort=high", with)
	}
	if slices.ContainsFunc(without, func(a string) bool { return strings.HasPrefix(a, "--effort") }) {
		t.Fatalf("args %q carry an effort without a ladder effort", without)
	}
	raw, err := parseLaunchArgs(with, nil)
	if err != nil || raw.effort != "high" {
		t.Fatalf("parseLaunchArgs = (%+v, %v), want effort high", raw, err)
	}
}

func TestLadderEffort_AnUnknownLadderEffortIsABadFlag(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	prof := writeProfile(t, ws, "ladder-effort-bad", "")
	var stderr bytes.Buffer
	eng := NewEngine(Deps{LookupEnv: mapLookup(nil)})
	args := launchArgs(core.BridgeRequest{CLI: "claude-tmux", Profile: prof, Model: "balanced", Effort: "turbo", Workspace: ws, ArtifactPath: filepath.Join(ws, "a.md")},
		writePromptFile(t, ws), filepath.Join(ws, "o.log"), filepath.Join(ws, "e.log"), Deps{})

	if code := eng.LaunchArgs(context.Background(), args, nil, &bytes.Buffer{}, &stderr); code != ExitBadFlags {
		t.Fatalf("exit %d, want %d for an unknown effort (stderr %s)", code, ExitBadFlags, stderr.String())
	}
	if !strings.Contains(stderr.String(), "turbo") {
		t.Fatalf("stderr %q does not name the bad effort", stderr.String())
	}
}

func TestLadderEffort_IntentUsesTheLadderEffortOverTheTierDefault(t *testing.T) {
	t.Parallel()
	ladder := launchIntentFor(&Config{CLI: "claude-tmux", Model: "balanced", Effort: "high"}, Profile{}, policy.EffortTable{})
	plain := launchIntentFor(&Config{CLI: "claude-tmux", Model: "balanced"}, Profile{}, policy.EffortTable{})

	if ladder.Effort != "high" || ladder.LadderEffort != "high" {
		t.Fatalf("ladder intent effort %q ladder %q, want high and high", ladder.Effort, ladder.LadderEffort)
	}
	if plain.Effort != LaunchEffort("claude-tmux", "balanced", policy.EffortTable{}) || plain.LadderEffort != "" {
		t.Fatalf("plain intent %+v, want the tier default and no ladder effort", plain)
	}
}

func TestLadderEffort_AgyCapsALadderEffortAboveFlashHighAndSaysSo(t *testing.T) {
	t.Parallel()
	cfg := &Config{CLI: "agy-tmux", Model: "balanced", Realization: RealizeFor("agy-tmux", LaunchIntent{ModelTier: "balanced", Effort: "xhigh", LadderEffort: "xhigh"})}
	var stderr bytes.Buffer

	reportLaunchModel(Deps{Stderr: &stderr}, cfg, "[agy-tmux]", "s", "w")

	if cfg.Realization.EffortCapped != "xhigh" || cfg.Realization.EffortVariant != "(High)" {
		t.Fatalf("realization %+v, want xhigh capped to (High)", cfg.Realization)
	}
	if !slices.Contains(cfg.Realization.LaunchFlags, "Gemini 3.8 Flash (High)") {
		t.Fatalf("flags %q, want the Flash High model unchanged", cfg.Realization.LaunchFlags)
	}
	if line := stderr.String(); !strings.Contains(line, "effort=xhigh") || !strings.Contains(line, "(High)") {
		t.Fatalf("report %q, want the requested xhigh and the granted (High)", line)
	}
}

func TestLadderEffort_AgyGrantsALadderEffortTheModelHas(t *testing.T) {
	t.Parallel()
	for _, effort := range []string{"low", "high"} {
		r := RealizeFor("agy-tmux", LaunchIntent{ModelTier: "balanced", Effort: effort, LadderEffort: effort})
		if r.EffortCapped != "" {
			t.Errorf("ladder effort %s at Flash High was capped: %+v", effort, r)
		}
	}
}

func TestLadderEffort_WithoutALadderEffortAgyReportsNoCap(t *testing.T) {
	t.Parallel()
	for _, tier := range policy.TierNames() {
		plain := RealizeFor("agy-tmux", LaunchIntent{ModelTier: tier, Effort: "max"})
		if plain.EffortCapped != "" || plain.EffortVariant != "" {
			t.Errorf("tier %s without a ladder effort reports a cap: %+v", tier, plain)
		}
	}
}

func TestLadderEffort_AgyClaudeRaisesTheModelVariant(t *testing.T) {
	t.Parallel()
	r := RealizeFor("agy-claude-tmux", LaunchIntent{ModelTier: "balanced", Effort: "high", LadderEffort: "high"})

	if !slices.Contains(r.LaunchFlags, "Claude Sonnet 5.5 (High)") || r.EffortCapped != "" {
		t.Fatalf("realization %+v, want Sonnet at the High variant, uncapped", r)
	}
}

func writePromptFile(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "prompt.txt")
	if err := os.WriteFile(path, []byte("work"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLadderEffort_AgyWithNoModelVariantReportsNoCap(t *testing.T) {
	t.Parallel()
	for _, tier := range []string{"", "gemini-custom"} {
		r := RealizeFor("agy-tmux", LaunchIntent{ModelTier: tier, Effort: "max", LadderEffort: "max"})
		if r.EffortCapped != "" {
			t.Errorf("tier %q has no model variant to cap to, yet reports %+v", tier, r)
		}
	}
}
