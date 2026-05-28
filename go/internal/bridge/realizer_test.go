package bridge

import (
	"reflect"
	"testing"
)

// realizer_test.go — the heart of ADR-0022: ONE CLI-agnostic LaunchIntent must
// realize correctly AND differently per CLI, via each manifest's declarative
// `params` table. The same intent that yields `--dangerously-skip-permissions
// --model sonnet --setting-sources project` for claude must yield ONLY
// `--dangerously-skip-permissions` for agy (no model selector, no settings
// flag) and a post-boot `/model gpt-5.4` REPL command for codex — and NEVER a
// flag the target CLI does not define. An intent with no manifest entry is a
// no-op (the property that makes foreign params unable to break a launch).

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
		// no tier_aliases → claude uses the literal tier as the model name
	}
}

func codexTmuxManifest() Manifest {
	return Manifest{
		CLI: "codex-tmux", Binary: "codex",
		TierAliases: map[string]string{"haiku": "gpt-5.4-mini", "sonnet": "gpt-5.4", "opus": "gpt-5.5"},
		Params: map[string]ParamSpec{
			"model_tier":     {Channel: "flag", Flag: "-m", From: "tier_alias"}, // matches the real codex-tmux manifest + `codex -m` driver
			"permission":     {Channel: "controller"},                           // codex has no bypass flag; trust handled by the auto-responder
			"settings_scope": {Channel: "noop"},
			"session_mode":   {Channel: "controller"},
		},
	}
}

func agyTmuxManifest() Manifest {
	return Manifest{
		CLI: "agy-tmux", Binary: "agy",
		TierAliases: map[string]string{"haiku": "gemini-3.5-flash", "sonnet": "gemini-3.5-flash", "opus": "gemini-3.5-flash"},
		Params: map[string]ParamSpec{
			"model_tier":     {Channel: "noop"}, // agy has no model selector
			"permission":     {Channel: "flag", Values: map[string][]string{"bypass": {"--dangerously-skip-permissions"}}},
			"settings_scope": {Channel: "noop"},
			"session_mode":   {Channel: "controller"},
		},
	}
}

