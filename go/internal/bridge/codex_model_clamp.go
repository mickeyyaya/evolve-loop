package bridge

// codex_model_clamp.go — cycle-142 incident fix. The codex ModelTierMap
// translates a tier to a model, but a ChatGPT/subscription codex account
// 400-rejects models outside its plan tier (the 2026-06 case was gpt-5.4)
// and pops a "Switch to <fallback>?" modal that the auto-responder does not
// dismiss — stalling the phase for the full artifact-wait window and
// surfacing as a generic ExitArtifactTimeout. The clamp substitutes a
// manifest-declared ChatGPT-safe model on subscription auth; API-key auth is
// left untouched so it can still use the larger models.
//
// Auth mode is determined entirely from the launch env — the codex-tmux
// credential-isolation guard already requires BRIDGE_ALLOW_OPENAI_API_KEY=1
// for any OPENAI_API_KEY, so reaching the clamp with an allowed key means the
// operator explicitly opted into API-key mode; everything else is subscription
// ("Sign in with ChatGPT"), which is this driver's documented default.

// codexAuthMode reports "api-key" only when OPENAI_API_KEY is set AND
// explicitly allowed via BRIDGE_ALLOW_OPENAI_API_KEY=1; otherwise "chatgpt"
// (the subscription default this driver is built around). No filesystem read.
func codexAuthMode(deps Deps) string {
	if v, ok := lookupEnv(deps, "OPENAI_API_KEY"); ok && v != "" {
		if allow, _ := lookupEnv(deps, "BRIDGE_ALLOW_OPENAI_API_KEY"); allow == "1" {
			return "api-key"
		}
	}
	return "chatgpt"
}

// clampCodexModelForAuth returns flags with the effective model-selector token
// rewritten to m.ChatGPTDefaultModel when authMode=="chatgpt" and the realized
// model is not in m.ChatGPTSafeModels. It shares selector grammar with dispatch
// attribution, including split and inline dedicated flags plus -c/--config
// model overrides. Dedicated flags take precedence over config overrides
// regardless of argv order. from/to report the substitution for logging
// ("","" = no clamp). The input slice is never mutated. No-ops when auth is
// api-key, policy is absent, the model is already safe, or no complete selector
// is present.
func clampCodexModelForAuth(flags []string, m Manifest, authMode string) (out []string, from, to string) {
	if authMode != "chatgpt" || len(m.ChatGPTSafeModels) == 0 || m.ChatGPTDefaultModel == "" {
		return flags, "", ""
	}
	selector, ok := effectiveCodexModelSelector(flags)
	if !ok {
		return flags, "", ""
	}
	current := selector.model
	for _, safe := range m.ChatGPTSafeModels {
		if current == safe {
			return flags, "", "" // already ChatGPT-safe
		}
	}
	clamped := make([]string, len(flags))
	copy(clamped, flags)
	clamped[selector.argIndex] = selector.replacementPrefix + m.ChatGPTDefaultModel
	return clamped, current, m.ChatGPTDefaultModel
}
