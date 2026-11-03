package bridge

import (
	"context"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func agyFastModel(t *testing.T, cli string) string {
	t.Helper()
	m, err := LoadManifest(cli)
	if err != nil {
		t.Fatalf("LoadManifest(%s): %v", cli, err)
	}
	fast := m.ModelTierMap["fast"]
	if fast == "" {
		t.Fatalf("%s manifest declares no fast model", cli)
	}
	return fast
}

func agyTmuxManifestWithTierMap(t *testing.T, mutate func(tiers map[string]string)) Manifest {
	t.Helper()
	m, err := LoadManifest("agy-tmux")
	if err != nil {
		t.Fatalf("LoadManifest(agy-tmux): %v", err)
	}
	tiers := maps.Clone(m.ModelTierMap)
	mutate(tiers)
	m.ModelTierMap = tiers
	return m
}

func modelFlagValue(t *testing.T, flags []string) string {
	t.Helper()
	i := slices.Index(flags, "--model")
	if i < 0 || i+1 >= len(flags) {
		t.Fatalf("no --model value in launch flags %v", flags)
	}
	return flags[i+1]
}

func bootAgyTmuxFromEngineIntent(t *testing.T, cfgModel string) string {
	t.Helper()
	cfg := &Config{CLI: "agy-tmux", Model: cfgModel}
	cfg.Realization = RealizeFor(cfg.CLI, launchIntentFor(cfg, Profile{}, policy.EffortTable{}))
	d, ok := LookupDriver("agy-tmux")
	if !ok {
		t.Fatal("agy-tmux is not a registered driver")
	}
	ws := t.TempDir()
	cfg.Workspace, cfg.Worktree, cfg.ProjectRoot = ws, ws, ws
	cfg.Agent, cfg.AllowBypass, cfg.BootOnly = "agy-every-launch-probe", true, true
	tm := &FakeTmuxController{CaptureFrames: slices.Repeat([]string{agyFooterGeminiFlash}, 12)}
	deps := Deps{
		Tmux: tm, Sleep: func(time.Duration) {}, Stderr: new(strings.Builder),
		LookupEnv: mapLookup(map[string]string{"EVOLVE_PHASE_RECOVERY": "off"}),
	}.withDefaults()
	if rc, err := d.Launch(context.Background(), cfg, deps); err != nil || rc == ExitBadFlags {
		t.Fatalf("agy-tmux Launch = (%d, %v), want a launch that reaches the pane", rc, err)
	}
	line, _ := launchLineFor(t, tm.SentKeys, "agy")
	return line
}

func TestAgyEveryLaunch_NoTierRealizesTheManifestFastModel(t *testing.T) {
	m := loadAgyManifestOffline(t)
	want := m.ModelTierMap["fast"]
	r := Realize(m, LaunchIntent{ModelTier: "auto"})
	if got := modelFlagValue(t, r.LaunchFlags); got != want {
		t.Errorf("tier auto: --model %q, want the manifest fast model %q", got, want)
	}
	if r.ModelOmitted != "" {
		t.Errorf("tier auto: ModelOmitted = %q, want the model passed", r.ModelOmitted)
	}
}

func TestAgyEveryLaunch_AnEmptyTierKeepsTheCLIsOwnDefaultModel(t *testing.T) {
	m := loadAgyManifestOffline(t)
	r := Realize(m, LaunchIntent{ModelTier: ""})
	if slices.Contains(r.LaunchFlags, "--model") {
		t.Errorf("empty tier is the CLI-health canary and must launch on agy's own default; flags = %v", r.LaunchFlags)
	}
}

func TestAgyEveryLaunch_UnresolvedTierRealizesTheManifestFastModel(t *testing.T) {
	m := agyTmuxManifestWithTierMap(t, func(tiers map[string]string) {
		delete(tiers, "deep")
		delete(tiers, "top")
	})
	want := m.ModelTierMap["fast"]
	for _, tier := range []string{"deep", "top"} {
		r := Realize(m, LaunchIntent{ModelTier: tier})
		if got := modelFlagValue(t, r.LaunchFlags); got != want {
			t.Errorf("unresolved tier %q: --model %q, want the manifest fast model %q", tier, got, want)
		}
	}
}

func TestAgyEveryLaunch_FallbackModelComesFromTheManifestMap(t *testing.T) {
	const syntheticFast = "Synthetic Fast (Test)"
	m := agyTmuxManifestWithTierMap(t, func(tiers map[string]string) { tiers["fast"] = syntheticFast })
	r := Realize(m, LaunchIntent{ModelTier: "auto"})
	if got := modelFlagValue(t, r.LaunchFlags); got != syntheticFast {
		t.Errorf("tier auto: --model %q, want %q read from model_tier_map", got, syntheticFast)
	}
}

func TestAgyEveryLaunch_ResolvedTierAndRawModelAreUnchanged(t *testing.T) {
	m := loadAgyManifestOffline(t)
	if got := modelFlagValue(t, Realize(m, LaunchIntent{ModelTier: "deep"}).LaunchFlags); got != m.ModelTierMap["deep"] {
		t.Errorf("deep tier: --model %q, want %q", got, m.ModelTierMap["deep"])
	}
	if got := modelFlagValue(t, Realize(m, LaunchIntent{ModelTier: "Gemini 3.1 Pro (High)"}).LaunchFlags); got != "Gemini 3.1 Pro (High)" {
		t.Errorf("raw model: --model %q, want it verbatim", got)
	}
}

func TestAgyEveryLaunch_OtherCLIsStillOmitTheModelForAnUnresolvedToken(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	for _, cli := range []string{"claude-tmux", "codex-tmux"} {
		r := RealizeFor(cli, LaunchIntent{ModelTier: "auto"})
		if slices.Contains(r.LaunchFlags, "--model") || slices.Contains(r.LaunchFlags, "auto") {
			t.Errorf("%s: tier auto must still omit the model; flags = %v", cli, r.LaunchFlags)
		}
	}
}

func TestAgyEveryLaunch_ProductionLaunchLineCarriesTheManifestFastModel(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	want := "--model " + shellQuotePOSIX(agyFastModel(t, "agy-tmux"))
	if line := bootAgyTmuxFromEngineIntent(t, "auto"); !strings.Contains(line, want) {
		t.Errorf("tier auto: agy launch line %q does not carry %s", line, want)
	}
	if line := bootAgyTmuxFromEngineIntent(t, ""); strings.Contains(line, "--model") {
		t.Errorf("empty tier: agy launch line %q must carry no --model", line)
	}
}

func TestAgyEveryLaunch_DoctorProbeCarriesTheManifestFastModel(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	d, ok := LookupDriver("agy")
	if !ok {
		t.Fatal("agy is not a registered driver")
	}
	prober, ok := d.(DeepProber)
	if !ok {
		t.Fatal("agy driver does not implement DeepProber")
	}
	m, err := LoadManifest("agy")
	if err != nil {
		t.Fatal(err)
	}
	args := prober.DeepProbeArgs(m)
	if got, want := modelFlagValue(t, args), m.ModelTierMap["fast"]; got != want {
		t.Errorf("agy -p probe --model %q, want the manifest fast model %q", got, want)
	}
}
