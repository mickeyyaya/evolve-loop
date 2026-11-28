package bridge

import (
	"errors"
	"os"
	"reflect"
	"testing"
)

var claudePSeatParams = []string{"permission", "settings_scope", "system_prompt_file", "model_tier"}

func TestLoadManifest_ParamsFrom_ClaudePProjectsTheClaudeTmuxSeatParams(t *testing.T) {
	t.Parallel()
	headless, err := LoadManifest("claude-p")
	if err != nil {
		t.Fatal(err)
	}
	tmux, err := LoadManifest("claude-tmux")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := loadManifestRaw("claude-p")
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range claudePSeatParams {
		if _, own := raw.Params[key]; own {
			t.Errorf("claude-p.json declares %s itself, want it projected from claude-tmux", key)
		}
		if !reflect.DeepEqual(headless.Params[key], tmux.Params[key]) {
			t.Errorf("param %s = %+v, want claude-tmux's %+v", key, headless.Params[key], tmux.Params[key])
		}
	}
}

func TestClaudePRealization_SeatFlagsEqualClaudeTmux(t *testing.T) {
	t.Parallel()
	headless, _ := LoadManifest("claude-p")
	tmux, _ := LoadManifest("claude-tmux")
	for _, permission := range []string{"bypass", "plan", "acceptEdits", "default", "dontAsk", "auto"} {
		intent := LaunchIntent{ModelTier: "deep", Permission: permission, SettingsScope: "project", SystemPromptFile: "/w/pane-authority.md"}
		got := RealizeFor("claude-p", intent).LaunchFlags[len(headless.DefaultArgs):]
		want := RealizeFor("claude-tmux", intent).LaunchFlags[len(tmux.DefaultArgs):]
		if len(want) == 0 || !reflect.DeepEqual(got, want) {
			t.Errorf("permission %s: claude-p flags %v, want claude-tmux's %v", permission, got, want)
		}
	}
}

func TestResolveParamsFrom(t *testing.T) {
	t.Parallel()
	source := Manifest{Params: map[string]ParamSpec{"permission": {Channel: "flag", Flag: "-p"}}}
	load := func(cli string) (Manifest, error) {
		if cli != "src" {
			return Manifest{}, os.ErrNotExist
		}
		return source, nil
	}
	spec := func(cli string) *ParamsFromSpec { return &ParamsFromSpec{CLI: cli, Keys: []string{"permission"}} }
	if got, err := resolveParamsFrom("x", Manifest{}, load); err != nil || got.Params != nil {
		t.Errorf("no params_from = (%+v, %v), want the manifest untouched", got.Params, err)
	}
	if _, err := resolveParamsFrom("x", Manifest{ParamsFrom: spec("none")}, load); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("an unresolvable cli = %v, want the load error", err)
	}
	if _, err := resolveParamsFrom("x", Manifest{ParamsFrom: &ParamsFromSpec{CLI: "src", Keys: []string{"absent"}}}, load); err == nil {
		t.Error("a key that the source lacks must error")
	}
	got, err := resolveParamsFrom("x", Manifest{ParamsFrom: spec("src")}, load)
	if err != nil || got.Params["permission"].Flag != "-p" {
		t.Errorf("projection = (%+v, %v), want the source param", got.Params, err)
	}
	own := Manifest{ParamsFrom: spec("src"), Params: map[string]ParamSpec{"permission": {Flag: "-own"}, "effort": {Flag: "-e"}}}
	got, err = resolveParamsFrom("x", own, load)
	if err != nil || got.Params["permission"].Flag != "-own" || got.Params["effort"].Flag != "-e" {
		t.Errorf("own params = (%+v, %v), want them to win and stay", got.Params, err)
	}
}
