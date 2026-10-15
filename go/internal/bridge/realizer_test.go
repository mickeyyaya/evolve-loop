package bridge

import (
	"reflect"
	"testing"
)

func claudeTmuxManifest() Manifest {
	return Manifest{
		CLI: "claude-tmux", Binary: "claude",
		Params: map[string]ParamSpec{
			"permission":     {Channel: "flag", Values: map[string][]string{"bypass": {"--dangerously-skip-permissions"}, "plan": {"--permission-mode", "plan"}}},
			"model_tier":     {Channel: "flag", Flag: "--model", From: "tier_alias"},
			"settings_scope": {Channel: "flag", Values: map[string][]string{"project": {"--setting-sources", "project"}}},
			"session_mode":   {Channel: "controller"},
			"allowed_tools":  {Channel: "flag", Flag: "--allowedTools"},
		},
	}
}

func codexTmuxManifest() Manifest {
	return Manifest{
		CLI: "codex-tmux", Binary: "codex",
		ModelTierMap: map[string]string{"haiku": "gpt-5.4-mini", "sonnet": "gpt-5.4", "opus": "gpt-5.5"},
		Params: map[string]ParamSpec{
			"model_tier":     {Channel: "flag", Flag: "-m", From: "tier_alias"},
			"permission":     {Channel: "controller"},
			"settings_scope": {Channel: "noop"},
			"session_mode":   {Channel: "controller"},
		},
	}
}

func agyTmuxManifest() Manifest {
	return Manifest{
		CLI: "agy-tmux", Binary: "agy",
		ModelTierMap: map[string]string{"fast": "Gemini 3.5 Flash (Low)", "balanced": "Gemini 3.5 Flash (High)", "deep": "Gemini 3.1 Pro (High)"},
		Params: map[string]ParamSpec{
			"model_tier":     {Channel: "flag", Flag: "--model", From: "model_tier_map"},
			"permission":     {Channel: "flag", Values: map[string][]string{"bypass": {"--dangerously-skip-permissions"}}},
			"settings_scope": {Channel: "noop"},
			"session_mode":   {Channel: "controller"},
		},
	}
}

func TestRealize_PerCLI_SameIntentDifferentRealization(t *testing.T) {
	t.Parallel()
	intent := LaunchIntent{
		ModelTier:     "sonnet",
		Permission:    "bypass",
		SettingsScope: "project",
		SessionMode:   "ephemeral",
	}

	t.Run("claude-tmux: flags for everything it defines", func(t *testing.T) {
		got := Realize(claudeTmuxManifest(), intent)
		wantFlags := []string{"--dangerously-skip-permissions", "--model", "sonnet", "--setting-sources", "project"}
		if !sameFlags(got.LaunchFlags, wantFlags) {
			t.Fatalf("LaunchFlags = %v, want (any order) %v", got.LaunchFlags, wantFlags)
		}
		if len(got.REPLInput) != 0 {
			t.Fatalf("REPLInput = %v, want none", got.REPLInput)
		}
		if !got.Ephemeral {
			t.Fatal("session_mode=ephemeral must set controller Ephemeral=true")
		}
		if containsToken(got.LaunchFlags, "--no-session-persistence") {
			t.Fatal("--no-session-persistence must not be emitted for an interactive REPL")
		}
	})

	t.Run("agy-tmux: permission flag + --model display-name token (cycle-447)", func(t *testing.T) {
		got := Realize(agyTmuxManifest(), intent)
		wantFlags := []string{"--dangerously-skip-permissions", "--model", "Gemini 3.5 Flash (High)"}
		if !sameFlags(got.LaunchFlags, wantFlags) {
			t.Fatalf("LaunchFlags = %v, want (any order) %v", got.LaunchFlags, wantFlags)
		}
		if containsToken(got.LaunchFlags, "--setting-sources") {
			t.Fatalf("agy must not get claude-only flags; got %v", got.LaunchFlags)
		}
		if !got.Ephemeral {
			t.Fatal("ephemeral controller hint expected")
		}
	})

	t.Run("codex-tmux: model via -m launch flag (tier_alias), permission via controller", func(t *testing.T) {
		got := Realize(codexTmuxManifest(), intent)
		if !reflect.DeepEqual(got.LaunchFlags, []string{"-m", "gpt-5.4"}) {
			t.Fatalf("LaunchFlags = %v, want [-m gpt-5.4] (tier_alias sonnet→gpt-5.4)", got.LaunchFlags)
		}
		if len(got.REPLInput) != 0 {
			t.Fatalf("REPLInput = %v, want none (codex model is a launch flag, not REPL)", got.REPLInput)
		}
		if !got.Ephemeral {
			t.Fatal("ephemeral controller hint expected")
		}
	})
}

