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

// defaultFallbackOnExit mirrors bridge/exitcodes.go as literals so this leaf package stays bridge-free:
// REPL boot timeout, artifact timeout, unknown prompt (incl. quota), timeout(1), missing binary.
var defaultFallbackOnExit = []int{80, 81, 85, 124, 127}

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

// AutoModel expands the "auto" sentinel for a phase role; ok=false leaves "auto" unchanged.
type AutoModel func(role string) (model string, ok bool)

// Plan is the resolved dispatch decision for one phase invocation.
type Plan struct {
	Candidates    []string // CLI chain, primary first
	Triggers      []int    // exit codes that advance the chain
	PrimarySource string   // "env(EVOLVE_AUDITOR_CLI)" / "env(EVOLVE_CLI)" / "profile.auditor.cli" / "policy.pin" / "default"
	Model         string   // resolved model, "auto" already expanded when possible
	Tiers         []string // ordered tier fallback chain, resolved tier first (see TierChain)

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

// TriggersFallback reports whether exitCode advances the chain; any other exit is a result, not a stall.
func (p Plan) TriggersFallback(exitCode int) bool {
	for _, t := range p.Triggers {
		if t == exitCode {
			return true
		}
	}
	return false
}

// Resolve composes the dispatch Plan: env and profile keyed by agent, AutoModel by phase; a non-nil pin is absolute.
func Resolve(agent, phase, defaultModel string, env map[string]string, prof *profiles.Profile, autoExpand AutoModel, pin *policy.Pin) Plan {
	primary, source := resolvePrimary(agent, env, prof)
	if pin != nil && pin.CLI != "" {
		primary, source = defaultDriverForFamily(pin.CLI), "policy.pin"
	}
	var model string
	tiers := []string(nil)
	if pin != nil && pin.Model != "" {
		model = pin.Model       // skips the env/profile/default chain and "auto" expansion
		tiers = []string{model} // a pinned model never steps down a tier
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

// envelopeMin returns the profile's tier floor, or "" (TierChain's universal floor) when absent.
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

// defaultDriverForFamily maps a bare family to its "<family>-tmux" driver when registered, since pins
// carry bare families and the headless driver lacks codex's model clamp; other names pass through.
func defaultDriverForFamily(cli string) string {
	if _, ok := cliBinaryFor[cli+"-tmux"]; ok {
		return cli + "-tmux"
	}
	return cli
}

func resolvePrimary(agent string, env map[string]string, prof *profiles.Profile) (cli, source string) {
	perAgentKey := envchain.PhaseEnvKey(agent, "CLI")
	if v := env[perAgentKey]; v != "" {
		return v, "env(" + perAgentKey + ")"
	}
	if v := envchain.Resolve("EVOLVE_CLI", env, "", ""); v != "" {
		return v, "env(EVOLVE_CLI)"
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

// Probe demotes (never drops) candidates whose binary is off PATH; nil lookPath means exec.LookPath.
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
			available = append(available, cli) // unknown name — keep position
			continue
		}
		if _, err := lookPath(bin); err == nil {
			available = append(available, cli)
		} else {
			missing = append(missing, cli)
		}
	}
	if len(available) == 0 {
		return p // all missing: keep the order so the classifier sees ExitMissingBinary
	}
	out := p
	out.Candidates = append(available, missing...)
	return out
}

// ApplyUniversalFallback appends the discovered CLIs after the configured chain, deduped against it.
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

// Family maps a driver name to its CLI family ("codex-tmux" → "codex"); unknown names map to themselves.
func Family(cli string) string {
	if _, registered := cliBinaryFor[cli]; registered {
		return profiles.BaseCLI(cli)
	}
	return cli
}

func Binary(cli string) string { return cliBinaryFor[cli] }

// ApplyDriverBench demotes candidates benched by full driver name (driver → BenchedAt), never by family.
func ApplyDriverBench(p Plan, benchedDrivers map[string]time.Time) Plan {
	return applyBenchKeyed(p, benchedDrivers, driverName)
}

// ApplyBench demotes candidates whose family is benched (family → BenchedAt); all benched runs least-recent first.
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
	if len(healthy) == 0 { // bench is advice, never a veto
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
