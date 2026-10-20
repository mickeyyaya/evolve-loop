package bridge

import (
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type ParamSpec struct {
	Channel  string              `json:"channel"`
	Flag     string              `json:"flag,omitempty"`
	From     string              `json:"from,omitempty"`
	Template string              `json:"template,omitempty"`
	Values   map[string][]string `json:"values,omitempty"`
	Default  string              `json:"default,omitempty"`
}

var unresolvedModelTokens = append([]string{"auto", "high"}, modelcatalog.CanonicalTiers...)

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

func RealizeFor(cli string, intent LaunchIntent) Realization {
	m, err := LoadManifest(cli)
	if err != nil {
		return Realization{}
	}
	return Realize(m, intent)
}

func Realize(m Manifest, intent LaunchIntent) Realization {
	var r Realization

	if len(m.DefaultArgs) > 0 {
		r.LaunchFlags = append(r.LaunchFlags, m.DefaultArgs...)
	}
	r.Env = maps.Clone(m.DefaultEnv)

	realizeScalar(&r, m, "model_tier", intent.ModelTier)
	realizeScalar(&r, m, "permission", intent.Permission)
	realizeScalar(&r, m, "settings_scope", intent.SettingsScope)
	realizeEffort(&r, m, intent.Effort)
	capLadderEffort(&r, m, intent.LadderEffort)
	realizeSystemPromptFile(&r, m, intent.SystemPromptFile)

	realizeSessionMode(&r, m, intent.SessionMode)

	if spec, ok := m.Params["allowed_tools"]; ok && spec.Channel == "flag" && spec.Flag != "" && len(intent.AllowedTools) > 0 {
		r.LaunchFlags = append(r.LaunchFlags, spec.Flag)
		r.LaunchFlags = append(r.LaunchFlags, intent.AllowedTools...)
	}

	trustedFlags := dedupeLaunchFlags(r.LaunchFlags)
	combinedFlags := append([]string(nil), r.LaunchFlags...)

	if raw, ok := intent.RawByCLI[m.CLI]; ok {
		combinedFlags = append(combinedFlags, raw...)
	}
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

const flagValueKeySeparator = "\x00"

func dedupeLaunchFlags(in []string) []string {
	if len(in) <= 1 {
		return in
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for i := 0; i < len(in); i++ {
		tok := in[i]
		unit, paired := tok, false
		if strings.HasPrefix(tok, "-") && i+1 < len(in) && in[i+1] != "" && !strings.HasPrefix(in[i+1], "-") {
			unit, paired = tok+flagValueKeySeparator+in[i+1], true
		}
		if _, dup := seen[unit]; dup {
			if paired {
				i++
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

func legacyTierAlias(value string) string {
	return translateV1TierKey(value)
}

func realizeScalar(r *Realization, m Manifest, param, value string) {
	spec, ok := m.Params[param]
	if !ok {
		return
	}
	if value == "" {
		value = spec.Default
	}
	if value == "" {
		return
	}
	if len(spec.Values) > 0 {
		if toks, found := spec.Values[value]; found {
			r.LaunchFlags = append(r.LaunchFlags, toks...)
		}
		return
	}
	resolved := value
	if spec.From == "model_tier_map" || spec.From == "tier_alias" {
		resolved = resolveTierModel(m, value)
	}
	if param == "model_tier" && isUnresolvedModelToken(resolved) {
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
}

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

const (
	effortChannelModelVariant = "model_variant"
	effortChannelNoop         = "noop"
)

var modelVariantRE = regexp.MustCompile(`\([A-Za-z]+\)$`)

func modelVariantSuffix(toks []string) string {
	if len(toks) != 1 || toks[0] == "" {
		return ""
	}
	return "(" + toks[0] + ")"
}

func realizeEffort(r *Realization, m Manifest, effort string) {
	if m.Params["effort"].Channel != effortChannelModelVariant {
		realizeScalar(r, m, "effort", effort)
		return
	}
	realizeModelVariant(r, m, effort)
}

func realizeModelVariant(r *Realization, m Manifest, effort string) {
	toks := m.Params["effort"].Values[effort]
	suffix := modelVariantSuffix(toks)
	i := slices.Index(r.LaunchFlags, m.Params["model_tier"].Flag)
	if suffix == "" || i < 0 || i+1 >= len(r.LaunchFlags) {
		return
	}
	loc := modelVariantRE.FindStringIndex(r.LaunchFlags[i+1])
	if loc == nil {
		r.EffortUnapplied = effort
		return
	}
	flags := slices.Clone(r.LaunchFlags)
	flags[i+1] = flags[i+1][:loc[0]] + suffix
	r.LaunchFlags = flags
	r.EffortVariant = suffix
	if !strings.EqualFold(effort, toks[0]) {
		r.EffortCapped = effort
	}
}

func capLadderEffort(r *Realization, m Manifest, requested string) {
	if requested == "" || m.Params["effort"].Channel != effortChannelNoop {
		return
	}
	i := slices.Index(r.LaunchFlags, m.Params["model_tier"].Flag)
	if i < 0 || i+1 >= len(r.LaunchFlags) {
		return
	}
	variant := modelVariantRE.FindString(r.LaunchFlags[i+1])
	granted := strings.ToLower(strings.Trim(variant, "()"))
	levels := policy.EffortLevels()
	if g := slices.Index(levels, granted); g >= 0 && slices.Index(levels, requested) > g {
		r.EffortCapped, r.EffortVariant = requested, variant
	}
}