func TestRealize_REPLChannel(t *testing.T) {
	t.Parallel()
	m := Manifest{
		CLI:          "hypo-tmux",
		ModelTierMap: map[string]string{"sonnet": "model-x"},
		Params:       map[string]ParamSpec{"model_tier": {Channel: "repl", Template: "/model {alias}", From: "tier_alias"}},
	}
	got := Realize(m, LaunchIntent{ModelTier: "sonnet"})
	if !reflect.DeepEqual(got.REPLInput, []string{"/model model-x"}) {
		t.Fatalf("REPLInput = %v, want [/model model-x]", got.REPLInput)
	}
	if len(got.LaunchFlags) != 0 {
		t.Fatalf("repl channel must emit no launch flags; got %v", got.LaunchFlags)
	}
}

func TestRealize_NamedSessionMode(t *testing.T) {
	t.Parallel()
	got := Realize(claudeTmuxManifest(), LaunchIntent{SessionMode: "named:work-1"})
	if got.Ephemeral {
		t.Fatal("named session must not be ephemeral")
	}
	if got.SessionName != "work-1" {
		t.Fatalf("SessionName = %q, want work-1", got.SessionName)
	}
}

func TestRealize_UnknownIntentIsNoop(t *testing.T) {
	t.Parallel()
	got := Realize(Manifest{CLI: "bare"}, LaunchIntent{ModelTier: "sonnet", Permission: "bypass", SettingsScope: "project"})
	if len(got.LaunchFlags) != 0 || len(got.REPLInput) != 0 {
		t.Fatalf("bare manifest must realize to nothing; got flags=%v repl=%v", got.LaunchFlags, got.REPLInput)
	}
}

func TestRealize_RawEscapeHatchAppliesOnlyToMatchingCLI(t *testing.T) {
	t.Parallel()
	intent := LaunchIntent{RawByCLI: map[string][]string{"claude-tmux": {"--foo", "bar"}, "agy-tmux": {"--baz"}}}
	cl := Realize(claudeTmuxManifest(), intent)
	if !containsToken(cl.LaunchFlags, "--foo") || !containsToken(cl.LaunchFlags, "bar") {
		t.Fatalf("claude should get its raw escape-hatch flags; got %v", cl.LaunchFlags)
	}
	if containsToken(cl.LaunchFlags, "--baz") {
		t.Fatalf("claude must NOT get agy's raw flags; got %v", cl.LaunchFlags)
	}
	ag := Realize(agyTmuxManifest(), intent)
	if !containsToken(ag.LaunchFlags, "--baz") || containsToken(ag.LaunchFlags, "--foo") {
		t.Fatalf("agy should get only its own raw flags; got %v", ag.LaunchFlags)
	}
}

func TestRealize_AllowedToolsExpandsFlag(t *testing.T) {
	t.Parallel()
	got := Realize(claudeTmuxManifest(), LaunchIntent{AllowedTools: []string{"Read", "Write"}})
	want := []string{"--allowedTools", "Read", "Write"}
	if !reflect.DeepEqual(got.LaunchFlags, want) {
		t.Fatalf("allowed_tools = %v, want %v", got.LaunchFlags, want)
	}
}

func TestRealize_DefaultArgs_LandFirst(t *testing.T) {
	t.Parallel()
	m := Manifest{
		CLI:         "hypo",
		DefaultArgs: []string{"--always", "--on"},
		Params:      map[string]ParamSpec{"permission": {Channel: "flag", Values: map[string][]string{"bypass": {"--bypass"}}}},
	}
	got := Realize(m, LaunchIntent{Permission: "bypass"})
	want := []string{"--always", "--on", "--bypass"}
	if !reflect.DeepEqual(got.LaunchFlags, want) {
		t.Fatalf("default_args must land FIRST; got %v, want %v", got.LaunchFlags, want)
	}

	mEmpty := Manifest{
		CLI:         "hypo-empty",
		DefaultArgs: []string{},
		Params:      map[string]ParamSpec{"permission": {Channel: "flag", Values: map[string][]string{"bypass": {"--bypass"}}}},
	}
	gotEmpty := Realize(mEmpty, LaunchIntent{Permission: "bypass"})
	if !reflect.DeepEqual(gotEmpty.LaunchFlags, []string{"--bypass"}) {
		t.Fatalf("empty default_args must be a no-op; got %v, want [--bypass]", gotEmpty.LaunchFlags)
	}

	mNil := Manifest{
		CLI:    "hypo-nil",
		Params: map[string]ParamSpec{"permission": {Channel: "flag", Values: map[string][]string{"bypass": {"--bypass"}}}},
	}
	gotNil := Realize(mNil, LaunchIntent{Permission: "bypass"})
	if !reflect.DeepEqual(gotNil.LaunchFlags, []string{"--bypass"}) {
		t.Fatalf("nil default_args must be a no-op; got %v, want [--bypass]", gotNil.LaunchFlags)
	}
}

