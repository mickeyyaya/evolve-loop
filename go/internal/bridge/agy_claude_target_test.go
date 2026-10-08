package bridge

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelquery"
)

func agyTmuxOverrideWithEnv(t *testing.T, env map[string]string) string {
	t.Helper()
	raw, err := embeddedManifests.ReadFile("manifests/agy-tmux.json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	m["default_env"] = env
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "agy-tmux.json"), out, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func bootAgyClaude(t *testing.T, intent LaunchIntent) []string {
	t.Helper()
	return bootAgyClaudeLogged(t, intent, io.Discard)
}

func bootAgyClaudeLogged(t *testing.T, intent LaunchIntent, stderr io.Writer) []string {
	t.Helper()
	d, ok := LookupDriver("agy-claude-tmux")
	if !ok {
		t.Fatal("agy-claude-tmux is not a registered driver")
	}
	ws := t.TempDir()
	cfg := &Config{
		Workspace: ws, Worktree: ws, ProjectRoot: ws, Agent: "agy-claude-probe",
		AllowBypass: true, BootOnly: true,
		Realization: RealizeFor("agy-claude-tmux", intent),
	}
	tm := &FakeTmuxController{CaptureFrames: slices.Repeat([]string{agyFooterClaudeOpus}, 12)}
	deps := Deps{
		Tmux: tm, Sleep: func(time.Duration) {}, Stderr: stderr,
		LookupEnv: mapLookup(map[string]string{"EVOLVE_PHASE_RECOVERY": "off"}),
	}.withDefaults()
	if code, err := d.Launch(context.Background(), cfg, deps); err != nil || code != ExitOK {
		t.Fatalf("agy-claude-tmux Launch = (%d, %v), want (ExitOK, nil)", code, err)
	}
	return tm.SentKeys
}

func TestAgyClaudeTmux_IsARegisteredTmuxTargetThatDrivesTheAgyBinaryAsAgyTmuxDoes(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	if _, ok := LookupDriver("agy-claude-tmux"); !ok {
		t.Fatal("agy-claude-tmux is not a registered driver")
	}
	target, err := LoadManifest("agy-claude-tmux")
	if err != nil {
		t.Fatalf("LoadManifest(agy-claude-tmux): %v", err)
	}
	agy, err := LoadManifest("agy-tmux")
	if err != nil {
		t.Fatalf("LoadManifest(agy-tmux): %v", err)
	}
	if target.CLI != "agy-claude-tmux" || target.Binary != "agy" || !target.IsTmux() {
		t.Fatalf("agy-claude-tmux = cli %q binary %q tmux %v; want its own name on the agy binary over tmux", target.CLI, target.Binary, target.IsTmux())
	}
	same := []struct {
		what        string
		target, agy any
	}{
		{"interactive_prompts", target.InteractivePrompts, agy.InteractivePrompts},
		{"controls", target.Controls, agy.Controls},
		{"prompt_marker", target.PromptMarker, agy.PromptMarker},
		{"transient_regex", target.TransientRegex, agy.TransientRegex},
		{"default_env", target.DefaultEnv, agy.DefaultEnv},
		{"default_args", target.DefaultArgs, agy.DefaultArgs},
		{"tier_dependencies", target.TierDependencies, agy.TierDependencies},
		{"params.permission", target.Params["permission"], agy.Params["permission"]},
	}
	for _, s := range same {
		if !reflect.DeepEqual(s.target, s.agy) {
			t.Errorf("agy-claude-tmux %s = %v, want agy-tmux's %v: one binary, one way to drive it", s.what, s.target, s.agy)
		}
	}
}

func TestAgyClaudeTmux_TierMapRunsSonnetForFastAndBalancedAndOpusForDeepAndTop(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	m, err := LoadManifest("agy-claude-tmux")
	if err != nil {
		t.Fatalf("LoadManifest(agy-claude-tmux): %v", err)
	}
	want := map[string]string{
		"fast":     "Claude Sonnet 5.5 (Low)",
		"balanced": "Claude Sonnet 5.5 (Medium)",
		"deep":     "Claude Opus 5.5 (Medium)",
		"top":      "Claude Opus 5.5 (Medium)",
	}
	if !reflect.DeepEqual(m.ModelTierMap, want) {
		t.Fatalf("agy-claude-tmux model_tier_map = %v, want %v", m.ModelTierMap, want)
	}
}

func TestAgyClaudeTmux_DeepLaunchIsAgyWithClaudeOpusAndTheAgyEnvironment(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	useBridgeManifestDir(t, agyTmuxOverrideWithEnv(t, map[string]string{agyAutoUpdateOffEnv: "1"}))

	sent := bootAgyClaude(t, LaunchIntent{ModelTier: "deep", Permission: "bypass"})

	line, launchAt := launchLineFor(t, sent, "agy")
	if want := "--model " + shellQuotePOSIX("Claude Opus 5.5 (Medium)"); !strings.Contains(line, want) {
		t.Errorf("deep launch line %q does not carry %s", line, want)
	}
	if !strings.Contains(line, "--dangerously-skip-permissions") {
		t.Errorf("deep launch line %q lost agy's permission realization", line)
	}
	exportAt := slices.Index(sent, "export "+agyAutoUpdateOffEnv+"=1")
	if exportAt < 0 || exportAt > launchAt {
		t.Errorf("keys sent = %q; want `export %s=1` before the agy launch line: the agy binary's environment reaches every agy target", sent, agyAutoUpdateOffEnv)
	}
}

func TestAgyClaudeTmux_ALaunchWithNoModelStillRunsAClaudeModel(t *testing.T) {
	injectCatalogDir(t, t.TempDir())

	sent := bootAgyClaude(t, LaunchIntent{Permission: "bypass"})

	line, _ := launchLineFor(t, sent, "agy")
	if want := "--model " + shellQuotePOSIX("Claude Sonnet 5.5 (Low)"); !strings.Contains(line, want) {
		t.Errorf("model-less launch line %q does not carry %s: agy's own default is a Gemini model, so a probe of this target would test the wrong quota", line, want)
	}
}

func TestAgyClaudeTmux_LaunchesWithWhateverEnvironmentAgyTmuxDeclares(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	for _, env := range []map[string]string{nil, {agyAutoUpdateOffEnv: "1"}, {agyAutoUpdateOffEnv: "1", "AGY_OTHER": "x"}} {
		useBridgeManifestDir(t, agyTmuxOverrideWithEnv(t, env))
		claude := RealizeFor("agy-claude-tmux", LaunchIntent{}).Env
		gemini := RealizeFor("agy-tmux", LaunchIntent{}).Env
		if !reflect.DeepEqual(claude, gemini) {
			t.Errorf("agy-tmux env %v: agy-claude-tmux launches with %v, agy-tmux with %v", env, claude, gemini)
		}
	}
}

func TestModelFamily_NamesTheFamilyOfTheModelsATargetDispatches(t *testing.T) {
	cases := map[string]string{
		"agy-tmux":        "gemini",
		"agy":             "gemini",
		"agy-claude-tmux": "claude",
		"claude-tmux":     "claude",
		"claude-p":        "claude",
		"codex":           "gpt",
		"codex-tmux":      "gpt",
		"ollama-tmux":     "local",
		"agy-claude":      "claude",
		"claude":          "claude",
		"ollama":          "local",
		"gpt-cli":         "",
		"":                "",
	}
	for name, want := range cases {
		if got := ModelFamily(name); got != want {
			t.Errorf("ModelFamily(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestModelFamily_EveryEmbeddedManifestDeclaresOneTheTierMapKeepsTo(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	lineageFamilies := []string{"claude", "gemini", "gpt"}
	for _, name := range ManifestNames() {
		m, err := LoadManifest(name)
		if err != nil {
			t.Fatalf("LoadManifest(%s): %v", name, err)
		}
		if !slices.Contains(append(lineageFamilies, "local"), m.ModelFamily) {
			t.Errorf("%s model_family = %q, want one of claude, gemini, gpt, local", name, m.ModelFamily)
			continue
		}
		if !slices.Contains(lineageFamilies, m.ModelFamily) {
			continue
		}
		for tier, model := range m.ModelTierMap {
			if got := modelquery.FamilyOf(model); got != m.ModelFamily {
				t.Errorf("%s declares model_family %q, but its %s tier runs %q (family %q)", name, m.ModelFamily, tier, model, got)
			}
		}
	}
}

func writeManifestFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name+".json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestManifestBase_ATargetIsAMergePatchOverItsBinarysManifest(t *testing.T) {
	dir := t.TempDir()
	writeManifestFile(t, dir, "fake-tmux", `{"cli":"fake-tmux","binary":"fake","transport":"tmux","prompt_marker":"> ",
		"default_env":{"FAKE_OFF":"1"},"model_tier_map":{"fast":"f1","deep":"f3"},
		"params":{"model_tier":{"channel":"flag","flag":"--model","from":"model_tier_map"},"permission":{"channel":"noop"}},
		"default_args":["--quiet"]}`)
	writeManifestFile(t, dir, "fake-alt-tmux", `{"cli":"fake-alt-tmux","base":"fake-tmux","model_family":"alt",
		"model_tier_map":{"fast":"a1"},"params":{"model_tier":{"default":"fast"}},"default_args":null}`)
	useBridgeManifestDir(t, dir)

	m, err := loadManifestRaw("fake-alt-tmux")
	if err != nil {
		t.Fatalf("loadManifestRaw: %v", err)
	}
	if m.CLI != "fake-alt-tmux" || m.Binary != "fake" || !m.IsTmux() || m.PromptMarker != "> " || m.ModelFamily != "alt" {
		t.Errorf("target = %+v; want its own cli and model_family, the base's binary, transport and marker", m)
	}
	if !reflect.DeepEqual(m.DefaultEnv, map[string]string{"FAKE_OFF": "1"}) {
		t.Errorf("default_env = %v, want the base's", m.DefaultEnv)
	}
	if want := map[string]string{"fast": "a1", "deep": "f3"}; !reflect.DeepEqual(m.ModelTierMap, want) {
		t.Errorf("model_tier_map = %v, want %v: objects merge key by key", m.ModelTierMap, want)
	}
	if want := (ParamSpec{Channel: "flag", Flag: "--model", From: "model_tier_map", Default: "fast"}); !reflect.DeepEqual(m.Params["model_tier"], want) {
		t.Errorf("params.model_tier = %+v, want %+v", m.Params["model_tier"], want)
	}
	if m.Params["permission"].Channel != "noop" {
		t.Errorf("params.permission = %+v, want the base's", m.Params["permission"])
	}
	if len(m.DefaultArgs) != 0 {
		t.Errorf("default_args = %v; a null in the target removes the base's key", m.DefaultArgs)
	}
}

func TestManifestBase_RefusesABaseThatCannotBeResolved(t *testing.T) {
	cases := map[string]struct {
		files map[string]string
		want  string
	}{
		"missing base": {map[string]string{
			"fake-alt-tmux": `{"cli":"fake-alt-tmux","base":"absent-tmux"}`,
		}, `base="absent-tmux"`},
		"chained base": {map[string]string{
			"fake-root-tmux": `{"cli":"fake-root-tmux","binary":"fake"}`,
			"fake-tmux":      `{"cli":"fake-tmux","base":"fake-root-tmux"}`,
			"fake-alt-tmux":  `{"cli":"fake-alt-tmux","base":"fake-tmux"}`,
		}, "itself names a base"},
		"malformed base": {map[string]string{
			"fake-tmux":     `{"cli":`,
			"fake-alt-tmux": `{"cli":"fake-alt-tmux","base":"fake-tmux"}`,
		}, "invalid JSON"},
		"base not a name": {map[string]string{
			"fake-alt-tmux": `{"cli":"fake-alt-tmux","base":7}`,
		}, "base must name"},
		"empty base": {map[string]string{
			"fake-alt-tmux": `{"cli":"fake-alt-tmux","base":""}`,
		}, "base must name"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			for file, body := range tc.files {
				writeManifestFile(t, dir, file, body)
			}
			useBridgeManifestDir(t, dir)
			_, err := loadManifestRaw("fake-alt-tmux")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("loadManifestRaw err = %v, want one naming %q", err, tc.want)
			}
		})
	}
}

func TestManifestBase_AMalformedTargetIsStillReportedAsInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	writeManifestFile(t, dir, "fake-alt-tmux", `{"cli":`)
	useBridgeManifestDir(t, dir)
	if _, err := loadManifestRaw("fake-alt-tmux"); err == nil || !strings.Contains(err.Error(), "invalid JSON for cli=fake-alt-tmux") {
		t.Fatalf("loadManifestRaw err = %v, want the parser's invalid JSON error", err)
	}
}

func TestInteractiveFamilies_AgyClaudeTmuxSharesTheAgyBinary(t *testing.T) {
	got := interactiveFamiliesFrom([]string{"agy-claude-tmux", "agy-tmux", "claude-tmux"}, loadManifestRaw, func(string) bool { return true })
	if want := []string{"agy", "claude"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("installed interactive binaries = %v, want %v: the usage probe and the updater run once per binary", got, want)
	}
}
