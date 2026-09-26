// Package phasespec parses the phase registry and per-phase overlays into a
// Catalog of declarative phase definitions. See docs/architecture/packages/internal-phasespec.md.
package phasespec

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// IO is a phase's typed input/output contract: the artifact files and namespaced signals.
type IO struct {
	Files   []string `json:"files,omitempty"`
	Signals []string `json:"signals,omitempty"`
	// AgentOwed names the secondary outputs (basenames of Files[1:]) the agent
	// must write itself; Files[0] is always owed.
	AgentOwed []string `json:"agent_owed,omitempty"`
	// HarnessProduced names the secondary outputs a harness component writes;
	// re-dispatching the agent cannot produce them, so the deliverables gate skips them.
	HarnessProduced []string `json:"harness_produced,omitempty"`
	// DerivedFrom maps an agent-owed secondary to the primary output the host
	// derives it from when the agent leaves it absent. See ADR-0106.
	DerivedFrom map[string]string `json:"derived_from,omitempty"`
}

// ClassifyRules is the declarative verdict spec a phase uses in place of Go classify code.
type ClassifyRules struct {
	RequireSections []string `json:"require_sections,omitempty"`
	FailIfEmpty     bool     `json:"fail_if_empty,omitempty"`
	// FailIfSignal cannot be evaluated until a signal bus exists, so declaring it fails the phase.
	FailIfSignal  map[string]string `json:"fail_if_signal,omitempty"`
	VerdictOnPass string            `json:"verdict_on_pass,omitempty"`
	// RequireFailureContext requires a FAIL/WARN sentinel to carry the structured failure block.
	// See ADR-0039.
	RequireFailureContext bool `json:"require_failure_context,omitempty"`
	// VerdictFromSentinel lets a judgment phase's stated verdict decide: "" off, "shadow" records, "enforce" decides.
	// See ADR-0091.
	VerdictFromSentinel string `json:"verdict_from_sentinel,omitempty"`
}

// Gates names the inter-phase gate functions; the guard layer resolves them, not this package.
type Gates struct {
	In  string `json:"in,omitempty"`
	Out string `json:"out,omitempty"`
}

// PhaseSpec is one phase's declarative definition; every JSON field is optional and accessors supply defaults.
type PhaseSpec struct {
	Name         string `json:"name"`
	Kind         string `json:"kind,omitempty"`      // "llm" (default) | "native" | "command" (reserved)
	Role         string `json:"archetype,omitempty"` // composition archetype; the registry's "role" key names the agent profile instead
	Optional     bool   `json:"optional,omitempty"`
	Agent        string `json:"agent,omitempty"`
	Model        string `json:"model,omitempty"`
	WritesSource bool   `json:"writes_source,omitempty"`
	// Advisor-facing SELECT-card metadata; absence degrades to a name-only card.
	Description string   `json:"description,omitempty"` // one line: what the phase produces
	WhenToUse   string   `json:"when_to_use,omitempty"` // the signal/goal that should trigger SELECTing it
	Categories  []string `json:"categories,omitempty"`  // goal types, validated softly by UnknownCategories
	// The phase's own dispatch guardrails, mirrored by router.PhaseCard so the advisor proposes within them.
	AllowedCLIs       []string                    `json:"allowed_clis,omitempty"`
	ModelTierEnvelope *profiles.ModelTierEnvelope `json:"model_tier_envelope,omitempty"`
	// Catalog decides only advisor SELECT-menu membership, never installation, routing or dispatch.
	Catalog   string `json:"catalog,omitempty"`
	Enabled   string `json:"enabled,omitempty"`
	EnableVar string `json:"enable_var,omitempty"`
	Inputs    IO     `json:"inputs,omitempty"`
	Outputs   IO     `json:"outputs,omitempty"`
	// Effects names lifecycle effects performed outside the workspace (triage: "inbox-claim"), each
	// checked by the deliverables gate. They only add checks, so user phases may declare them.
	Effects       []string             `json:"effects,omitempty"`
	PromptContext []string             `json:"prompt_context,omitempty"`
	Classify      *ClassifyRules       `json:"classify,omitempty"`
	Routing       *config.RoutingBlock `json:"routing,omitempty"`
	Gates         Gates                `json:"gates,omitempty"`
	// After is the routing-order anchor this phase follows; empty places it just before "audit".
	After string `json:"after,omitempty"`
	// Verdict branch targets: OnPass on PASS/WARN, OnFail on FAIL; empty uses the literal table.
	OnPass string `json:"on_pass,omitempty"`
	OnFail string `json:"on_fail,omitempty"`
	// BranchingStrategy picks how the successor is chosen; empty uses the phase-identity default.
	BranchingStrategy string `json:"branching_strategy,omitempty"`
	// Gate, Recovery and EarlyExit config-drive the kernel; nil uses the literal fallback.
	// See ADR-0060.
	Gate     *ArtifactGate `json:"gate,omitempty"`
	Recovery *RecoveryMap  `json:"recovery,omitempty"`
	// EarlyExit is whether this phase may end a no-ship cycle; a ship-intended cycle never exits early, and Go enforces that.
	EarlyExit *bool `json:"early_exit,omitempty"`
}

