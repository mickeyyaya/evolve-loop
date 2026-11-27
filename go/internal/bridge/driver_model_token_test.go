package bridge

import (
	"strings"
	"testing"
)

func TestClaudePRealization_OmitsUnresolvedModelToken(t *testing.T) {
	t.Parallel()
	for _, tok := range unresolvedModelTokens {
		t.Run(tok, func(t *testing.T) {
			r := RealizeFor("claude-p", LaunchIntent{ModelTier: tok})
			args := claudePArgs(&Config{Model: tok, Realization: r}, "prompt")
			if tok != "auto" {
				if !containsToken(args, "--model") {
					t.Errorf("tier %q must map to a real model through model_tier_map; got %v", tok, args)
				}
				return
			}
			if containsToken(args, "--model") {
				t.Errorf("token %q reached the CLI as a model: %v", tok, args)
			}
			if r.ModelOmitted != tok {
				t.Errorf("ModelOmitted = %q, want %q", r.ModelOmitted, tok)
			}
		})
	}
}

func TestClaudePRealization_TierTokenMapsToARealModel(t *testing.T) {
	t.Parallel()
	for tier, want := range map[string]string{"fast": "haiku", "balanced": "sonnet", "deep": "opus", "top": "opus"} {
		r := RealizeFor("claude-p", LaunchIntent{ModelTier: tier})
		if !containsRun(r.LaunchFlags, []string{"--model", want}) {
			t.Errorf("tier %q realized to %v, want --model %s", tier, r.LaunchFlags, want)
		}
	}
}

func TestClaudePRealization_EmptyModelSendsNoFlag(t *testing.T) {
	t.Parallel()
	r := RealizeFor("claude-p", LaunchIntent{})
	if containsToken(r.LaunchFlags, "--model") || r.ModelOmitted != "" {
		t.Errorf("empty model realized to %v omitted=%q, want no flag", r.LaunchFlags, r.ModelOmitted)
	}
}

func TestClaudePArgs_PreservesNonModelArgv(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		AllowedTools: []string{"Read", "Grep"},
		ExtraFlags:   []string{"--bare"},
		Realization:  RealizeFor("claude-p", LaunchIntent{Permission: "plan", SettingsScope: "project"}),
	}
	args := claudePArgs(cfg, "the prompt")
	for _, want := range []string{"-p", "the prompt", "--permission-mode", "plan", "--allowedTools", "Read", "--setting-sources", "--bare"} {
		if !containsToken(args, want) {
			t.Errorf("claudePArgs dropped %q; got %v", want, args)
		}
	}
	if got := strings.Join(args[:2], " "); got != "-p the prompt" {
		t.Errorf("prompt must stay in the leading -p position, got %q", got)
	}
}

func TestUnresolvedModelTokens_CoversVocabularyWithoutSwallowingRealModels(t *testing.T) {
	t.Parallel()
	for _, tok := range []string{"auto", "high", "fast", "balanced", "deep", "top"} {
		if !isUnresolvedModelToken(tok) {
			t.Errorf("isUnresolvedModelToken(%q) = false — %q is vocabulary and must never reach a CLI as a model", tok, tok)
		}
	}
	for _, model := range []string{"haiku", "sonnet", "opus", "claude-opus-5", "gpt-5.5", "Gemini 3.1 Pro (High)", "qwen3:30b"} {
		if isUnresolvedModelToken(model) {
			t.Errorf("isUnresolvedModelToken(%q) = true — suppressing a real model id disables model routing entirely for that CLI", model)
		}
	}
}
