package setup

import (
	"fmt"
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type Assignment struct {
	Role               string `json:"role"`
	CLI                string `json:"cli"`
	Tier               string `json:"tier"`
	Model              string `json:"model,omitempty"`
	Rationale          string `json:"rationale,omitempty"`
	DiffersFromDefault bool   `json:"differs_from_default"`
	TierClamped        bool   `json:"tier_clamped,omitempty"`
	CLIFallback        bool   `json:"cli_fallback,omitempty"`
	Warning            string `json:"warning,omitempty"`
}

type Preset struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Assignments []Assignment `json:"assignments"`
	Degraded    bool         `json:"degraded"`
}

type RecommendReport struct {
	AvailableFamilies []string `json:"available_families"`
	CrossFamilyOK     bool     `json:"cross_family_ok"`
	Presets           []Preset `json:"presets"`
	Default           string   `json:"default"`
}

func canonTier(s string) string {
	return tierFromRank(policy.TierRank(s))
}

func tierFromRank(r int) string {
	switch r {
	case 1:
		return "fast"
	case 2:
		return "balanced"
	case 3:
		return "deep"
	case 4:
		return "top"
	}
	return ""
}

func Recommend(rep DetectReport, cfg PresetConfig) RecommendReport {
	avail := availableFamilies(rep)
	crossOK := len(avail) >= 2

	cliByBase := make(map[string]CLIStatus, len(rep.CLIs))
	for _, c := range rep.CLIs {
		cliByBase[c.CLI] = c
	}

	builderPS, hasBuilder := phaseByRoleOK(rep, "builder")
	auditorPS, hasAuditor := phaseByRoleOK(rep, "auditor")

	rr := RecommendReport{AvailableFamilies: avail, CrossFamilyOK: crossOK, Default: defaultPresetName(cfg)}
	for _, spec := range cfg.Presets {
		p := Preset{Name: spec.Name, Description: spec.Description}

		bldFam, audFam := "", ""
		if hasBuilder && hasAuditor {
			bldFam, audFam = chooseCrossFamilyPair(builderPS, auditorPS, avail, crossOK)
		}

		for _, ps := range rep.Phases {
			a := assignPhase(spec, ps, bldFam, audFam, avail, cliByBase)
			if a.Warning != "" {
				p.Degraded = true
			}
			p.Assignments = append(p.Assignments, a)
		}
		rr.Presets = append(rr.Presets, p)
	}
	return rr
}

func defaultPresetName(cfg PresetConfig) string {
	def := cfg.Default
	if def == "" && len(cfg.Presets) > 0 {
		def = cfg.Presets[0].Name
	}
	return def
}

func assignPhase(spec PresetSpec, ps PhaseStatus, bldFam, audFam string, avail []string, cliByBase map[string]CLIStatus) Assignment {
	tier, clamped := clampTier(biasTier(spec.TierBias, ps.DefaultTier, ps.Envelope), ps.Envelope)
	prefBase := policy.BaseCLI(ps.DefaultCLI)

	var (
		cli      string
		fallback bool
		warn     string
	)
	switch {
	case ps.Role == "builder" && bldFam != "":
		cli, fallback = bldFam, bldFam != prefBase
	case ps.Role == "auditor" && audFam != "":
		cli, fallback = audFam, audFam != prefBase
	default:
		cli, fallback, warn = chooseCLI(ps.Role, prefBase, ps.AllowedCLIs, avail)
	}

	model := ""
	if cs, ok := cliByBase[cli]; ok && cs.TierModels != nil {
		model = cs.TierModels[tier]
	}

	return Assignment{
		Role: ps.Role, CLI: cli, Tier: tier, Model: model,
		TierClamped: clamped, CLIFallback: fallback, Warning: warn,
		DiffersFromDefault: cli != prefBase || tier != effectiveDefaultTier(ps.DefaultTier, ps.Envelope),
		Rationale:          rationale(spec.Name, cli, tier, clamped, fallback, warn),
	}
}

func effectiveDefaultTier(defaultTier string, env Envelope) string {
	if t := canonTier(defaultTier); t != "" {
		return t
	}
	if t := canonTier(env.Default); t != "" {
		return t
	}
	return "balanced"
}

