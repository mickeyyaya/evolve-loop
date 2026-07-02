package bridge

// model_tier_parity_test.go — cycle-447 Task 2 (model-tier-matrix-parity-pin):
// every embedded *-tmux CLI must translate the abstract intent.ModelTier
// through SOME effective channel — flag emits the flag+value, repl emits
// REPLInput, and ollama is classified effective-POSITIONAL (its model is the
// positional arg of `ollama run <model>`, never force-migrated to a flag).
// A multi-model CLI with channel:"noop" silently drops the tier — the exact
// defect agy-tmux carried until cycle-447 — so the parity rule REJECTS it
// (negative fixture below). The manifest glob is completeness-driven: a
// future *-tmux CLI added without an effective channel fails here, no
// hardcoded CLI list.

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// positionalTmuxCLIs names the manifests whose model_tier channel is
// legitimately noop because their DRIVER delivers the model positionally.
// The assertion is behavioral (the composed launch line must embed the
// model), so membership here is a claim the test verifies, not an exemption.
var positionalTmuxCLIs = map[string]func(m Manifest) error{
	"ollama-tmux": func(m Manifest) error {
		model := m.ModelTierMap["deep"]
		if model == "" {
			return fmt.Errorf("ollama-tmux model_tier_map has no deep entry")
		}
		if cmd := ollamaComposeLaunchCmd("ollama", model, nil); !strings.Contains(cmd, model) {
			return fmt.Errorf("positional launch line %q does not carry the model %q", cmd, model)
		}
		return nil
	},
}

// tierChannelEffective is the parity rule: does this manifest translate the
// abstract tier into something the CLI actually receives? flag/repl are
// verified behaviorally at the Realize seam; noop is acceptable ONLY for a
// verified positional driver or a genuinely single-model map (nothing to
// switch). A multi-model noop manifest is the silent-drop defect → error.
func tierChannelEffective(m Manifest) error {
	distinct := map[string]struct{}{}
	for _, tier := range []string{"fast", "balanced", "deep"} {
		if v := m.ModelTierMap[tier]; v != "" {
			distinct[v] = struct{}{}
		}
	}
	spec := m.Params["model_tier"]
	switch spec.Channel {
	case "flag":
		if spec.Flag == "" {
			return fmt.Errorf("channel=flag with empty flag name")
		}
		deep := m.ModelTierMap["deep"]
		r := Realize(m, LaunchIntent{ModelTier: "deep"})
		if deep == "" || !containsToken(r.LaunchFlags, spec.Flag) || !containsToken(r.LaunchFlags, deep) {
			return fmt.Errorf("Realize(tier=deep) did not emit %s %q (LaunchFlags=%v)", spec.Flag, deep, r.LaunchFlags)
		}
		return nil
	case "repl":
		if spec.Template == "" {
			return fmt.Errorf("channel=repl with empty template")
		}
		deep := m.ModelTierMap["deep"]
		r := Realize(m, LaunchIntent{ModelTier: "deep"})
		if deep == "" || len(r.REPLInput) == 0 || !strings.Contains(strings.Join(r.REPLInput, "\n"), deep) {
			return fmt.Errorf("Realize(tier=deep) did not seed REPL input with %q (REPLInput=%v)", deep, r.REPLInput)
		}
		return nil
	default: // noop / absent
		if check, ok := positionalTmuxCLIs[m.CLI]; ok {
			return check(m)
		}
		if len(distinct) > 1 {
			return fmt.Errorf("multi-model CLI (%d distinct tier models) with channel=%q silently drops intent.ModelTier", len(distinct), spec.Channel)
		}
		return nil
	}
}

