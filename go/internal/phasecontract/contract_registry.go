package phasecontract

import "path/filepath"

// Kind is the deliverable shape the verifier validates: markdown sections and verdict, or JSON.
type Kind int

// Kind values.
const (
	KindMarkdown Kind = iota
	KindJSON
)

// JSONShape constrains a JSON deliverable's top-level value; the zero value accepts any value.
type JSONShape int

// JSONShape values.
const (
	JSONShapeAny JSONShape = iota
	JSONShapeObject
	JSONShapeArray
)

// String names the shape as the prompt describes it: object, array, or value.
func (s JSONShape) String() string {
	switch s {
	case JSONShapeObject:
		return "object"
	case JSONShapeArray:
		return "array"
	default:
		return "value"
	}
}

// Roots carries the directories an artifact path resolves against, plus the verifying cycle's context.
type Roots struct {
	Workspace string
	Worktree  string
	EvolveDir string
	// DispatchedArtifact overrides the contract path with the exact path the runner dispatched; only the runner sets it.
	DispatchedArtifact string
	// ExplanationDocumentationVersion is the cycle's explanation-documentation contract version; 0 means inactive.
	ExplanationDocumentationVersion int
	// Cycle is the verifying cycle number; 0 means unknown, and a declared effect then fails open with an error.
	Cycle int
}

// WriteTarget values; Roots.Worktree is only checked for stray reports, never a target.
const (
	TargetWorkspace = "workspace"
	TargetEvolveDir = "evolve_dir"
)

// Contract is one deliverable protocol: its output location, kind and well-formedness rules.
type Contract struct {
	Phase        string
	AgentName    string // profile/persona basename (e.g. "builder", "tdd-engineer")
	ArtifactName string
	Kind         Kind
	Sections     []Section // markdown only
	// ExplanationSections are owed only while Roots.ExplanationDocumentationVersion != 0. Markdown only.
	ExplanationSections []Section
	Verdicts            []string // markdown only — allowed verdict tokens
	RequiredKeys        []string // json only — minimal required top-level keys
	JSONShape           JSONShape
	WriteTarget         string // one of Target*
	// RequireFailureContext makes a FAIL/WARN sentinel without a failure block a violation; prose-only verdicts stay legal.
	RequireFailureContext bool
	// RequireFailureContextPhaseIO applies the same check to verdict-less phases, only at EVOLVE_PHASE_IO>=enforce.
	RequireFailureContextPhaseIO bool
	// RequireChallengeToken makes a report that does not echo the minted token a violation; fails open with no token.
	// scout mints the token, so it must never set this.
	RequireChallengeToken bool
	// NoArtifact marks a phase whose result is a side effect, not a file; the verifier passes it.
	NoArtifact bool

	// AgentOwedFiles and Effects come only from the registry declaration, never a built-in literal. See ADR-0100.
	AgentOwedFiles []string
	Effects        []string
}

// EffectInboxClaim names the effect of claiming a cycle's committed inbox items.
const EffectInboxClaim = "inbox-claim"

// TopLevelJSONShape returns JSONShape, or object when RequiredKeys are set, since keys apply only to an object.
func (c Contract) TopLevelJSONShape() JSONShape {
	if c.JSONShape == JSONShapeAny && len(c.RequiredKeys) > 0 {
		return JSONShapeObject
	}
	return c.JSONShape
}

// ArtifactPath joins ArtifactName to the root that WriteTarget selects.
func (c Contract) ArtifactPath(r Roots) string {
	if c.WriteTarget == TargetEvolveDir {
		return filepath.Join(r.EvolveDir, c.ArtifactName)
	}
	return filepath.Join(r.Workspace, c.ArtifactName)
}

// verdictsPassFailWarnSkp is audit's verdict vocabulary; audit is the only built-in that extracts a verdict token.
var verdictsPassFailWarnSkp = []string{"PASS", "FAIL", "WARN", "SKIPPED"}

