// Package llmroute resolves a phase's dispatch plan (CLI chain, triggers, model, tiers) and walks it.
// For an overlay CLI, a name that is not a registered driver is a family selector (matched by Family), a registered driver name is a driver selector, and an exact chain entry outranks both.
// See docs/architecture/packages/internal-llmroute.md.
package llmroute

import (
	"os/exec"
	"slices"
	"sort"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

const ExitModelMismatch = 87

var defaultFallbackOnExit = []int{80, 81, 85, ExitModelMismatch, 124, 127}

var cliBinaryFor = map[string]string{
	"claude-p":        "claude",
	"claude-tmux":     "claude",
	"codex":           "codex",
	"codex-tmux":      "codex",
	"agy":             "agy",
	"agy-tmux":        "agy",
	"agy-claude-tmux": "agy",
	"ollama-tmux":     "ollama",
}

type AutoModel func(role string) (model string, ok bool)

type Plan struct {
	Candidates    []string
	Triggers      []int
	PrimarySource string
	Model         string
	Tiers         []string

	TierCeiling map[string][]string
}

func (p Plan) Permits(cli, tier string) bool {
	if name := policy.TierName(policy.TierRank(tier)); name != "" {
		tier = name
	}
	allowed, capped := p.TierCeiling[tier]
	if !capped {
		return true
	}
	return slices.Contains(allowed, Family(cli))
}

func DefaultDriverForFamily(cli string) string { return defaultDriverForFamily(cli) }

func (p Plan) TriggersFallback(exitCode int) bool {
	for _, t := range p.Triggers {
		if t == exitCode {
			return true
		}
	}
	return false
}

func Resolve(agent, phase, defaultModel string, env map[string]string, prof *profiles.Profile, autoExpand AutoModel, pin *policy.Pin) Plan {
	primary, source := resolvePrimary(agent, env, prof)
	if pin != nil && pin.CLI != "" {
		primary, source = defaultDriverForFamily(pin.CLI), "policy.pin"
	}
	var model string
	tiers := []string(nil)
	if pin != nil && pin.Model != "" {
		model = pin.Model
		tiers = []string{model}
	} else {
		model = resolveModel(agent, phase, defaultModel, env, prof, autoExpand)
		tiers = TierChain(model, envelopeMin(prof))
	}
	return Plan{
		Candidates:    buildCandidates(primary, prof, false),
		Triggers:      resolveTriggers(prof),
		PrimarySource: source,
		Model:         model,
		Tiers:         tiers,
	}
}

func envelopeMin(prof *profiles.Profile) string {
	if prof == nil || prof.ModelTierEnvelope == nil {
		return ""
	}
	return prof.ModelTierEnvelope.Min
}

func resolveModel(agent, phase, defaultModel string, env map[string]string, prof *profiles.Profile, autoExpand AutoModel) string {
	profileModelTier := ""
	if prof != nil {
		profileModelTier = prof.ModelTierDefault
	}
	model := env[envchain.PhaseEnvKey(agent, "MODEL")]
	if model == "" {
		model = profileModelTier
	}
	if model == "" {
		model = defaultModel
	}
	if model == "auto" && autoExpand != nil {
		if m, ok := autoExpand(phase); ok {
			model = m
		}
	}
	return model
}

func defaultDriverForFamily(cli string) string {
	if _, ok := cliBinaryFor[cli+"-tmux"]; ok {
		return cli + "-tmux"
	}
	return cli
}

func EnvPrimary(agent string, env map[string]string) (cli, source string) {
	perAgentKey := envchain.PhaseEnvKey(agent, "CLI")
	if v := env[perAgentKey]; v != "" {
		return v, "env(" + perAgentKey + ")"
	}
	if v := envchain.Resolve("EVOLVE_CLI", env, "", ""); v != "" {
		return v, "env(EVOLVE_CLI)"
	}
	return "", ""
}

func resolvePrimary(agent string, env map[string]string, prof *profiles.Profile) (cli, source string) {
	if cli, source := EnvPrimary(agent, env); cli != "" {
		return cli, source
	}
	if prof != nil && prof.CLI != "" {
		return prof.CLI, "profile." + agent + ".cli"
	}
	return "claude-tmux", "default"
}

func resolveTriggers(prof *profiles.Profile) []int {
	if prof != nil && len(prof.CLIFallbackOnExit) > 0 {
		return append([]int(nil), prof.CLIFallbackOnExit...)
	}
	return DefaultTriggers()
}

func Probe(p Plan, lookPath func(string) (string, error)) Plan {
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	if len(p.Candidates) <= 1 {
		return p
	}
	var available, missing []string
	for _, cli := range p.Candidates {
		bin := cliBinaryFor[cli]
		if bin == "" {
			available = append(available, cli)
			continue
		}
		if _, err := lookPath(bin); err == nil {
			available = append(available, cli)
		} else {
			missing = append(missing, cli)
		}
	}
	if len(available) == 0 {
		return p
	}
	out := p
	out.Candidates = append(available, missing...)
	return out
}

func ApplyUniversalFallback(p Plan, discovered []string) Plan {
	if len(discovered) == 0 {
		return p
	}
	seen := make(map[string]struct{}, len(p.Candidates))
	for _, c := range p.Candidates {
		seen[c] = struct{}{}
	}
	cands := append([]string(nil), p.Candidates...)
	for _, d := range discovered {
		if _, dup := seen[d]; dup {
			continue
		}
		seen[d] = struct{}{}
		cands = append(cands, d)
	}
	out := p
	out.Candidates = cands
	return out
}

func Family(cli string) string {
	if _, registered := cliBinaryFor[cli]; registered {
		return profiles.BaseCLI(cli)
	}
	return cli
}

func Binary(cli string) string { return cliBinaryFor[cli] }

func ApplyDriverBench(p Plan, benchedDrivers map[string]time.Time) Plan {
	return applyBenchKeyed(p, benchedDrivers, driverName)
}

func ApplyBench(p Plan, benched map[string]time.Time) Plan {
	return applyBenchKeyed(p, benched, Family)
}

func driverName(cli string) string { return cli }

func applyBenchKeyed(p Plan, benched map[string]time.Time, benchKey func(cli string) string) Plan {
	if len(p.Candidates) <= 1 || len(benched) == 0 {
		return p
	}
	var healthy, demoted []string
	for _, cli := range p.Candidates {
		if _, hit := benched[benchKey(cli)]; hit {
			demoted = append(demoted, cli)
		} else {
			healthy = append(healthy, cli)
		}
	}
	out := p
	if len(healthy) == 0 {
		all := append([]string(nil), p.Candidates...)
		sort.SliceStable(all, func(i, j int) bool {
			return benched[benchKey(all[i])].Before(benched[benchKey(all[j])])
		})
		out.Candidates = all
		return out
	}
	out.Candidates = append(healthy, demoted...)
	return out
}