func TestRealize_DefaultArgs_Deduped(t *testing.T) {
	t.Parallel()
	m := Manifest{
		CLI:         "hypo",
		DefaultArgs: []string{"--bypass"},
		Params:      map[string]ParamSpec{"permission": {Channel: "flag", Values: map[string][]string{"bypass": {"--bypass"}}}},
	}
	got := Realize(m, LaunchIntent{Permission: "bypass"})
	if !reflect.DeepEqual(got.LaunchFlags, []string{"--bypass"}) {
		t.Fatalf("colliding token must dedupe to one; got %v, want [--bypass]", got.LaunchFlags)
	}

	m2 := Manifest{
		CLI:         "hypo2",
		DefaultArgs: []string{"--model", "gpt-5.4"},
		Params: map[string]ParamSpec{
			"model_tier": {Channel: "flag", Flag: "--model", From: "tier_alias"},
		},
		ModelTierMap: map[string]string{"sonnet": "gpt-5.5"},
	}
	got2 := Realize(m2, LaunchIntent{ModelTier: "sonnet"})
	if !reflect.DeepEqual(got2.LaunchFlags, []string{"--model", "gpt-5.4", "--model", "gpt-5.5"}) {
		t.Fatalf("pair-level dedupe expected; got %v, want [--model gpt-5.4 --model gpt-5.5]", got2.LaunchFlags)
	}
}