// contracts is the built-in registry; Sections are wired from the Report vars so headings stay single-sourced.
var contracts = map[string]Contract{
	// These phases classify on section presence, so Verdicts stays nil: a verdict
	// requirement they never emit would false-block at enforce.
	"build": {
		Phase: "build", AgentName: "builder", ArtifactName: "build-report.md",
		Kind: KindMarkdown, Sections: alwaysOn(Build.Sections), Verdicts: nil,
		WriteTarget: TargetWorkspace, RequireChallengeToken: true,
		RequireFailureContextPhaseIO: true,
	},
	"scout": {
		Phase: "scout", AgentName: "scout", ArtifactName: "scout-report.md",
		Kind: KindMarkdown, Sections: alwaysOn(Scout.Sections), Verdicts: nil,
		WriteTarget: TargetWorkspace, RequireFailureContextPhaseIO: true,
	},
	"tdd": {
		Phase: "tdd", AgentName: "tdd-engineer", ArtifactName: "test-report.md",
		Kind: KindMarkdown, Sections: TDD.Sections, Verdicts: nil,
		WriteTarget: TargetWorkspace,
	},
	"audit": {
		Phase: "audit", AgentName: "auditor", ArtifactName: "audit-report.md",
		Kind: KindMarkdown, Sections: alwaysOn(Audit.Sections), Verdicts: verdictsPassFailWarnSkp,
		WriteTarget: TargetWorkspace, RequireFailureContext: true,
		ExplanationSections: []Section{ExplanationDocumentation},
	},
	"intent": {
		Phase: "intent", AgentName: "intent", ArtifactName: "intent.md",
		Kind: KindMarkdown, Sections: Intent.Sections, Verdicts: nil,
		WriteTarget: TargetWorkspace,
	},
	"triage": {
		Phase: "triage", AgentName: "triage", ArtifactName: "triage-report.md",
		Kind: KindMarkdown, Sections: Triage.Sections, Verdicts: nil,
		WriteTarget: TargetWorkspace, RequireFailureContextPhaseIO: true,
	},
	// The three router protocols share one persona but keep distinct contracts, so a
	// proposal or replan self-check never validates a stale whole-cycle plan.
	"router": {
		Phase: "router", AgentName: "router", ArtifactName: "routing-plan.json",
		// PhaseAdvisor writes a bare JSON array, which has no keys to require.
		Kind: KindJSON, JSONShape: JSONShapeArray,
		WriteTarget: TargetWorkspace,
	},
	"router-replan": {
		Phase: "router-replan", AgentName: "router", ArtifactName: "routing-replan.json",
		Kind: KindJSON, JSONShape: JSONShapeArray,
		WriteTarget: TargetWorkspace,
	},
	"router-proposal": {
		Phase: "router-proposal", AgentName: "router", ArtifactName: "routing-proposal.json",
		Kind: KindJSON, JSONShape: JSONShapeObject,
		WriteTarget: TargetWorkspace,
	},
	"orchestrator": {
		Phase: "orchestrator", AgentName: "orchestrator", ArtifactName: "cycle-state.json",
		Kind: KindJSON, RequiredKeys: []string{"cycle_id", "phase"}, JSONShape: JSONShapeObject,
		WriteTarget: TargetEvolveDir,
	},
	// Registered for location and well-formedness only; nil Sections/Verdicts invent
	// no requirement these phases never emitted.
	"retro": {
		Phase: "retro", AgentName: "retrospective", ArtifactName: "retrospective-report.md",
		Kind: KindMarkdown, Sections: nil, Verdicts: nil,
		WriteTarget: TargetWorkspace,
	},
	"build-planner": {
		Phase: "build-planner", AgentName: "build-planner", ArtifactName: "build-plan.md",
		Kind: KindMarkdown, Sections: nil, Verdicts: nil,
		WriteTarget: TargetWorkspace,
	},
	// ship's deliverable is the pushed commit, which the ship-gate and commit-gate
	// attestation enforce; the explicit contract passes instead of failing open.
	"ship": {
		Phase: "ship", AgentName: "ship", ArtifactName: "",
		NoArtifact: true,
	},
}

// stagedSections are declared sections whose enforcement rolls out through their own gate
// (HandoffSummary: the report-size gate), so they cannot block before that gate is promoted.
var stagedSections = map[string]bool{HandoffSummary.Canonical: true}

// alwaysOn returns the declared sections minus stagedSections.
func alwaysOn(sections []Section) []Section {
	out := make([]Section, 0, len(sections))
	for _, s := range sections {
		if stagedSections[s.Canonical] {
			continue
		}
		out = append(out, s)
	}
	return out
}

// aliases maps human-facing names to the registry key.
var aliases = map[string]string{"advisor": "router"}

// RegistryKey maps a core or human-facing phase name to its phase-registry key (advisor→router, retro→retrospective).
func RegistryKey(name string) string {
	if canon, ok := aliases[name]; ok {
		name = canon
	}
	if name == "retro" {
		return "retrospective"
	}
	return name
}

// For returns the built-in contract for a phase, resolving human-facing aliases.
func For(phase string) (Contract, bool) {
	if canon, ok := aliases[phase]; ok {
		phase = canon
	}
	c, ok := contracts[phase]
	return c, ok
}

// ArtifactName returns a phase's registered filename, or "" when the phase is unregistered or NoArtifact.
func ArtifactName(phase string) string {
	c, ok := For(phase)
	if !ok || c.NoArtifact {
		return ""
	}
	return c.ArtifactName
}

// ArtifactFilename returns ArtifactName, falling back to "<phase>-report.md"; use ArtifactName when "" is meaningful.
func ArtifactFilename(phase string) string {
	if name := ArtifactName(phase); name != "" {
		return name
	}
	return phase + "-report.md"
}

// requiredPhases are the spine phases every completed cycle must have produced.
var requiredPhases = []string{"scout", "build", "audit"}

// RequiredRoles returns the registry AgentName of each phase a completed cycle must have run.
func RequiredRoles() []string {
	out := make([]string, 0, len(requiredPhases))
	for _, p := range requiredPhases {
		if c, ok := contracts[p]; ok {
			out = append(out, c.AgentName)
		}
	}
	return out
}

// RequiredArtifacts returns the registry ArtifactName of each phase a completed cycle must have run.
func RequiredArtifacts() []string {
	out := make([]string, 0, len(requiredPhases))
	for _, p := range requiredPhases {
		if c, ok := contracts[p]; ok && c.ArtifactName != "" {
			out = append(out, c.ArtifactName)
		}
	}
	return out
}

// Contracts returns every built-in contract in unspecified order.
func Contracts() []Contract {
	out := make([]Contract, 0, len(contracts))
	for _, c := range contracts {
		out = append(out, c)
	}
	return out
}
