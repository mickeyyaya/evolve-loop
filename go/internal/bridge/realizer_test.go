package bridge

import (
	"reflect"
	"testing"
)

// realizer_test.go — the heart of ADR-0022: ONE CLI-agnostic LaunchIntent must
// realize correctly AND differently per CLI, via each manifest's declarative
// `params` table. The same intent that yields `--dangerously-skip-permissions
// --model sonnet --setting-sources project` for claude must yield
// `--dangerously-skip-permissions --model "Gemini 3.5 Flash (High)"` for agy
// (display-name model tokens, no settings flag; cycle-447) and `-m gpt-5.4`
// for codex — and NEVER a flag the target CLI does not define. An intent with
// no manifest entry is a no-op (the property that makes foreign params unable
// to break a launch).

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
		ModelTierMap: map[string]string{"haiku": "gpt-5.4-mini", "sonnet": "gpt-5.4", "opus": "gpt-5.5"},
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
		// Mirrors the real manifest post-cycle-447: agy 1.0.15 grew a --model
		// launch flag whose selectable tokens are display names (spaces/parens).
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

	t.Run("agy-tmux: permission flag + --model display-name token (cycle-447)", func(t *testing.T) {
		got := Realize(agyTmuxManifest(), intent)
		// intent tier "sonnet" → legacy ladder → "balanced" → display name.
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

// TestRealize_REPLChannel covers the post-boot REPL-injection channel — a
// supported engine capability for CLIs whose model can only be set in-session.
// No production manifest uses it today (every tmux CLI's model is a launch flag
// or a no-op), so it's pinned here with a synthetic manifest so the channel
// stays covered and documented as reserved.
func TestRealize_REPLChannel(t *testing.T) {
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
		ModelTierMap: map[string]string{"sonnet": "gpt-5.5"},
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

// TestDedupeLaunchFlags_Edges directly exercises the helper that the cycle-124
// HIGH review-finding had us re-write with a fresh backing slice. The Realize
// integration tests above exercise the helper indirectly; this table drives
// the pure function across every plausible input shape so future refactors
// surface here first. Order-preservation (first occurrence wins) is the
// load-bearing contract — flags-first reflects the operator-declared default,
// per-param scalars deduplicate against it, and raw extras come last.
func TestDedupeLaunchFlags_Edges(t *testing.T) {
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
			"flag-value pair with DISTINCT values → flag dedup, both values kept (documented footgun)",
			[]string{"--model", "x", "--model", "y"},
			[]string{"--model", "x", "y"},
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
			// Compare as slices; an "empty == nil" wash matches Realize's call
			// site behavior (it iterates either to the same empty result).
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("dedupeLaunchFlags(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestDedupeLaunchFlags_AliasSafety pins the cycle-124 review HIGH fix:
// dedupeLaunchFlags MUST return a slice that does not share backing storage
// with its input. Pre-fix `out := in[:0]` would alias, so a caller holding
// the original could see the deduped result through their reference. The
// fix `out := make([]string, 0, len(in))` allocates fresh. This test
// captures the invariant so a future "optimization" can't silently
// regress it.
func TestDedupeLaunchFlags_AliasSafety(t *testing.T) {
	in := []string{"--a", "--b", "--a", "--c"}
	original := append([]string(nil), in...) // snapshot before
	got := dedupeLaunchFlags(in)

	// Mutating the returned slice must NOT mutate the input.
	if len(got) > 0 {
		got[0] = "--MUTATED"
	}
	if !reflect.DeepEqual(in, original) {
		t.Fatalf("dedupe result aliases input backing array; in mutated to %v (was %v)", in, original)
	}

	// And vice versa — mutating the input must NOT mutate the returned slice.
	in[0] = "--TAMPERED"
	if got[0] == "--TAMPERED" {
		t.Fatalf("input mutation leaked into dedupe result; got[0]=%q", got[0])
	}
}

// TestRealize_DefaultArgs_StackInteraction pins the FULL ordering invariant
// across all 4 launch-flag sources Realize composes:
//
//	[default_args] + [model_tier] + [permission] + [settings_scope]
//	  + [allowed_tools] + [raw_by_cli]   → dedupe → r.LaunchFlags
//
// Default_args land FIRST, per-param scalars next (in the realizer's
// internal order), raw escape-hatch flags last. This is the documented
// contract for a single CLI; cross-CLI tests live in realizer_realmanifest_test.go.
func TestRealize_DefaultArgs_StackInteraction(t *testing.T) {
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

	// Exact ordering: default_args FIRST, then each per-param scalar in the
	// internal realize order (model → permission → settings → allowed_tools),
	// then raw extras at the tail.
	want := []string{
		"--default-1", "--default-2", // default_args
		"--model", "model-x", // model_tier (tier_alias resolved)
		"--bypass",           // permission bypass
		"--scope", "project", // settings_scope
		"--raw-tail", // raw_by_cli (last)
	}
	if !reflect.DeepEqual(got.LaunchFlags, want) {
		t.Fatalf("stack order broken;\ngot:  %v\nwant: %v", got.LaunchFlags, want)
	}
}

// TestRealize_DefaultArgs_DegenerateManifest covers manifests with NO params
// table — default_args still fires. This protects the "minimum-viable
// manifest" path: a brand-new driver with only DefaultArgs and a Binary can
// still emit boot flags before its param table is fleshed out.
func TestRealize_DefaultArgs_DegenerateManifest(t *testing.T) {
	m := Manifest{CLI: "bare", DefaultArgs: []string{"--only-this"}}
	got := Realize(m, LaunchIntent{ModelTier: "sonnet", Permission: "bypass"})
	if !reflect.DeepEqual(got.LaunchFlags, []string{"--only-this"}) {
		t.Fatalf("default_args must fire on degenerate manifest; got %v", got.LaunchFlags)
	}
	if len(got.REPLInput) != 0 {
		t.Fatalf("degenerate manifest must not emit REPL input; got %v", got.REPLInput)
	}
}

// TestRealize_DefaultArgs_InternalDuplicates pins that dedupe handles
// repeats WITHIN default_args itself, not just collisions WITH the params
// channel. A typo'd manifest `["--yolo", "--yolo"]` shouldn't double-emit.
func TestRealize_DefaultArgs_InternalDuplicates(t *testing.T) {
	m := Manifest{
		CLI:         "dup-default",
		DefaultArgs: []string{"--yolo", "--yolo", "--yolo"},
	}
	got := Realize(m, LaunchIntent{})
	if !reflect.DeepEqual(got.LaunchFlags, []string{"--yolo"}) {
		t.Fatalf("internal default_args duplicates must collapse; got %v", got.LaunchFlags)
	}
}

// TestRealize_DefaultArgs_OrthogonalToREPLChannel ensures default_args
// does NOT leak into REPLInput (the post-boot injection channel). REPL
// channel params write to REPLInput; default_args writes to LaunchFlags.
// They must remain orthogonal so a manifest can use both safely.
func TestRealize_DefaultArgs_OrthogonalToREPLChannel(t *testing.T) {
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

// TestRealize_DefaultArgs_EmptyTokenDefensive covers a typo'd manifest
// declaring default_args with an empty string in the list. The dedupe
// keeps one empty (defensive) so the realizer doesn't crash, but the
// downstream launch helpers (driver_tmux_repl.go launchCmdLine,
// driver_ollamatmux.go ollamaComposeLaunchCmd) treat empty tokens as
// no-ops. This pins that contract at the realizer level.
func TestRealize_DefaultArgs_EmptyTokenDefensive(t *testing.T) {
	m := Manifest{CLI: "typo", DefaultArgs: []string{"", "--real", ""}}
	got := Realize(m, LaunchIntent{})
	// Dedupe collapses the two empties to one. The downstream join paths
	// drop empties — the realizer's job is just to not crash on them.
	if len(got.LaunchFlags) != 2 || got.LaunchFlags[1] != "--real" {
		t.Fatalf("empty + real tokens; got %v (want a 2-elem slice with --real)", got.LaunchFlags)
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
