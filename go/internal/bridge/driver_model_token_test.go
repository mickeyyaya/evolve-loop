package bridge

import (
	"strings"
	"testing"
)

// Sweeps the single vocabulary source (unresolvedModelTokens) rather than a
// local copy, so a token added there is automatically covered here.
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

func TestClaudePArgs_EmitsConcreteModel(t *testing.T) {
	args, omitted := claudePArgs(&Config{Model: "opus"}, "prompt")
	if !containsToken(args, "--model") || !containsToken(args, "opus") {
		t.Errorf("claudePArgs(model=opus) = %v, want --model opus — a concrete model must still reach the CLI", args)
	}
	if omitted != "" {
		t.Errorf("claudePArgs(model=opus) reported omitted=%q, want empty", omitted)
	}
}

func TestClaudePArgs_EmptyModelOmitsWithoutClaimingSuppression(t *testing.T) {
	args, omitted := claudePArgs(&Config{Model: ""}, "prompt")
	if containsToken(args, "--model") {
		t.Errorf("claudePArgs(model=\"\") emitted --model in %v — an empty model must send no flag", args)
	}
	if omitted != "" {
		t.Errorf("claudePArgs(model=\"\") reported omitted=%q, want empty", omitted)
	}
}

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

func TestUnresolvedModelTokens_CoversVocabularyWithoutSwallowingRealModels(t *testing.T) {
	for _, tok := range []string{"auto", "high", "fast", "balanced", "deep", "top"} {
		if !isUnresolvedModelToken(tok) {
			t.Errorf("isUnresolvedModelToken(%q) = false — %q is vocabulary and must never reach a CLI as a model", tok, tok)
		}
	}
	// haiku/sonnet/opus are both legacy tier aliases (translateV1TierKey) and
	// real claude models, and must pass through as models here.
	for _, model := range []string{"haiku", "sonnet", "opus", "claude-opus-5", "gpt-5.5", "Gemini 3.1 Pro (High)", "qwen3:30b"} {
		if isUnresolvedModelToken(model) {
			t.Errorf("isUnresolvedModelToken(%q) = true — suppressing a real model id disables model routing entirely for that CLI", model)
		}
	}
}