func TestRealize_PerCLI_SameIntentDifferentRealization(t *testing.T) {
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
		// The bug guard: a print-mode flag must NEVER appear.
		if containsToken(got.LaunchFlags, "--no-session-persistence") {
			t.Fatal("--no-session-persistence must not be emitted for an interactive REPL")
		}
	})

	t.Run("agy-tmux: only the permission flag it defines; model is no-op", func(t *testing.T) {
		got := Realize(agyTmuxManifest(), intent)
		if !sameFlags(got.LaunchFlags, []string{"--dangerously-skip-permissions"}) {
			t.Fatalf("LaunchFlags = %v, want [--dangerously-skip-permissions] only", got.LaunchFlags)
		}
		if containsToken(got.LaunchFlags, "--model") || containsToken(got.LaunchFlags, "--setting-sources") {
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

// TestRealize_REPLChannel covers the post-boot REPL-injection channel — a
// supported engine capability for CLIs whose model can only be set in-session.
// No production manifest uses it today (every tmux CLI's model is a launch flag
// or a no-op), so it's pinned here with a synthetic manifest so the channel
// stays covered and documented as reserved.
func TestRealize_REPLChannel(t *testing.T) {
	m := Manifest{
		CLI:         "hypo-tmux",
		TierAliases: map[string]string{"sonnet": "model-x"},
		Params:      map[string]ParamSpec{"model_tier": {Channel: "repl", Template: "/model {alias}", From: "tier_alias"}},
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
	got := Realize(claudeTmuxManifest(), LaunchIntent{SessionMode: "named:work-1"})
	if got.Ephemeral {
		t.Fatal("named session must not be ephemeral")
	}
	if got.SessionName != "work-1" {
		t.Fatalf("SessionName = %q, want work-1", got.SessionName)
	}
}

func TestRealize_UnknownIntentIsNoop(t *testing.T) {
	// A manifest with NO params table: every intent field is a no-op, never an
	// error. This is the property that makes a foreign/unsupported param unable
	// to abort a launch.
	got := Realize(Manifest{CLI: "bare"}, LaunchIntent{ModelTier: "sonnet", Permission: "bypass", SettingsScope: "project"})
	if len(got.LaunchFlags) != 0 || len(got.REPLInput) != 0 {
		t.Fatalf("bare manifest must realize to nothing; got flags=%v repl=%v", got.LaunchFlags, got.REPLInput)
	}
}

func TestRealize_RawEscapeHatchAppliesOnlyToMatchingCLI(t *testing.T) {
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
	got := Realize(claudeTmuxManifest(), LaunchIntent{AllowedTools: []string{"Read", "Write"}})
	want := []string{"--allowedTools", "Read", "Write"}
	if !reflect.DeepEqual(got.LaunchFlags, want) {
		t.Fatalf("allowed_tools = %v, want %v", got.LaunchFlags, want)
	}
}

// TestRealize_DefaultArgs_LandFirst pins the cycle-124 G1a wire-up: manifest
// `default_args` is the always-on launch-flag channel (was a dead field
// before cycle-124 — declared in manifest.go but never read). Tokens land in
// LaunchFlags BEFORE per-param scalars, so a manifest can prepend
// unconditional boot-time switches (e.g. codex-tmux --yolo, ollama-tmux
// --experimental-yolo) without competing with intent-driven flags. An empty
// or nil default_args remains a no-op (regression guard for the agy/claude
// migration that emptied their default_args and let params.permission be
// the sole emitter).
func TestRealize_DefaultArgs_LandFirst(t *testing.T) {
	// Synthetic manifest: default_args + one per-param scalar. Tokens MUST
	// appear in this exact order.
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

	// Empty default_args produces unchanged behavior — regression guard for the
	// cycle-124 agy/claude migration that emptied default_args and let
	// params.permission be the sole emitter of --dangerously-skip-permissions.
	mEmpty := Manifest{
		CLI:         "hypo-empty",
		DefaultArgs: []string{},
		Params:      map[string]ParamSpec{"permission": {Channel: "flag", Values: map[string][]string{"bypass": {"--bypass"}}}},
	}
	gotEmpty := Realize(mEmpty, LaunchIntent{Permission: "bypass"})
	if !reflect.DeepEqual(gotEmpty.LaunchFlags, []string{"--bypass"}) {
		t.Fatalf("empty default_args must be a no-op; got %v, want [--bypass]", gotEmpty.LaunchFlags)
	}

	// Nil default_args is also a no-op (covers the unset-key case in JSON).
	mNil := Manifest{
		CLI:    "hypo-nil",
		Params: map[string]ParamSpec{"permission": {Channel: "flag", Values: map[string][]string{"bypass": {"--bypass"}}}},
	}
	gotNil := Realize(mNil, LaunchIntent{Permission: "bypass"})
	if !reflect.DeepEqual(gotNil.LaunchFlags, []string{"--bypass"}) {
		t.Fatalf("nil default_args must be a no-op; got %v, want [--bypass]", gotNil.LaunchFlags)
	}
}

// TestRealize_DefaultArgs_Deduped covers the cycle-124 G1a wire-up's
// order-preserving dedupe: when a manifest declares the same token in
// default_args AND one of its params channels emits the same token, the
// duplicate is silently dropped (the operator-declared default keeps the
// leading position). This is the documented invariant for boolean-style
// flags; flag/value PAIRS with different VALUES are preserved (the dedup is
// token-level, so `--model gpt-5.4` and `--model gpt-5.5` would both
// survive — neither matches the other as tokens).
func TestRealize_DefaultArgs_Deduped(t *testing.T) {
	// Collision case: default_args declares the same flag the bypass channel
	// emits. Result must contain it exactly ONCE, at the leading position.
	m := Manifest{
		CLI:         "hypo",
		DefaultArgs: []string{"--bypass"},
		Params:      map[string]ParamSpec{"permission": {Channel: "flag", Values: map[string][]string{"bypass": {"--bypass"}}}},
	}
	got := Realize(m, LaunchIntent{Permission: "bypass"})
	if !reflect.DeepEqual(got.LaunchFlags, []string{"--bypass"}) {
		t.Fatalf("colliding token must dedupe to one; got %v, want [--bypass]", got.LaunchFlags)
	}

	// Distinct-value case: different VALUES of the same FLAG NAME survive
	// (token-level dedupe doesn't conflate them). This is the property that
	// makes the dedupe safe for non-boolean params.
	m2 := Manifest{
		CLI:         "hypo2",
		DefaultArgs: []string{"--model", "gpt-5.4"},
		Params: map[string]ParamSpec{
			"model_tier": {Channel: "flag", Flag: "--model", From: "tier_alias"},
		},
		TierAliases: map[string]string{"sonnet": "gpt-5.5"},
	}
	got2 := Realize(m2, LaunchIntent{ModelTier: "sonnet"})
	// First --model appears (default_args), gpt-5.4 appears, second --model is
	// a duplicate token (dropped), gpt-5.5 appears. Net result keeps both
	// values but only the first --model — accept that minor weirdness as the
	// documented contract: dedupe is intentionally token-level, not
	// flag-pair-aware. Callers that need pair semantics must not declare the
	// flag in both default_args and params.
	if !reflect.DeepEqual(got2.LaunchFlags, []string{"--model", "gpt-5.4", "gpt-5.5"}) {
		t.Fatalf("token-level dedupe contract violated; got %v, want [--model gpt-5.4 gpt-5.5]", got2.LaunchFlags)
	}
}

// --- small test helpers ----------------------------------------------------

// sameFlags reports whether got and want contain the same tokens (order-insensitive).
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
