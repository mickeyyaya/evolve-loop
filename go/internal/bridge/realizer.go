package bridge

import (
	"maps"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

// ParamSpec is the declarative realization of one high-level intent parameter for one CLI (a manifest `params.<name>` entry).
type ParamSpec struct {
	Channel  string              `json:"channel"`            // flag | repl | controller | noop
	Flag     string              `json:"flag,omitempty"`     // flag name for a dynamic value (model_tier) or a multi-value flag (allowed_tools)
	From     string              `json:"from,omitempty"`     // "model_tier_map" (canonical) | "tier_alias" (deprecated) → resolve via Manifest.ModelTierMap
	Template string              `json:"template,omitempty"` // repl: "/model {alias}"
	Values   map[string][]string `json:"values,omitempty"`   // enum intent value → flag tokens
	// Default is the value realized when the intent leaves this param empty —
	// data-driven, per CLI.
	Default string `json:"default,omitempty"`
}

var unresolvedModelTokens = append([]string{"auto", "high"}, modelcatalog.CanonicalTiers...)

// isUnresolvedModelToken reports whether a resolved model value is
// vocabulary rather than a concrete model id; every builder of a model
// argument — the realizer's channels and each headless driver's own argv —
// consults this one predicate.
func isUnresolvedModelToken(resolved string) bool {
	return slices.Contains(unresolvedModelTokens, resolved)
}

func permissionIntent(permissionMode string) string {
	switch permissionMode {
	case "", "bypassPermissions":
		return "bypass"
	default:
		return permissionMode
	}
}

// RealizeFor loads the embedded manifest for cli and realizes intent against
// it; a missing or unreadable manifest realizes to an empty Realization
// rather than aborting the launch. An empty Realization is indistinguishable
// from "manifest missing," so the caller must validate the CLI (via the
// driver registry or LoadManifest) before trusting an empty result as "no
// flags needed."
func RealizeFor(cli string, intent LaunchIntent) Realization {
	m, err := LoadManifest(cli)
	if err != nil {
		return Realization{}
	}
	return Realize(m, intent)
}

// Realize maps a LaunchIntent onto a CLI's Realization using m.Params: an
// intent field whose param is absent from the manifest, or marked noop,
// emits nothing rather than aborting the launch.
func Realize(m Manifest, intent LaunchIntent) Realization {
	var r Realization

	// Manifest-level default_args land FIRST so per-param flags and the raw
	// escape-hatch flags append after them — the "always-on" hook a CLI uses
	// for unconditional launch flags (e.g. codex-tmux's --yolo, which
	// short-circuits the per-edit-approval modal).
	if len(m.DefaultArgs) > 0 {
		r.LaunchFlags = append(r.LaunchFlags, m.DefaultArgs...)
	}
	r.Env = maps.Clone(m.DefaultEnv)

	realizeScalar(&r, m, "model_tier", intent.ModelTier)
	realizeScalar(&r, m, "permission", intent.Permission)
	realizeScalar(&r, m, "settings_scope", intent.SettingsScope)
	realizeScalar(&r, m, "effort", intent.Effort)
	realizeSystemPromptFile(&r, m, intent.SystemPromptFile)

	realizeSessionMode(&r, m, intent.SessionMode)

	// allowed_tools → a multi-value flag: the flag once, then every tool
	// (claude's `--allowedTools Read Write`).
	if spec, ok := m.Params["allowed_tools"]; ok && spec.Channel == "flag" && spec.Flag != "" && len(intent.AllowedTools) > 0 {
		r.LaunchFlags = append(r.LaunchFlags, spec.Flag)
		r.LaunchFlags = append(r.LaunchFlags, intent.AllowedTools...)
	}

	// trustedFlags keeps the manifest-realized prefix separate so selector
	// provenance can be derived from the exact final argv.
	trustedFlags := dedupeLaunchFlags(r.LaunchFlags)
	combinedFlags := append([]string(nil), r.LaunchFlags...)

	// Raw escape hatch: only the matching CLI's flags.
	if raw, ok := intent.RawByCLI[m.CLI]; ok {
		combinedFlags = append(combinedFlags, raw...)
	}
	// A manifest's default_args may declare a flag that one of its params ALSO
	// emits for a particular intent value (e.g. agy-tmux declares
	// --dangerously-skip-permissions in default_args AND in
	// params.permission.values.bypass); dedupe is order-preserving so the
	// operator-declared default keeps the leading position.
	r.LaunchFlags = dedupeLaunchFlags(combinedFlags)
	r.modelDispatchEffect = modelDispatchFromFinalizedFlags(m.CLI, trustedFlags, r.LaunchFlags)
	return r
}

func realizeSessionMode(r *Realization, m Manifest, mode string) {
	spec, ok := m.Params["session_mode"]
	if !ok || spec.Channel != "controller" || mode == "" {
		return
	}
	if name, named := strings.CutPrefix(mode, "named:"); named {
		r.SessionName = name
	} else if mode == "ephemeral" {
		r.Ephemeral = true
	}
}

func realizeSystemPromptFile(r *Realization, m Manifest, path string) {
	spec, ok := m.Params["system_prompt_file"]
	if !ok || spec.Channel != "flag" || spec.Flag == "" || path == "" {
		return
	}
	r.LaunchFlags = append(r.LaunchFlags, spec.Flag, path)
	r.SystemPromptFile = path
}

// dedupeLaunchFlags returns a copy of in with subsequent duplicate units
// removed, preserving order. A unit is a flag-value pair when a `-`-prefixed
// token is followed by a token that doesn't start with `-` (`-c key=val`,
// `-m model`), otherwise the single token (`--yolo`); a flag repeated with
// distinct values is kept in full.
func dedupeLaunchFlags(in []string) []string {
	if len(in) <= 1 {
		return in
	}
	seen := make(map[string]struct{}, len(in))
	// Fresh backing array: `out := in[:0]` would alias in's storage, but the
	// contract is "returns a copy" — callers rely on `in` staying unmodified.
	out := make([]string, 0, len(in))
	for i := 0; i < len(in); i++ {
		tok := in[i]
		// NUL joins the pair key so a value containing the separator cannot
		// forge a collision with a different flag/value split.
		unit, paired := tok, false
		// An empty next token is manifest noise (a typo or a stray element), not a
		// flag's value: pairing with it would let two different flags with empty
		// values collide on the bare "" key, so empty tokens stay standalone.
		if strings.HasPrefix(tok, "-") && i+1 < len(in) && in[i+1] != "" && !strings.HasPrefix(in[i+1], "-") {
			unit, paired = tok+"\x00"+in[i+1], true
		}
		if _, dup := seen[unit]; dup {
			if paired {
				i++ // drop the pair's value alongside its flag
			}
			continue
		}
		seen[unit] = struct{}{}
		out = append(out, tok)
		if paired {
			out = append(out, in[i+1])
			i++
		}
	}
	return out
}

// legacyTierAlias translates the deprecated Anthropic-named tier vocabulary
// (haiku/sonnet/opus) into the canonical vocabulary (fast/balanced/deep).
// Pass-through for already-canonical names and for raw model identifiers.
// Delegates to manifest.translateV1TierKey so the mapping has a single
// source of truth.
// See ADR-0022.
func legacyTierAlias(value string) string {
	return translateV1TierKey(value)
}

// realizeScalar handles a single-valued intent param (model_tier,
// permission, settings_scope); a missing entry, empty value, or unmapped
// enum value emits nothing.
func realizeScalar(r *Realization, m Manifest, param, value string) {
	spec, ok := m.Params[param]
	if !ok {
		return
	}
	if value == "" {
		value = spec.Default // manifest-declared default; still may be empty
	}
	if value == "" {
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
	// through: "model_tier_map" is canonical; the legacy "tier_alias"
	// spelling is accepted unchanged so operator-installed v1 override
	// manifests keep working.
	if spec.From == "model_tier_map" || spec.From == "tier_alias" {
		resolved = resolveTierModel(m, value)
	}
	// A vocabulary token here means model_tier_map translation fell through, so
	// the value names no model on any CLI — emitting `<cli> --model <token>`
	// would boot-fail. Omit the param instead so the CLI's own default wins.
	// This is the emit point for every flag/repl CLI; the headless drivers guard
	// their own argv against the same vocabulary (claudePArgs, driver_codex.go).
	// See ADR-0044.
	if param == "model_tier" && isUnresolvedModelToken(resolved) {
		// Record the suppression so the driver's launch line can report the
		// model the CLI will actually run under, rather than silently logging
		// the requested tier while it degrades to the account default.
		r.ModelOmitted = resolved
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

// resolveTierModel is the one tier→model ladder for every transport: the
// realizer's flag/repl emit and the headless codex driver's own -m
// composition both resolve through it. It tries the raw intent value first
// (synthetic fixtures and operator v1 manifests still keyed by
// haiku/sonnet/opus), then the canonical translation (fast/balanced/deep);
// native ids and unknown values pass through unchanged for the vocabulary
// guard at each emit point to decide.
func resolveTierModel(m Manifest, value string) string {
	if alias, found := m.ModelTierMap[value]; found && alias != "" {
		return alias
	}
	if canonical := legacyTierAlias(value); canonical != value {
		if alias, found := m.ModelTierMap[canonical]; found && alias != "" {
			return alias
		}
	}
	return value
}
