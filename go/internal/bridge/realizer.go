package bridge

import "strings"

// realizer.go — the Go engine half of the hybrid Realizer (ADR-0022). The
// per-CLI mapping data lives declaratively in each manifest's `params` table;
// this engine interprets it. Flags-first: an intent realizes to a launch flag
// when the CLI declares one, to REPL injection when declared `repl`, to a
// controller hint for session lifecycle, or to nothing (no entry / `noop`).

// ParamSpec is the declarative realization of ONE high-level intent parameter
// for ONE CLI (a manifest `params.<name>` entry).
//
// Two shapes:
//   - Enum-mapped (permission, settings_scope): Values maps each intent value to
//     the concrete flag tokens, e.g. {"bypass": ["--dangerously-skip-permissions"]}.
//   - Dynamic (model_tier): Channel + Flag/Template, with From:"tier_alias" to
//     resolve the value through the manifest's TierAliases before emitting.
type ParamSpec struct {
	Channel  string              `json:"channel"`            // flag | repl | controller | noop
	Flag     string              `json:"flag,omitempty"`     // flag name for a dynamic value (model_tier) or a multi-value flag (allowed_tools)
	From     string              `json:"from,omitempty"`     // "tier_alias" → resolve via Manifest.TierAliases
	Template string              `json:"template,omitempty"` // repl: "/model {alias}"
	Values   map[string][]string `json:"values,omitempty"`   // enum intent value → flag tokens
}

// permissionIntent maps a profile's claude-style permission_mode string onto
// the high-level LaunchIntent.Permission the Realizer understands. An empty
// permission_mode means "bypass" — matching the *-tmux drivers' historical
// default (claude/agy launch with --dangerously-skip-permissions when no mode
// is set). "bypassPermissions" is the explicit spelling of the same posture;
// every other claude mode (plan, acceptEdits, …) passes through verbatim and
// realizes per the CLI's manifest (claude maps bypass+plan; agy bypass only;
// codex none — its trust posture is handled by the auto-responder).
func permissionIntent(permissionMode string) string {
	switch permissionMode {
	case "", "bypassPermissions":
		return "bypass"
	default:
		return permissionMode
	}
}

// RealizeFor loads the embedded manifest for cli and realizes intent against
// it. A missing/unreadable manifest realizes to an empty Realization — the
// same no-op philosophy as an absent param: a launch is never aborted by the
// realizer itself (the driver separately validates the CLI/binary).
//
// CAVEAT for the launch-path wiring (next slice): an empty Realization is
// indistinguishable from "manifest missing" here, so a typo'd CLI name would
// realize to zero flags rather than error. The caller MUST validate the CLI
// (e.g. via the driver registry / LoadManifest) before trusting an empty
// realization — do not infer "no flags needed" from an empty result.
func RealizeFor(cli string, intent LaunchIntent) Realization {
	m, err := LoadManifest(cli)
	if err != nil {
		return Realization{}
	}
	return Realize(m, intent)
}

// Realize maps a LaunchIntent onto a CLI's Realization using m.Params. Any
// intent field whose param is absent from the manifest (or marked noop) emits
// nothing — the property that makes a foreign/unsupported parameter unable to
// abort a launch.
func Realize(m Manifest, intent LaunchIntent) Realization {
	var r Realization

	realizeScalar(&r, m, "model_tier", intent.ModelTier)
	realizeScalar(&r, m, "permission", intent.Permission)
	realizeScalar(&r, m, "settings_scope", intent.SettingsScope)

	// session_mode → controller lifecycle (never a CLI flag for a REPL).
	if spec, ok := m.Params["session_mode"]; ok && spec.Channel == "controller" && intent.SessionMode != "" {
		if name, named := strings.CutPrefix(intent.SessionMode, "named:"); named {
			r.SessionName = name
		} else if intent.SessionMode == "ephemeral" {
			r.Ephemeral = true
		}
	}

	// allowed_tools → a multi-value flag: the flag once, then every tool
	// (claude's `--allowedTools Read Write`).
	if spec, ok := m.Params["allowed_tools"]; ok && spec.Channel == "flag" && spec.Flag != "" && len(intent.AllowedTools) > 0 {
		r.LaunchFlags = append(r.LaunchFlags, spec.Flag)
		r.LaunchFlags = append(r.LaunchFlags, intent.AllowedTools...)
	}

	// Raw escape hatch: only the matching CLI's flags.
	if raw, ok := intent.RawByCLI[m.CLI]; ok {
		r.LaunchFlags = append(r.LaunchFlags, raw...)
	}
	return r
}

// realizeScalar handles a single-valued intent param (model_tier, permission,
// settings_scope). No manifest entry, empty value, or an unmapped enum value
// emits nothing.
func realizeScalar(r *Realization, m Manifest, param, value string) {
	spec, ok := m.Params[param]
	if !ok || value == "" {
		return
	}
	// Enum-mapped: the intent value selects concrete flag tokens.
	if len(spec.Values) > 0 {
		if toks, found := spec.Values[value]; found {
			r.LaunchFlags = append(r.LaunchFlags, toks...)
		}
		return
	}
	// Dynamic: resolve the value (optionally via tier_alias) then emit per channel.
	resolved := value
	if spec.From == "tier_alias" {
		if alias, found := m.TierAliases[value]; found && alias != "" {
			resolved = alias
		}
	}
	switch spec.Channel {
	case "flag":
		if spec.Flag != "" {
			r.LaunchFlags = append(r.LaunchFlags, spec.Flag, resolved)
		}
	case "repl":
		if spec.Template != "" {
			r.REPLInput = append(r.REPLInput, strings.ReplaceAll(spec.Template, "{alias}", resolved))
		}
	}
	// controller / noop / unknown channel → no scalar emission.
}
