// Package llmroute is the single resolver for a phase's dispatch decision:
// the ordered CLI fallback chain AND the concrete model, in one place.
//
// Before this package the decision lived in two code paths the runner had to
// stitch together: runner.resolveCLIChain picked the CLI chain, while
// resolvellm.Resolve (invoked only to expand the "auto" model sentinel)
// separately computed a CLI that the runner then DISCARDED. Two readers of
// profile.cli, one ignored. llmroute.Resolve folds both into a single Plan so
// there is exactly one place to reason about "which CLI + model runs this
// phase" — the seam the advisor/Registrar will reuse when it mints a phase.
//
// Precedence (preserved verbatim from the prior two paths):
//
//	CLI primary:  EVOLVE_<AGENT>_CLI > EVOLVE_CLI > profile.cli > "claude-tmux"
//	CLI chain:    primary + profile.cli_fallback (deduped, order-preserving)
//	triggers:     profile.cli_fallback_on_exit or {80,81,124,127}
//	model:        EVOLVE_<AGENT>_MODEL > profile.model_tier_default > defaultModel,
//	              then if the result is "auto", expand via the injected resolver
//	              (the per-phase profile) — same call the runner made before.
//
// Layering: imports envchain + profiles + stdlib only. It MUST NOT import the
// runner, resolvellm, or core. resolvellm stays an independent public API; the
// runner bridges it in via the AutoModel seam, so "auto" expansion is
// byte-identical to the pre-llmroute behavior. (Step 9 removed the
// llm_config.json layer entirely; the resolver now reads the per-phase profile.)
package llmroute

