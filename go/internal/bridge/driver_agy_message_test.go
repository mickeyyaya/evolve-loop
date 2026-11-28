package bridge

import (
	"bytes"
	"context"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const agyBypassFlag = "--dangerously-skip-permissions"

func agyHeadlessManifest(t *testing.T) Manifest {
	t.Helper()
	injectCatalogDir(t, t.TempDir())
	m, err := LoadManifest("agy")
	if err != nil {
		t.Fatalf("LoadManifest(agy): %v", err)
	}
	return m
}

func launchAgyHeadless(t *testing.T, cfg *Config, realization Realization) (argv []string, stderr string) {
	t.Helper()
	cfg.CLI = "agy"
	cfg.PromptFile = agyPromptFile(t)
	cfg.Realization = realization
	logs := t.TempDir()
	cfg.StdoutLog, cfg.StderrLog = filepath.Join(logs, "stdout.log"), filepath.Join(logs, "stderr.log")
	runner := &fakeRunner{}
	var log bytes.Buffer
	rc, err := (agyDriver{}).Launch(context.Background(), cfg, Deps{Runner: runner.runner(), Stderr: &log})
	if err != nil || rc != 0 {
		t.Fatalf("agy headless launch rc=%d err=%v", rc, err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("agy headless launch ran %d commands, want 1", len(runner.calls))
	}
	return runner.calls[0].args, log.String()
}

func routedRealization(m Manifest, tier, effort string) Realization {
	return Realize(m, LaunchIntent{ModelTier: tier, Effort: effort})
}

func TestAgyDriver_HeadlessArgvCarriesTheRoutedModelAndEffort(t *testing.T) {
	m := agyHeadlessManifest(t)
	deepModel := m.ModelTierMap["deep"]
	if deepModel == "" {
		t.Fatal("agy manifest family table declares no deep model")
	}
	argv, _ := launchAgyHeadless(t, &Config{Model: "deep", Effort: "high"}, routedRealization(m, "deep", "high"))
	want := []string{"-p", "implement", agyBypassFlag, "--model", deepModel, "--effort", "high"}
	if !slices.Equal(argv, want) {
		t.Fatalf("agy headless argv = %q, want %q", argv, want)
	}
}

func TestAgyDriver_HeadlessArgvWithoutRouteDataHasNoModelOrEffortFlag(t *testing.T) {
	argv, _ := launchAgyHeadless(t, &Config{Model: "deep"}, Realization{})
	want := []string{"-p", "implement", agyBypassFlag}
	if !slices.Equal(argv, want) {
		t.Fatalf("agy headless argv = %q, want %q", argv, want)
	}
}

func TestAgyDriver_MessageNamesTheRoutedFlags(t *testing.T) {
	m := agyHeadlessManifest(t)
	deepModel := m.ModelTierMap["deep"]
	_, stderr := launchAgyHeadless(t, &Config{Model: "deep", Effort: "high"}, routedRealization(m, "deep", "high"))
	want := "[agy] tier 'deep' → route flags: --model " + deepModel + " --effort high\n"
	if !strings.Contains(stderr, want) {
		t.Fatalf("agy headless log lacks the pinned message %q:\n%s", want, stderr)
	}
}

func TestAgyDriver_MessageWithoutRouteDataNamesTheAgyDefault(t *testing.T) {
	_, stderr := launchAgyHeadless(t, &Config{Model: "sonnet"}, Realization{})
	want := "[agy] tier 'sonnet' → no model flag in the route; agy uses its own default model\n"
	if !strings.Contains(stderr, want) {
		t.Fatalf("agy headless log lacks the pinned message %q:\n%s", want, stderr)
	}
}

func TestAgyDriver_NoMessageSaysAgyHasNoModelFlag(t *testing.T) {
	m := agyHeadlessManifest(t)
	cases := map[string]Config{
		"canonical tier":  {Model: "deep", Effort: "high"},
		"claude alias":    {Model: "sonnet"},
		"auto":            {Model: "auto"},
		"empty":           {},
		"explicit model":  {Model: "Gemini Custom Pick"},
		"unrouted model":  {Model: "opus", Effort: "low"},
		"effort only":     {Effort: "medium"},
		"unrouted effort": {Model: "haiku", Effort: "max"},
	}
	for name, cfg := range cases {
		for label, realization := range map[string]Realization{
			"routed":   routedRealization(m, cfg.Model, cfg.Effort),
			"unrouted": {},
		} {
			c := cfg
			_, stderr := launchAgyHeadless(t, &c, realization)
			for _, stale := range []string{"no -m flag", "has no -m", "no model flag in agy", "agy ignores it anyway", "not a Claude tier alias"} {
				if strings.Contains(stderr, stale) {
					t.Errorf("%s/%s: agy headless log still says %q:\n%s", name, label, stale, stderr)
				}
			}
		}
	}
}

func TestAgyDriver_RouteChangeChangesTheArgv(t *testing.T) {
	m := agyHeadlessManifest(t)
	baseline, _ := launchAgyHeadless(t, &Config{Model: "deep", Effort: "high"}, routedRealization(m, "deep", "high"))

	tierChanged, _ := launchAgyHeadless(t, &Config{Model: "fast", Effort: "high"}, routedRealization(m, "fast", "high"))
	if slices.Equal(baseline, tierChanged) {
		t.Errorf("a tier change left the argv unchanged: %q", baseline)
	}

	effortChanged, _ := launchAgyHeadless(t, &Config{Model: "deep", Effort: "low"}, routedRealization(m, "deep", "low"))
	if slices.Equal(baseline, effortChanged) || !slices.Contains(effortChanged, "low") {
		t.Errorf("an effort change did not reach the argv: %q", effortChanged)
	}

	remapped := m
	remapped.ModelTierMap = maps.Clone(m.ModelTierMap)
	remapped.ModelTierMap["deep"] = "Remapped Route Model"
	modelChanged, _ := launchAgyHeadless(t, &Config{Model: "deep", Effort: "high"}, routedRealization(remapped, "deep", "high"))
	if i := slices.Index(modelChanged, "--model"); i < 0 || modelChanged[i+1] != "Remapped Route Model" {
		t.Errorf("a model_tier_map change did not reach the argv: %q", modelChanged)
	}
}

func TestAgyDriver_RouteIntentFromConfigReachesTheArgv(t *testing.T) {
	m := agyHeadlessManifest(t)
	cfg := &Config{CLI: "agy", Model: "balanced", Effort: "medium"}
	realization := RealizeFor(cfg.CLI, launchIntentFor(cfg, Profile{}, policy.EffortTable{}))
	argv, _ := launchAgyHeadless(t, cfg, realization)
	want := []string{"-p", "implement", agyBypassFlag, "--model", m.ModelTierMap["balanced"], "--effort", "medium"}
	if !slices.Equal(argv, want) {
		t.Fatalf("agy headless argv = %q, want %q", argv, want)
	}
}
