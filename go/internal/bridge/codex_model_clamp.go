package bridge

// codexAuthMode is "api-key" only when OPENAI_API_KEY is set and BRIDGE_ALLOW_OPENAI_API_KEY=1, else "chatgpt".
// The credential-isolation guard already demands that opt-in for any key, so the env alone decides.
func codexAuthMode(deps Deps) string {
	if v, ok := lookupEnv(deps, "OPENAI_API_KEY"); ok && v != "" {
		if allow, _ := lookupEnv(deps, "BRIDGE_ALLOW_OPENAI_API_KEY"); allow == "1" {
			return "api-key"
		}
	}
	return "chatgpt"
}

// clampCodexModelForAuth rewrites the effective model selector to m.ChatGPTDefaultModel when a chatgpt-auth launch
// asks for a model outside m.ChatGPTSafeModels, which a subscription account rejects. Dedicated flags beat -c
// overrides. from and to report the substitution ("" when none); flags is never mutated.
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
			return flags, "", ""
		}
	}
	clamped := make([]string, len(flags))
	copy(clamped, flags)
	clamped[selector.argIndex] = selector.replacementPrefix + m.ChatGPTDefaultModel
	return clamped, current, m.ChatGPTDefaultModel
}
