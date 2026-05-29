package bridge

// codex_model_clamp.go — cycle-142 incident fix. The codex ModelTierMap
// translates tier "sonnet" → "gpt-5.4", but a ChatGPT/subscription codex
// account 400-rejects gpt-5.4 (it is effectively API-key-only by plan tier)
// and pops a "Switch to gpt-5.4-mini?" modal that the auto-responder does not
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

// clampCodexModelForAuth returns flags with the `-m <model>` value rewritten to
// m.ChatGPTDefaultModel when authMode=="chatgpt" and the realized model is not
// in m.ChatGPTSafeModels. from/to report the substitution for logging ("","" =
// no clamp). The input slice is never mutated (a fresh slice is returned on a
// clamp). No-ops when: auth is api-key, the manifest declares no safe set, the
// model is already safe, or no -m flag is present.
func clampCodexModelForAuth(flags []string, m Manifest, authMode string) (out []string, from, to string) {
	if authMode != "chatgpt" || len(m.ChatGPTSafeModels) == 0 || m.ChatGPTDefaultModel == "" {
		return flags, "", ""
	}
	idx := modelFlagIndex(flags)
	if idx < 0 {
		return flags, "", ""
	}
	current := flags[idx]
	for _, safe := range m.ChatGPTSafeModels {
		if current == safe {
			return flags, "", "" // already ChatGPT-safe
		}
	}
	clamped := make([]string, len(flags))
	copy(clamped, flags)
	clamped[idx] = m.ChatGPTDefaultModel
	return clamped, current, m.ChatGPTDefaultModel
}

// modelFlagIndex returns the index of the value following the first "-m" (or
// "--model") flag in flags, or -1 when absent or trailing with no value.
func modelFlagIndex(flags []string) int {
	for i, f := range flags {
		if f == "-m" || f == "--model" {
			if i+1 < len(flags) {
				return i + 1
			}
			return -1
		}
	}
	return -1
}
