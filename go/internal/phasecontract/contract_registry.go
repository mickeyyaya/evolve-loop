package phasecontract

import "path/filepath"

type Kind int

const (
	KindMarkdown Kind = iota
	KindJSON
)

type JSONShape int

const (
	JSONShapeAny JSONShape = iota
	JSONShapeObject
	JSONShapeArray
)

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

type Roots struct {
	Workspace                       string
	Worktree                        string
	EvolveDir                       string
	DispatchedArtifact              string
	ExplanationDocumentationVersion int
	Cycle                           int
}

const (
	TargetWorkspace = "workspace"
	TargetEvolveDir = "evolve_dir"
)

type Contract struct {
	Phase                        string
	AgentName                    string
	ArtifactName                 string
	Kind                         Kind
	Sections                     []Section
	ExplanationSections          []Section
	Verdicts                     []string
	RequiredKeys                 []string
	JSONShape                    JSONShape
	WriteTarget                  string
	RequireFailureContext        bool
	RequireFailureContextPhaseIO bool
	RequireChallengeToken        bool
	NoArtifact                   bool

	AgentOwedFiles []string
	Effects        []string
	DerivedFrom    map[string]string
	Grammars       []string
}

const EffectInboxClaim = "inbox-claim"

func (c Contract) TopLevelJSONShape() JSONShape {
	if c.JSONShape == JSONShapeAny && len(c.RequiredKeys) > 0 {
		return JSONShapeObject
	}
	return c.JSONShape
}

func (c Contract) ArtifactPath(r Roots) string {
	if c.WriteTarget == TargetEvolveDir {
		return filepath.Join(r.EvolveDir, c.ArtifactName)
	}
	return filepath.Join(r.Workspace, c.ArtifactName)
}

var verdictsPassFailWarnSkp = []string{"PASS", "FAIL", "WARN", "SKIPPED"}

var contracts = map[string]Contract{
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
	"router": {
		Phase: "router", AgentName: "router", ArtifactName: "routing-plan.json",
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
	"ship": {
		Phase: "ship", AgentName: "ship", ArtifactName: "",
		NoArtifact: true,
	},
}

var stagedSections = map[string]bool{HandoffSummary.Canonical: true}

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

var aliases = map[string]string{"advisor": "router"}

func RegistryKey(name string) string {
	if canon, ok := aliases[name]; ok {
		name = canon
	}
	if name == "retro" {
		return "retrospective"
	}
	return name
}

func For(phase string) (Contract, bool) {
	if canon, ok := aliases[phase]; ok {
		phase = canon
	}
	c, ok := contracts[phase]
	return c, ok
}

func ArtifactName(phase string) string {
	c, ok := For(phase)
	if !ok || c.NoArtifact {
		return ""
	}
	return c.ArtifactName
}

func ArtifactFilename(phase string) string {
	if name := ArtifactName(phase); name != "" {
		return name
	}
	return phase + "-report.md"
}

var requiredPhases = []string{"scout", "build", "audit"}

func RequiredRoles() []string {
	out := make([]string, 0, len(requiredPhases))
	for _, p := range requiredPhases {
		if c, ok := contracts[p]; ok {
			out = append(out, c.AgentName)
		}
	}
	return out
}

func RequiredArtifacts() []string {
	out := make([]string, 0, len(requiredPhases))
	for _, p := range requiredPhases {
		if c, ok := contracts[p]; ok && c.ArtifactName != "" {
			out = append(out, c.ArtifactName)
		}
	}
	return out
}

func Contracts() []Contract {
	out := make([]Contract, 0, len(contracts))
	for _, c := range contracts {
		out = append(out, c)
	}
	return out
}
