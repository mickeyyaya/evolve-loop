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
//     resolve the value through the manifest's ModelTierMap before emitting.
type ParamSpec struct {
	Channel  string              `json:"channel"`            // flag | repl | controller | noop
	Flag     string              `json:"flag,omitempty"`     // flag name for a dynamic value (model_tier) or a multi-value flag (allowed_tools)
	From     string              `json:"from,omitempty"`     // "model_tier_map" (canonical) | "tier_alias" (deprecated) → resolve via Manifest.ModelTierMap
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

	// Manifest-level default_args land FIRST so per-param flags + raw
	// escape-hatch flags append after them. This is the "always-on" hook
	// each CLI uses for unconditional launch flags (e.g. codex-tmux's
	// --yolo to short-circuit the per-edit-approval modal that stalled
	// cycle-123 tdd — see docs/incidents/cycle-123-codex-edit-approval-
	// modal-and-empty-fallback-chain.md G1a). The field has existed on
	// Manifest since manifest.go:63 but was previously unread; wired in
	// cycle-124 Fix G1a.
	if len(m.DefaultArgs) > 0 {
		r.LaunchFlags = append(r.LaunchFlags, m.DefaultArgs...)
	}

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
	// Dedupe LaunchFlags (cycle-124 G1a wire-up consequence): a manifest's
	// default_args may declare a flag that one of its params ALSO emits when
	// a particular intent value is set (e.g. agy-tmux declares
	// --dangerously-skip-permissions in default_args AND in
	// params.permission.values.bypass; both fire under
	// intent.Permission="bypass"). Dedupe is order-preserving (keep first
	// occurrence) so the operator-declared default still takes the leading
	// position. Idempotent for the already-unique case.
	r.LaunchFlags = dedupeLaunchFlags(r.LaunchFlags)
	return r
}

// dedupeLaunchFlags returns a copy of in with subsequent duplicates removed,
// preserving order. Treats each token as an independent unit — a flag with
// distinct values (e.g. -m gpt-5.4 vs -m gpt-5.5) is correctly kept twice
// because the token values differ. Use ONLY for boolean-style flags
// (--yolo, --dangerously-skip-permissions) where a duplicate is purely
// redundant; flag-value pairs that legitimately repeat (e.g. multiple
// --include patterns) should NOT be deduped this way. The current callers
// from Realize emit boolean flags or unique flag/value pairs, so this
// matches the contract.
func dedupeLaunchFlags(in []string) []string {
	if len(in) <= 1 {
		return in
	}
	seen := make(map[string]struct{}, len(in))
	// Fresh backing array (cycle-124 review HIGH): `out := in[:0]` would
	// alias `in`'s storage. The function's contract is "returns a copy"
	// and the call site relies on that — keeping `in` intact lets the
	// caller hold a pre-dedupe reference for diagnostics without
	// witnessing in-place writes through it.
	out := make([]string, 0, len(in))
	for _, tok := range in {
		if _, dup := seen[tok]; dup {
			continue
		}
		seen[tok] = struct{}{}
		out = append(out, tok)
	}
	return out
}

// legacyTierAlias translates the deprecated Anthropic-named tier vocabulary
// (haiku/sonnet/opus) into the canonical abstract vocabulary (fast/balanced/
// deep) used by ModelTierMap keys after the cycle-124 schema migration.
// Pass-through for already-canonical names AND for raw model identifiers
// (which fall through to the realizer's identity-fallback at the call site).
// Delegates to manifest.translateV1TierKey so the 3-entry mapping has a
// single source of truth (avoids silent drift if a fourth legacy alias is
// ever added — see ADR-0022 PR 2 addendum).
func legacyTierAlias(value string) string {
	return translateV1TierKey(value)
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
	// ParamSpec.From identifies the manifest sidecar table to translate
	// through. "model_tier_map" is canonical (cycle-124 followup); the
	// legacy spelling "tier_alias" is accepted unchanged for one release
	// so operator-installed v1 override manifests keep working.
	if spec.From == "model_tier_map" || spec.From == "tier_alias" {
		// Fallback ladder for the cycle-124 deprecation window: try the raw
		// intent value first (handles synthetic test fixtures + operator
		// v1 manifests where keys are still haiku/sonnet/opus). If that
		// misses, try the canonical translation (handles parseManifest's
		// v1-shimmed manifests where keys are now fast/balanced/deep). Both
		// surfaces remove together one release after the migration.
		if alias, found := m.ModelTierMap[value]; found && alias != "" {
			resolved = alias
		} else if canonical := legacyTierAlias(value); canonical != value {
			if alias, found := m.ModelTierMap[canonical]; found && alias != "" {
				resolved = alias
			}
		}
	}
	// ModelFlagPolicy (ADR-0044 C2 / D3): "auto" is the loop's resolve-me
	// sentinel, never a valid concrete model for ANY CLI. When resolution
	// leaves the sentinel intact (cycle-262: retro was dispatched with no
	// concrete model assigned, and `claude --model auto` boots into the fatal
	// "There's an issue with the selected model (auto)" pane), omit the model
	// param entirely — the CLI's own default model is always preferable to a
	// fatal boot. The realizer is the single emit point for every flag/repl
	// CLI, so this one guard is matrix-wide; the headless codex driver keeps
	// its own equivalent (driver_codex.go omit-on-auto) for the exec path.
	if param == "model_tier" && resolved == "auto" {
		return
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