// RecoveryMap maps a recovery key (verdict, failure-adapter action or debugger action) to a successor phase.
type RecoveryMap struct {
	Targets map[string]string `json:"targets,omitempty"`
}

// ArtifactGate is an anchor phase's operator-settable artifact-floor threshold over the trusted Go digest.
type ArtifactGate struct {
	RequiresPresent bool     `json:"requires_present,omitempty"`
	VerdictIn       []string `json:"verdict_in,omitempty"`
}

// Branching strategy values for PhaseSpec.BranchingStrategy; empty means BranchingVerdict.
const (
	BranchingVerdict = "verdict" // successor chosen by this phase's own verdict
	BranchingHistory = "history" // successor chosen by the failure-adapter from cycle history
	BranchingSignal  = "signal"  // successor chosen by the debugger decision signal
)

// KindOrDefault returns Kind, defaulting to "llm".
func (s PhaseSpec) KindOrDefault() string {
	if s.Kind == "" {
		return "llm"
	}
	return s.Kind
}

// Role is the archetype a phase fulfills; the advisor composes within roles and the floor needs an Evaluate phase.
type Role string

// Role values; Control covers pipeline mechanics that are none of the other three.
const (
	RolePlan     Role = "plan"     // decide what and how
	RoleBuild    Role = "build"    // produce the change
	RoleEvaluate Role = "evaluate" // verify the change
	RoleControl  Role = "control"  // pipeline control
)

// inferredRoles gives built-in phases an archetype without a registry edit.
// An unknown name is Plan, the safest "needs scoping" bucket.
var inferredRoles = map[string]Role{
	"intent": RolePlan, "scout": RolePlan, "triage": RolePlan, "tdd": RolePlan,
	"build-planner": RolePlan, "swarm-plan": RolePlan, "architecture-design": RolePlan, "plan-review": RolePlan,
	"build": RoleBuild,
	"audit": RoleEvaluate, "tester": RoleEvaluate, "evaluator": RoleEvaluate,
	"ship": RoleControl, "retro": RoleControl, "retrospective": RoleControl,
	"memo": RoleControl, "debugger": RoleControl, "start": RoleControl, "end": RoleControl,
}

// RoleOrDefault returns the normalized explicit Role, else infers one from Name, so a typo never yields an unmatchable Role.
func (s PhaseSpec) RoleOrDefault() Role {
	if s.Role != "" {
		switch normalized := Role(strings.ToLower(strings.TrimSpace(s.Role))); normalized {
		case RolePlan, RoleBuild, RoleEvaluate, RoleControl:
			return normalized
		}
	}
	if r, ok := inferredRoles[s.Name]; ok {
		return r
	}
	return RolePlan
}

// AgentName returns Agent, defaulting to the evolve-<name> convention.
func (s PhaseSpec) AgentName() string {
	if s.Agent != "" {
		return s.Agent
	}
	return "evolve-" + s.Name
}

