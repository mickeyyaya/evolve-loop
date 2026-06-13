package phasecontract

import "path/filepath"

// This file extends the phasecontract SSOT (see contract.go) from "report
// section headings" to a full per-agent Contract: WHERE the deliverable is
// written, WHAT kind it is, and the well-formedness rules. It is consumed by
// the shared go/internal/deliverable package (the `evolve phase verify`
// self-check AND the host-side contract gate run the SAME checks against this
// registry), and by the bridge prompt-injection that tells each agent its exact
// output path. Design: ADR-0034.

// Kind distinguishes the two deliverable shapes the verifier knows how to
// validate. Markdown deliverables are checked for required Sections + a parseable
// verdict; JSON deliverables are checked for valid JSON + RequiredKeys (a
// tolerant reader — unknown/future keys are ignored).
type Kind int

const (
	KindMarkdown Kind = iota
	KindJSON
)

// Roots carries the three real directories an artifact can live in. A Contract's
// WriteTarget selects which one ArtifactPath joins against. EvolveDir is the
// project's .evolve/ dir (where cycle-state.json lives); Workspace is the
// per-cycle .evolve/runs/cycle-N/ dir (where phase reports live); Worktree is
// the isolated build worktree.
type Roots struct {
	Workspace string
	Worktree  string
	EvolveDir string
}

// WriteTarget values. Every deliverable currently lands in either the per-cycle
// workspace (phase reports, routing-plan.json) or the .evolve dir
// (cycle-state.json). Roots.Worktree exists for the "report must NOT be stray in
// the worktree" check in the verifier, not as a deliverable target.
const (
	TargetWorkspace = "workspace"
	TargetEvolveDir = "evolve_dir"
)

// Contract is the deliverable contract for one agent: the SSOT for its output
// location, kind, and well-formedness rules. For markdown kinds, Sections and
// Verdicts mirror the phase's Report (no duplication — they are wired from the
// same vars in contract.go). For JSON kinds, RequiredKeys lists the minimal
// stable top-level keys the verifier requires.
type Contract struct {
	Phase        string
	AgentName    string // profile/persona basename (e.g. "builder", "tdd-engineer")
	ArtifactName string // runtime-truth filename (matches the in-process runner hook)
	Kind         Kind
	Sections     []Section // markdown only
	Verdicts     []string  // markdown only — allowed verdict tokens
	RequiredKeys []string  // json only — minimal required top-level keys
	WriteTarget  string    // one of Target*
	// RequireFailureContext makes a FAIL/WARN verdict sentinel without a
	// structured failure block a violation (ADR-0039 §7) — the correction
	// loop then re-dispatches with the exact fix. Applies only to
	// sentinel-declared verdicts: legacy prose-only artifacts stay legal
	// forever. Set for built-ins that extract a verdict; user phases opt in
	// via classify.require_failure_context.
	RequireFailureContext bool
	// RequireChallengeToken makes a report that fails to echo the minted
	// <workspace>/challenge-token.txt token a violation (cycle-269: the
	// proof-of-read protocol was audit-enforced only — unrecoverable — and
	// the bash→Go migration had dropped the prompt-side injection entirely).
	// The runner injects the token block at dispatch; the deliverable gate
	// checks the echo so the correction loop re-dispatches with the exact
	// fix BEFORE audit. Fail-open when no token was minted. scout (the
	// minter) must never set this — echoing yourself is circular.
	RequireChallengeToken bool
}

// ArtifactPath resolves the absolute path the agent must write to, joining the
// ArtifactName against the root selected by WriteTarget.
func (c Contract) ArtifactPath(r Roots) string {
	if c.WriteTarget == TargetEvolveDir {
		return filepath.Join(r.EvolveDir, c.ArtifactName)
	}
	return filepath.Join(r.Workspace, c.ArtifactName)
}

// markdownVerdicts is the standard verdict-token vocabulary for phases that
// declare a verdict. Audit additionally emits SKIPPED.
// verdictsPassFailWarnSkp is audit's verdict vocabulary — audit is the only
// phase whose classifier extracts a verdict token (the others classify on
// section presence, so their contracts leave Verdicts nil).
var verdictsPassFailWarnSkp = []string{"PASS", "FAIL", "WARN", "SKIPPED"}