func TestDedupeLaunchFlags_Edges(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"nil → nil (early return; len<=1)", nil, nil},
		{"empty → empty (early return; len<=1)", []string{}, []string{}},
		{"single → unchanged (early return; len<=1)", []string{"--a"}, []string{"--a"}},
		{"two distinct → unchanged", []string{"--a", "--b"}, []string{"--a", "--b"}},
		{"two identical → one (collision dedup)", []string{"--a", "--a"}, []string{"--a"}},
		{"all-identical run → one (worst-case dedup)", []string{"--a", "--a", "--a", "--a"}, []string{"--a"}},
		{
			"flag-value pair declared twice → token-level dedup keeps the pair",
			[]string{"--flag", "value", "--flag", "value"},
			[]string{"--flag", "value"},
		},
		{
			"flag-value pair with DISTINCT values → both pairs kept intact",
			[]string{"--model", "x", "--model", "y"},
			[]string{"--model", "x", "--model", "y"},
		},
		{
			"heterogeneous duplicates → first occurrence wins, order preserved",
			[]string{"--a", "--b", "--a", "--c", "--b"},
			[]string{"--a", "--b", "--c"},
		},
		{
			"empty-string tokens collapse to one (defensive — manifest typo)",
			[]string{"", "--a", ""},
			[]string{"", "--a"},
		},
		{
			"unicode + ascii mix preserved",
			[]string{"--lang", "日本語", "--lang", "日本語"},
			[]string{"--lang", "日本語"},
		},
		{
			"order: late-occurring distinct value lands at the tail",
			[]string{"--a", "--b", "--c", "--a"},
			[]string{"--a", "--b", "--c"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dedupeLaunchFlags(tc.in)
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("dedupeLaunchFlags(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestDedupeLaunchFlags_AliasSafety(t *testing.T) {
	t.Parallel()
	in := []string{"--a", "--b", "--a", "--c"}
	original := append([]string(nil), in...)
	got := dedupeLaunchFlags(in)

	if len(got) > 0 {
		got[0] = "--MUTATED"
	}
	if !reflect.DeepEqual(in, original) {
		t.Fatalf("dedupe result aliases input backing array; in mutated to %v (was %v)", in, original)
	}

	in[0] = "--TAMPERED"
	if got[0] == "--TAMPERED" {
		t.Fatalf("input mutation leaked into dedupe result; got[0]=%q", got[0])
	}
}

func TestRealize_DefaultArgs_StackInteraction(t *testing.T) {
	t.Parallel()
	m := Manifest{
		CLI:          "stack",
		DefaultArgs:  []string{"--default-1", "--default-2"},
		ModelTierMap: map[string]string{"sonnet": "model-x"},
		Params: map[string]ParamSpec{
			"model_tier":     {Channel: "flag", Flag: "--model", From: "tier_alias"},
			"permission":     {Channel: "flag", Values: map[string][]string{"bypass": {"--bypass"}}},
			"settings_scope": {Channel: "flag", Values: map[string][]string{"project": {"--scope", "project"}}},
		},
	}
	intent := LaunchIntent{
		ModelTier:     "sonnet",
		Permission:    "bypass",
		SettingsScope: "project",
		RawByCLI:      map[string][]string{"stack": {"--raw-tail"}},
	}
	got := Realize(m, intent)

	want := []string{
		"--default-1", "--default-2",
		"--model", "model-x",
		"--bypass",
		"--scope", "project",
		"--raw-tail",
	}
	if !reflect.DeepEqual(got.LaunchFlags, want) {
		t.Fatalf("stack order broken;\ngot:  %v\nwant: %v", got.LaunchFlags, want)
	}
}

func TestRealize_DefaultArgs_DegenerateManifest(t *testing.T) {
	t.Parallel()
	m := Manifest{CLI: "bare", DefaultArgs: []string{"--only-this"}}
	got := Realize(m, LaunchIntent{ModelTier: "sonnet", Permission: "bypass"})
	if !reflect.DeepEqual(got.LaunchFlags, []string{"--only-this"}) {
		t.Fatalf("default_args must fire on degenerate manifest; got %v", got.LaunchFlags)
	}
	if len(got.REPLInput) != 0 {
		t.Fatalf("degenerate manifest must not emit REPL input; got %v", got.REPLInput)
	}
}

func TestRealize_DefaultArgs_InternalDuplicates(t *testing.T) {
	t.Parallel()
	m := Manifest{
		CLI:         "dup-default",
		DefaultArgs: []string{"--yolo", "--yolo", "--yolo"},
	}
	got := Realize(m, LaunchIntent{})
	if !reflect.DeepEqual(got.LaunchFlags, []string{"--yolo"}) {
		t.Fatalf("internal default_args duplicates must collapse; got %v", got.LaunchFlags)
	}
}

func TestRealize_DefaultArgs_OrthogonalToREPLChannel(t *testing.T) {
	t.Parallel()
	m := Manifest{
		CLI:          "mixed",
		DefaultArgs:  []string{"--launch-only"},
		ModelTierMap: map[string]string{"sonnet": "model-x"},
		Params: map[string]ParamSpec{
			"model_tier": {Channel: "repl", Template: "/model {alias}", From: "tier_alias"},
		},
	}
	got := Realize(m, LaunchIntent{ModelTier: "sonnet"})
	if !reflect.DeepEqual(got.LaunchFlags, []string{"--launch-only"}) {
		t.Fatalf("default_args must reach LaunchFlags only; got %v", got.LaunchFlags)
	}
	if !reflect.DeepEqual(got.REPLInput, []string{"/model model-x"}) {
		t.Fatalf("REPL channel must reach REPLInput; got %v", got.REPLInput)
	}
}

func TestRealize_DefaultArgs_EmptyTokenDefensive(t *testing.T) {
	t.Parallel()
	m := Manifest{CLI: "typo", DefaultArgs: []string{"", "--real", ""}}
	got := Realize(m, LaunchIntent{})
	if len(got.LaunchFlags) != 2 || got.LaunchFlags[1] != "--real" {
		t.Fatalf("empty + real tokens; got %v (want a 2-elem slice with --real)", got.LaunchFlags)
	}
}

func sameFlags(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	m := map[string]int{}
	for _, g := range got {
		m[g]++
	}
	for _, w := range want {
		m[w]--
	}
	for _, v := range m {
		if v != 0 {
			return false
		}
	}
	return true
}

func containsToken(s []string, tok string) bool {
	for _, x := range s {
		if x == tok {
			return true
		}
	}
	return false
}