import (
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// defaultFallbackOnExit is the conservative trigger set covering all known
// CLI-side stall + missing-binary signals (mirror of bridge/exitcodes.go;
// kept as integer literals so this leaf package doesn't depend on bridge):
//
//   - 80  ExitREPLBootTimeout    (the *-tmux REPL never showed its prompt)
//   - 81  ExitArtifactTimeout    (bridge artifact-timeout; cycle-122 codex stall)
//   - 85  ExitUnknownPrompt      (pane stuck on an unhandled interactive prompt,
//     incl. provider rate-limit escalations — cycle-267: codex's usage quota
//     exhausted mid-batch, the rate_limit pattern escalated with 85, and the
//     codex→claude chain never fired because 85 wasn't a trigger; a
//     quota-blocked/stuck primary is exactly when a different CLI family can
//     serve. The escalation report is still written before the chain advances.)
//   - 124 coreutils timeout(1)   (defensive — if any wrapper uses `timeout`)
//   - 127 ExitMissingBinary      (the CLI binary isn't on PATH)
//
// Operators extend per-agent via profile.cli_fallback_on_exit (e.g. add 2
// ExitSafetyGate) or shrink to [80,127] for the production-strict posture. A
// CLI failure NOT in this list still hard-fails — a legitimate FAIL verdict
// never silently routes to a different CLI.
var defaultFallbackOnExit = []int{80, 81, 85, 124, 127}

// cliBinaryFor maps a registered CLI driver name to the binary the host needs
// on PATH. Used by Probe to demote candidates whose binary is missing — fast
// fail in milliseconds instead of a 60s REPL boot timeout. Mirror of
// bridge.doctorBinaryFor (kept here so this leaf package stays bridge-free).
var cliBinaryFor = map[string]string{
	"claude-p":    "claude",
	"claude-tmux": "claude",
	"codex":       "codex",
	"codex-tmux":  "codex",
	"agy":         "agy",
	"agy-tmux":    "agy",
	"ollama-tmux": "ollama",
}

// AutoModel expands the "auto" model sentinel for a phase role, returning the
// concrete model (or tier) and ok=false when it cannot (so the caller keeps
// "auto" unchanged, matching the pre-llmroute switch). The runner supplies a
// closure over resolvellm.Resolve; tests can stub it.
type AutoModel func(role string) (model string, ok bool)

// Plan is the resolved dispatch decision for one phase invocation: the ordered
// CLI chain to try, the exit codes that promote to the next CLI, a human label
// for the primary's source, and the resolved (auto-expanded) model.
type Plan struct {
	Candidates    []string // CLI chain, primary first
	Triggers      []int    // exit codes that advance the chain
	PrimarySource string   // "env(EVOLVE_AUDITOR_CLI)" / "env(EVOLVE_CLI)" / "profile.auditor.cli" / "default"
	Model         string   // resolved model, "auto" already expanded when possible
	Tiers         []string // ordered tier fallback chain, resolved tier first (see TierChain)
}

// TriggersFallback reports whether exitCode should advance the chain. A
// non-trigger exit (or zero) breaks the dispatch loop — either the phase
// succeeded or it produced a legitimate FAIL the classifier should see.
func (p Plan) TriggersFallback(exitCode int) bool {
	for _, t := range p.Triggers {
		if t == exitCode {
			return true
		}
	}
	return false
}

// Resolve composes the full dispatch Plan for one phase invocation.
//
//   - agent is the canonical profile name ("auditor", "tdd-engineer"): it keys
//     the per-agent env vars (EVOLVE_<AGENT>_CLI / _MODEL) and the profile.
//   - phase is the phase/role name ("audit", "build"): it is the role passed to
//     the AutoModel expander (keyed by phase, not agent — this asymmetry is
//     preserved from the prior runner behavior, when the now-removed llm_config
//     layer was phase-keyed).
//   - defaultModel is the phase Hooks' DefaultModel() (usually "auto").
//   - prof may be nil (no profile on disk).
//   - autoExpand may be nil (then "auto" is left as-is).
//   - pin may be nil (no policy pin). A non-nil pin is ABSOLUTE: pin.CLI
//     replaces the resolved primary CLI (source="policy.pin"), normalized from a
//     bare family to its default tmux driver (defaultDriverForFamily: "codex" →
//     "codex-tmux"), and pin.Model replaces the resolved model outright —
//     bypassing the env/profile/default
//     chain AND the "auto" expansion (so a pinned model never triggers a
//     resolvellm/catalog lookup). The caller is responsible for the
//     --bypass-policy escape hatch (pass nil to bypass) and for validating
//     the pin against the profile guardrails (policy.ValidatePin) before here.
//     The profile fallback CHAIN is still appended after a pinned primary, so a
//     pinned phase keeps CLI-failure resilience; an operator wanting a strict
//     single-CLI phase empties profile.cli_fallback.
func Resolve(agent, phase, defaultModel string, env map[string]string, prof *profiles.Profile, autoExpand AutoModel, pin *policy.Pin) Plan {
	primary, source := resolvePrimary(agent, env, prof)
	if pin != nil && pin.CLI != "" {
		primary, source = defaultDriverForFamily(pin.CLI), "policy.pin"
	}
	var model string
	tiers := []string(nil)
	if pin != nil && pin.Model != "" {
		model = pin.Model       // absolute — skip the env/profile/default/auto chain entirely
		tiers = []string{model} // a pin is ABSOLUTE: never tier-step-down away from it
	} else {
		model = resolveModel(agent, phase, defaultModel, env, prof, autoExpand)
		tiers = TierChain(model, envelopeMin(prof))
	}
	return Plan{
		Candidates:    candidatesFrom(primary, prof),
		Triggers:      resolveTriggers(prof),
		PrimarySource: source,
		Model:         model,
		Tiers:         tiers,
	}
}

// envelopeMin returns the profile's ModelTierEnvelope.Min tier floor, or ""
// (→ TierChain's universal "balanced" floor) when the profile or envelope is
// absent.
func envelopeMin(prof *profiles.Profile) string {
	if prof == nil || prof.ModelTierEnvelope == nil {
		return ""
	}
	return prof.ModelTierEnvelope.Min
}

// resolveModel runs the model precedence: request override > profile
// .model_tier_default > defaultModel, then expands "auto" via autoExpand.
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

// defaultDriverForFamily normalizes a bare CLI family (e.g. "codex") to its
// default interactive driver ("codex-tmux") when one is registered. Policy pins
// and `evolve setup apply` emit bare base families (Assignment.CLI is the base
// family), but the dispatch default is the tmux driver (CLAUDE.md: "Default
// execution = tmux-LLM drivers"). The headless "<family>" driver lacks the
// manifest model_tier_map and the codex ChatGPT model clamp, so a bare-family
// pin previously selected it and codex exited rc=1 every cycle (cycle-378). A
// name that is already driver-qualified, or whose "<family>-tmux" form is not a
// registered driver (e.g. the explicit headless "claude-p"), is returned
// unchanged. cliBinaryFor is the single source of registered driver names.
func defaultDriverForFamily(cli string) string {
	if _, ok := cliBinaryFor[cli+"-tmux"]; ok {
		return cli + "-tmux"
	}
	return cli
}

// resolvePrimary returns the primary CLI and its provenance label.
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

// candidatesFrom builds the chain: primary first, then the deduped
// profile.cli_fallback list (whitespace-trimmed, empties dropped, first
// occurrence wins to preserve operator order).
func candidatesFrom(primary string, prof *profiles.Profile) []string {
	candidates := []string{primary}
	if prof == nil {
		return candidates
	}
	seen := map[string]struct{}{primary: {}}
	for _, c := range prof.CLIFallback {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if _, dup := seen[c]; dup {
			continue
		}
		seen[c] = struct{}{}
		candidates = append(candidates, c)
	}
	return candidates
}

// resolveTriggers returns profile.cli_fallback_on_exit or the conservative
// default when unset.
func resolveTriggers(prof *profiles.Profile) []int {
	if prof != nil && len(prof.CLIFallbackOnExit) > 0 {
		return append([]int(nil), prof.CLIFallbackOnExit...)
	}
	return defaultFallbackOnExit
}

// Probe returns a copy of p with candidates whose binary isn't on PATH demoted
// (not deleted) to the end of the chain, so an already-missing CLI doesn't burn
// a 60s boot timeout before the chain advances. If ALL candidates are missing
// the original order is kept so the classifier still sees a real
// ExitMissingBinary. lookPath is the seam: production passes nil (exec.LookPath);
// tests inject a closure.
//
// The reorder is intentionally a demote, not a drop: a CLI may be installed but
// not yet on PATH at probe time, and the bridge's later launch may still resolve
// it via a richer search.
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
		return p
	}
	// Copy p and overwrite only Candidates so any future Plan field is carried
	// through Probe automatically (no silent omission).
	out := p
	out.Candidates = append(available, missing...)
	return out
}

