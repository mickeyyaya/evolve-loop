package bridge

// driver_model_token_test.go — the unresolved-model-token invariant across EVERY
// driver that builds a model argument, not just the realizer.
//
// realizer_modelpolicy_test.go pins omit-on-"auto" for the tmux/flag matrix
// (ADR-0044 C2/D3, cycle-262). That guard is only matrix-wide for CLIs whose
// model flag the REALIZER emits. The headless drivers build their own argv:
// driver_codex.go has always had its own omit-on-auto, and claude-p had none —
// `claude -p --model <tier>` (and `--model auto`) went straight to the CLI.
//
// These tests drive the pure argv builders, so a driver that reintroduces an
// unguarded model flag fails here without launching anything.

import (
	"strings"
	"testing"
)

// TestClaudePArgs_OmitsUnresolvedModelToken sweeps the SINGLE vocabulary source
// (unresolvedModelTokens) rather than a local copy, so a token added there is
// automatically covered here.
func TestClaudePArgs_OmitsUnresolvedModelToken(t *testing.T) {
	for _, tok := range unresolvedModelTokens {
		t.Run(tok, func(t *testing.T) {
			args, omitted := claudePArgs(&Config{Model: tok}, "prompt")
			if containsToken(args, "--model") {
				t.Errorf("claudePArgs(model=%q) emitted --model %v — %q names a tier, not a model; "+
					"`claude -p --model %s` boots into the fatal \"issue with the selected model\" pane "+
					"(cycle-262 class). The CLI's own default always beats a fatal boot.", tok, args, tok, tok)
			}
			if omitted != tok {
				t.Errorf("claudePArgs(model=%q) reported omitted=%q, want %q — suppression must be reported so "+
					"Launch can log the truth instead of the requested tier", tok, omitted, tok)
			}
		})
	}
}

// TestClaudePArgs_EmitsConcreteModel is the anti-degenerate twin: a builder that
// suppressed everything would pass the test above while disabling model routing.
func TestClaudePArgs_EmitsConcreteModel(t *testing.T) {
	args, omitted := claudePArgs(&Config{Model: "opus"}, "prompt")
	if !containsToken(args, "--model") || !containsToken(args, "opus") {
		t.Errorf("claudePArgs(model=opus) = %v, want --model opus — a concrete model must still reach the CLI", args)
	}
	if omitted != "" {
		t.Errorf("claudePArgs(model=opus) reported omitted=%q, want empty", omitted)
	}
}

// TestClaudePArgs_EmptyModelOmitsWithoutClaimingSuppression (negative): an unset
// model is "nothing requested", not "a request we refused" — reporting it as
// omitted would fill the log with phantom suppressions on every default launch.
func TestClaudePArgs_EmptyModelOmitsWithoutClaimingSuppression(t *testing.T) {
	args, omitted := claudePArgs(&Config{Model: ""}, "prompt")
	if containsToken(args, "--model") {
		t.Errorf("claudePArgs(model=\"\") emitted --model in %v — an empty model must send no flag", args)
	}
	if omitted != "" {
		t.Errorf("claudePArgs(model=\"\") reported omitted=%q, want empty", omitted)
	}
}

// TestClaudePArgs_PreservesNonModelArgv guards the extraction itself: pulling the
// argv build out of Launch must not drop the prompt or the pass-through flags.
func TestClaudePArgs_PreservesNonModelArgv(t *testing.T) {
	cfg := &Config{
		Model:          "opus",
		PermissionMode: "plan",
		AllowedTools:   []string{"Read", "Grep"},
		ExtraFlags:     []string{"--bare"},
		Realization:    Realization{LaunchFlags: []string{"--setting-sources", "project"}},
	}
	args, _ := claudePArgs(cfg, "the prompt")
	for _, want := range []string{"-p", "the prompt", "--permission-mode", "plan", "--allowedTools", "Read", "--setting-sources", "--bare"} {
		if !containsToken(args, want) {
			t.Errorf("claudePArgs dropped %q during extraction; got %v", want, args)
		}
	}
	if got := strings.Join(args[:2], " "); got != "-p the prompt" {
		t.Errorf("prompt must stay in the leading -p position, got %q", got)
	}
}

// TestUnresolvedModelTokens_CoversVocabularyWithoutSwallowingRealModels pins the
// single source itself: it must contain every abstract token AND must NOT
// contain a concrete model id, or the guard would suppress legitimate routing.
func TestUnresolvedModelTokens_CoversVocabularyWithoutSwallowingRealModels(t *testing.T) {
	for _, tok := range []string{"auto", "high", "fast", "balanced", "deep", "top"} {
		if !isUnresolvedModelToken(tok) {
			t.Errorf("isUnresolvedModelToken(%q) = false — %q is vocabulary and must never reach a CLI as a model", tok, tok)
		}
	}
	// Real model ids that translateV1TierKey also maps (haiku/sonnet/opus are
	// BOTH legacy tier aliases and real claude models) must pass through.
	for _, model := range []string{"haiku", "sonnet", "opus", "claude-opus-5", "gpt-5.5", "Gemini 3.1 Pro (High)", "qwen3:30b"} {
		if isUnresolvedModelToken(model) {
			t.Errorf("isUnresolvedModelToken(%q) = true — suppressing a real model id disables model routing entirely for that CLI", model)
		}
	}
}
