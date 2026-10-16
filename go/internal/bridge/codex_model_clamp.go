package bridge

func codexAuthMode(deps Deps) string {
	if v, ok := lookupEnv(deps, "OPENAI_API_KEY"); ok && v != "" {
		if allow, _ := lookupEnv(deps, "BRIDGE_ALLOW_OPENAI_API_KEY"); allow == "1" {
			return "api-key"
		}
	}
	return "chatgpt"
}

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