// hasAvailableCandidate reports whether any candidate's binary is on PATH. An
// unknown candidate name (not in cliBinaryFor) is treated as available — it
// matches Probe's "unknown name keeps position" and means an
// operator-configured novel CLI is trusted over discovery.
func hasAvailableCandidate(candidates []string, lookPath func(string) (string, error)) bool {
	for _, cli := range candidates {
		bin := cliBinaryFor[cli]
		if bin == "" {
			return true
		}
		if _, err := lookPath(bin); err == nil {
			return true
		}
	}
	return false
}

// ApplyUniversalFallback is the LAST-RESORT dispatch tier (any_cli_any_phase
// invariant): when EVERY candidate in the static chain (primary +
// profile.cli_fallback) has an absent binary — e.g. an isolated agy-only host
// whose profiles still name claude/codex — it appends the caller-DISCOVERED
// CLIs so the loop routes to whatever LLM is actually installed instead of
// halting the batch. Discovered CLIs (already filtered by the caller to
// binary-present + auth-configured + phase-allowlist + policy) are appended
// AFTER the configured chain, so an operator CLI merely not-yet-on-PATH still
// leads, and are deduped against it.
//
// It is a NO-OP when any static candidate is available (operator config is
// authoritative — universal fallback never overrides a working configured CLI)
// or when discovered is empty (fail-loud preserved: the classifier still sees a
// real ExitMissingBinary on the absent chain, never a silent green). lookPath is
// the seam (nil ⇒ exec.LookPath). Non-Candidates Plan fields are carried through.
func ApplyUniversalFallback(p Plan, discovered []string, lookPath func(string) (string, error)) Plan {
	if len(discovered) == 0 {
		return p
	}
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	if hasAvailableCandidate(p.Candidates, lookPath) {
		return p // a configured CLI is usable → last resort not needed
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

// Family maps a registered CLI driver name to its CLI family — the binary
// name from cliBinaryFor ("codex-tmux" → "codex"). A transient outage (quota
// wall, auth expiry) hits every transport of a family, so health state is
// keyed here, not per driver. Unknown names map to themselves.
func Family(cli string) string {
	if bin := cliBinaryFor[cli]; bin != "" {
		return bin
	}
	return cli
}

// ApplyDriverBench demotes candidates whose specific driver is bench-flagged
// (driver-scoped boot-timeout bench: "codex-tmux" benched ≠ "codex" benched).
// benchedDrivers maps driver name → BenchedAt from clihealth.Active(). Bench
// is advice, never a veto: when ALL candidates are driver-benched, the chain is
// ordered least-recently-benched first (same policy as ApplyBench). Keyed on
// the full driver name, never on Family(cli), so headless/tmux variants are
// independent. Copy-struct convention carries non-Candidates fields.
func ApplyDriverBench(p Plan, benchedDrivers map[string]time.Time) Plan {
	if len(p.Candidates) <= 1 || len(benchedDrivers) == 0 {
		return p
	}
	var healthy, demoted []string
	for _, cli := range p.Candidates {
		if _, hit := benchedDrivers[cli]; hit {
			demoted = append(demoted, cli)
		} else {
			healthy = append(healthy, cli)
		}
	}
	out := p
	if len(healthy) == 0 {
		all := append([]string(nil), p.Candidates...)
		sort.SliceStable(all, func(i, j int) bool {
			return benchedDrivers[all[i]].Before(benchedDrivers[all[j]])
		})
		out.Candidates = all
		return out
	}
	out.Candidates = append(healthy, demoted...)
	return out
}

// ApplyBench demotes candidates whose family is benched (cycle-283: a walled
// codex re-burned its 5-15min boot on every dispatch) to the chain end,
// mirroring Probe's demote-not-drop reorder. benched maps family → BenchedAt.
// Bench is advice, never a veto: when EVERY candidate is benched the chain is
// instead ordered least-recently-benched first — the caller logs loudly and
// dispatch proceeds. Copy-struct convention carries non-Candidates fields.
func ApplyBench(p Plan, benched map[string]time.Time) Plan {
	if len(p.Candidates) <= 1 || len(benched) == 0 {
		return p
	}
	var healthy, demoted []string
	for _, cli := range p.Candidates {
		if _, hit := benched[Family(cli)]; hit {
			demoted = append(demoted, cli)
		} else {
			healthy = append(healthy, cli)
		}
	}
	out := p
	if len(healthy) == 0 {
		all := append([]string(nil), p.Candidates...)
		sort.SliceStable(all, func(i, j int) bool {
			return benched[Family(all[i])].Before(benched[Family(all[j])])
		})
		out.Candidates = all
		return out
	}
	out.Candidates = append(healthy, demoted...)
	return out
}