// TestModelTierMatrixParity runs the parity rule over every embedded tmux
// manifest (one subtest per manifest base name) with the live-catalog overlay
// neutralized, plus the noop-rejection negative fixture that keeps the rule
// itself honest.
func TestModelTierMatrixParity(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	for _, name := range ManifestNames() {
		m, err := LoadManifest(name)
		if err != nil {
			t.Fatalf("LoadManifest(%s): %v", name, err)
		}
		if !m.IsTmux() {
			continue
		}
		t.Run(name, func(t *testing.T) {
			if err := tierChannelEffective(m); err != nil {
				t.Errorf("%s does not translate intent.ModelTier effectively: %v", name, err)
			}
		})
	}

	t.Run("synthetic-multimodel-NoopRejected", func(t *testing.T) {
		syn := Manifest{
			CLI: "synthetic-tmux", Binary: "syn", Transport: "tmux",
			Params:       map[string]ParamSpec{"model_tier": {Channel: "noop"}},
			ModelTierMap: map[string]string{"fast": "syn-a", "balanced": "syn-b", "deep": "syn-c"},
		}
		if err := tierChannelEffective(syn); err == nil {
			t.Error("a multi-model manifest with channel=noop must be REJECTED by the parity rule — it silently drops the tier")
		}
	})
}

// TestAgyTierDeepLaunchCarriesModelToPane is the integration-style pin: a
// DISPATCHED agy-tmux launch at tier=deep must carry the resolved deep model
// into the pane — a distinct property from Realize() merely emitting it
// (hypothesis H3: the launch line is one shell line via SendKeys, so the
// display-name token survives only if launchCmdLine quotes it). Drives the
// real agyTmuxDriver.Launch through the FakeTmuxController and asserts the
// launch keystroke line delivered to the pane embeds the quoted deep model.
func TestAgyTierDeepLaunchCarriesModelToPane(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	m, err := LoadManifest("agy-tmux")
	if err != nil {
		t.Fatalf("LoadManifest(agy-tmux): %v", err)
	}
	deep := m.ModelTierMap["deep"]
	if deep == "" {
		t.Fatal("agy-tmux model_tier_map has no deep entry")
	}

	ws := t.TempDir()
	cfg := &Config{
		Workspace:   ws,
		Worktree:    ws,
		ProjectRoot: ws,
		Agent:       "parity-probe",
		AllowBypass: true,
		BootOnly:    true, // boot-to-marker is enough: the launch line is sent before boot completes
		Realization: RealizeFor("agy-tmux", LaunchIntent{ModelTier: "deep", Permission: "bypass"}),
	}
	// Boot loop consumes ~2 frames per iteration (wait-loop capture + the
	// tickDuringBoot auto-responder capture); queue a healthy surplus of
	// idle-marker frames plus the cleanup scrollback capture.
	frames := make([]string, 12)
	for i := range frames {
		frames[i] = "? for shortcuts"
	}
	tm := &FakeTmuxController{CaptureFrames: frames}
	deps := Deps{
		Tmux:      tm,
		Sleep:     func(time.Duration) {},
		LookupEnv: mapLookup(map[string]string{"EVOLVE_PHASE_RECOVERY": "off"}),
		Stderr:    os.Stderr,
	}.withDefaults()

	code, err := agyTmuxDriver{}.Launch(context.Background(), cfg, deps)
	if err != nil || code != ExitOK {
		t.Fatalf("agyTmuxDriver.Launch = (%d, %v), want (ExitOK, nil)", code, err)
	}
	wantLine := "--model " + shellQuotePOSIX(deep)
	var launchLine string
	for _, sent := range tm.SentKeys {
		if strings.HasPrefix(sent, "agy ") || strings.Contains(sent, " agy ") {
			launchLine = sent
			break
		}
	}
	if launchLine == "" {
		t.Fatalf("no agy launch line reached the pane; SentKeys=%v", tm.SentKeys)
	}
	if !strings.Contains(launchLine, wantLine) {
		t.Errorf("launch line %q does not carry the resolved deep model (%q)", launchLine, wantLine)
	}
	if !strings.Contains(launchLine, "--dangerously-skip-permissions") {
		t.Errorf("launch line %q lost the permission realization", launchLine)
	}
}