// contracts is the registry: the 6 phase agents + the advisor (LLM routing
// brain, JSON deliverable) + the orchestrator (host-side driver, validates its
// own cycle-state.json). Section sets are wired from the Report vars in
// contract.go so the headings stay single-sourced.
var contracts = map[string]Contract{
	// build/scout/tdd/intent/triage classify on SECTION presence, not a verdict
	// token (only audit extracts a verdict). Leaving Verdicts nil keeps the
	// contract gate strictly additive — it requires the same sections the
	// existing classifiers do, plus correct location, without inventing a
	// verdict requirement those phases never emitted (which would false-block at
	// enforce).
	"build": {
		Phase: "build", AgentName: "builder", ArtifactName: "build-report.md",
		Kind: KindMarkdown, Sections: Build.Sections, Verdicts: nil,
		WriteTarget: TargetWorkspace, RequireChallengeToken: true,
	},
	"scout": {
		Phase: "scout", AgentName: "scout", ArtifactName: "scout-report.md",
		Kind: KindMarkdown, Sections: Scout.Sections, Verdicts: nil,
		WriteTarget: TargetWorkspace,
	},
	"tdd": {
		Phase: "tdd", AgentName: "tdd-engineer", ArtifactName: "test-report.md",
		Kind: KindMarkdown, Sections: TDD.Sections, Verdicts: nil,
		WriteTarget: TargetWorkspace,
	},
	"audit": {
		Phase: "audit", AgentName: "auditor", ArtifactName: "audit-report.md",
		Kind: KindMarkdown, Sections: Audit.Sections, Verdicts: verdictsPassFailWarnSkp,
		WriteTarget: TargetWorkspace, RequireFailureContext: true,
	},
	"intent": {
		Phase: "intent", AgentName: "intent", ArtifactName: "intent.md",
		Kind: KindMarkdown, Sections: Intent.Sections, Verdicts: nil,
		WriteTarget: TargetWorkspace,
	},
	"triage": {
		Phase: "triage", AgentName: "triage", ArtifactName: "triage-report.md",
		Kind: KindMarkdown, Sections: Triage.Sections, Verdicts: nil,
		WriteTarget: TargetWorkspace,
	},
	// The routing brain (PhaseAdvisor) dispatches with Agent="router" (persona
	// agents/evolve-router.md, profile router.json) and writes routing-plan.json
	// in whole-cycle Plan mode. Keyed by the wire identity "router" so the bridge
	// injects the contract; "advisor" resolves here too via aliases.
	"router": {
		Phase: "router", AgentName: "router", ArtifactName: "routing-plan.json",
		// routing-plan.json is a BARE JSON ARRAY (PhaseAdvisor.Plan writes "a
		// strict JSON array"; the consumer parses an array). No required keys —
		// an array has none. The prior RequiredKeys=["plan"] expected an object
		// and failed `evolve phase verify router` every cycle.
		Kind: KindJSON, RequiredKeys: nil,
		WriteTarget: TargetWorkspace,
	},
	"orchestrator": {
		Phase: "orchestrator", AgentName: "orchestrator", ArtifactName: "cycle-state.json",
		Kind: KindJSON, RequiredKeys: []string{"cycle_id", "phase"},
		WriteTarget: TargetEvolveDir,
	},
}

// aliases maps human-facing names to the canonical wire identity used as the
// registry key. "advisor" is the conceptual name for the routing brain whose
// agent identity on the wire is "router".
var aliases = map[string]string{"advisor": "router"}

// For returns the contract for a phase/agent and whether one is registered.
// Human-facing aliases (e.g. "advisor") resolve to their canonical key.
func For(phase string) (Contract, bool) {
	if canon, ok := aliases[phase]; ok {
		phase = canon
	}
	c, ok := contracts[phase]
	return c, ok
}

// Contracts returns every registered contract (stable order not guaranteed;
// callers that need order should sort by Phase).
func Contracts() []Contract {
	out := make([]Contract, 0, len(contracts))
	for _, c := range contracts {
		out = append(out, c)
	}
	return out
}