// ModelOrDefault returns Model, defaulting to the "auto" resolution sentinel.
func (s PhaseSpec) ModelOrDefault() string {
	if s.Model == "" {
		return "auto"
	}
	return s.Model
}

// ApplyArchetypeDefaults fills an evaluate-archetype user spec's zero fields; never call it from Load, where audit must stay required.
func ApplyArchetypeDefaults(s *PhaseSpec) {
	if s.RoleOrDefault() != RoleEvaluate {
		return
	}
	if !s.Optional {
		s.Optional = true
	}
	if len(s.PromptContext) == 0 {
		s.PromptContext = []string{"goal"}
	}
	if s.Classify == nil {
		s.Classify = &ClassifyRules{}
	}
	if !s.Classify.FailIfEmpty {
		s.Classify.FailIfEmpty = true
	}
	if s.Classify.VerdictOnPass == "" {
		s.Classify.VerdictOnPass = "PASS"
	}
}

// Catalog membership words for PhaseSpec.Catalog.
const (
	// CatalogSelect is the absent default: the phase is offered to the advisor as a SELECT card.
	CatalogSelect = ""
	// CatalogOnDemand keeps the phase installed and dispatchable but off the advisor's menu.
	CatalogOnDemand = "on-demand"
)

// IsOnDemand reports whether this phase has declined its advisor SELECT slot.
func (s PhaseSpec) IsOnDemand() bool { return s.Catalog == CatalogOnDemand }

// KnownCatalogWord reports whether c is a membership word; an unknown one would silently leave the phase on the menu.
func KnownCatalogWord(c string) bool { return c == CatalogSelect || c == CatalogOnDemand }

// Catalog is the ordered, lookup-able set of phase specs.
type Catalog struct {
	order     []string
	byName    map[string]PhaseSpec
	userNames map[string]bool
}

// Get returns the spec for name, or false on a miss.
func (c Catalog) Get(name string) (PhaseSpec, bool) {
	s, ok := c.byName[name]
	return s, ok
}

// All returns specs in registry (insertion) order.
func (c Catalog) All() []PhaseSpec {
	out := make([]PhaseSpec, 0, len(c.order))
	for _, n := range c.order {
		out = append(out, c.byName[n])
	}
	return out
}

// UserPhases returns the operator-overlay specs in insertion order.
func (c Catalog) UserPhases() []PhaseSpec {
	out := make([]PhaseSpec, 0)
	for _, n := range c.order {
		if c.userNames[n] {
			out = append(out, c.byName[n])
		}
	}
	return out
}

// Names returns a sorted snapshot of spec names.
func (c Catalog) Names() []string {
	out := append([]string(nil), c.order...)
	sort.Strings(out)
	return out
}

type registryDoc struct {
	Phases []PhaseSpec `json:"phases"`
}

// Load parses the registry at path; the registry is a required contract, so any read, parse or validation failure is an error.
func Load(path string) (Catalog, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Catalog{}, fmt.Errorf("read phase registry %q: %w", path, err)
	}
	var doc registryDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return Catalog{}, fmt.Errorf("parse phase registry %q: %w", path, err)
	}
	cat := Catalog{byName: make(map[string]PhaseSpec, len(doc.Phases))}
	for _, s := range doc.Phases {
		if s.Name == "" {
			continue
		}
		if viol := ValidateActivatingFields(s); len(viol) > 0 {
			return Catalog{}, fmt.Errorf("phase registry %q: phase %q: %s", path, s.Name, strings.Join(viol, "; "))
		}
		if viol := ValidateOutputsPartition(s); len(viol) > 0 {
			return Catalog{}, fmt.Errorf("phase registry %q: phase %q: %s", path, s.Name, strings.Join(viol, "; "))
		}
		if _, ok := cat.byName[s.Name]; ok {
			continue
		}
		cat.order = append(cat.order, s.Name)
		cat.byName[s.Name] = s
	}
	return cat, nil
}