func biasTier(bias, defaultTier string, env Envelope) string {
	base := effectiveDefaultTier(defaultTier, env)
	switch bias {
	case "down":
		r := policy.TierRank(base)
		if r > 1 {
			r--
		}
		return tierFromRank(r)
	case "up":
		r := policy.TierRank(base)
		if r < 4 {
			r++
		}
		return tierFromRank(r)
	case "min":
		if m := canonTier(env.Min); m != "" {
			return m
		}
		return base
	case "max":
		if m := canonTier(env.Max); m != "" {
			return m
		}
		return base
	default:
		return base
	}
}

func clampTier(want string, env Envelope) (string, bool) {
	if env.Min == "" && env.Max == "" {
		if want == "" {
			return "balanced", true
		}
		return want, false
	}
	rWant := policy.TierRank(want)
	rMin := policy.TierRank(env.Min)
	rMax := policy.TierRank(env.Max)
	if rWant == 0 {
		if d := canonTier(env.Default); d != "" {
			return d, true
		}
		if m := canonTier(env.Min); m != "" {
			return m, true
		}
		return "balanced", true
	}
	if rMin > 0 && rWant < rMin {
		return tierFromRank(rMin), true
	}
	if rMax > 0 && rWant > rMax {
		return tierFromRank(rMax), true
	}
	return want, false
}

func chooseCLI(role, prefBase string, allowed, avail []string) (cli string, fallback bool, warn string) {
	if len(avail) == 0 {
		return prefBase, false, "no CLI families authed; pipeline defaults apply"
	}
	pool := poolFor(allowed, avail)
	if len(pool) == 0 {
		return prefBase, true, fmt.Sprintf(
			"no available allowed CLI for %s (allowed=%v, available=%v); recommendation is advisory",
			role, allowed, avail)
	}
	if inList(pool, prefBase) {
		return prefBase, false, ""
	}
	return pool[0], true, ""
}

func chooseCrossFamilyPair(b, a PhaseStatus, avail []string, crossOK bool) (string, string) {
	bPool := poolFor(b.AllowedCLIs, avail)
	aPool := poolFor(a.AllowedCLIs, avail)
	bFam := pickPreferred(policy.BaseCLI(b.DefaultCLI), bPool)
	aFam := pickPreferred(policy.BaseCLI(a.DefaultCLI), aPool)
	if crossOK && bFam != "" && aFam != "" && bFam == aFam {
		if alt := firstNotEqual(aPool, bFam); alt != "" {
			aFam = alt
		} else if alt := firstNotEqual(bPool, aFam); alt != "" {
			bFam = alt
		}
	}
	return bFam, aFam
}

func availableFamilies(rep DetectReport) []string {
	var out []string
	for _, c := range rep.CLIs {
		if c.Verdict != "blocked" {
			out = append(out, c.CLI)
		}
	}
	sort.Strings(out)
	return out
}

func allowedBaseSet(allowed []string) (set map[string]bool, anyOK bool) {
	if len(allowed) == 0 {
		return nil, true
	}
	set = map[string]bool{}
	for _, a := range allowed {
		if a == "all" {
			return nil, true
		}
		set[policy.BaseCLI(a)] = true
	}
	return set, false
}

func poolFor(allowed, avail []string) []string {
	set, anyOK := allowedBaseSet(allowed)
	var pool []string
	for _, f := range avail {
		if anyOK || set[f] {
			pool = append(pool, f)
		}
	}
	return pool
}

func pickPreferred(pref string, pool []string) string {
	if inList(pool, pref) {
		return pref
	}
	if len(pool) > 0 {
		return pool[0]
	}
	return ""
}

func firstNotEqual(pool []string, x string) string {
	for _, f := range pool {
		if f != x {
			return f
		}
	}
	return ""
}

func inList(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func phaseByRoleOK(rep DetectReport, role string) (PhaseStatus, bool) {
	for _, p := range rep.Phases {
		if p.Role == role {
			return p, true
		}
	}
	return PhaseStatus{}, false
}

func rationale(preset, cli, tier string, clamped, fallback bool, warn string) string {
	if warn != "" {
		return warn
	}
	r := fmt.Sprintf("%s: %s on %s", preset, tier, cli)
	if clamped {
		r += " (clamped to envelope)"
	}
	if fallback {
		r += " (fallback: profile default CLI unavailable)"
	}
	return r
}
