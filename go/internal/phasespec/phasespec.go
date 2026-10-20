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

type IO struct {
	Files           []string          `json:"files,omitempty"`
	Signals         []string          `json:"signals,omitempty"`
	AgentOwed       []string          `json:"agent_owed,omitempty"`
	HarnessProduced []string          `json:"harness_produced,omitempty"`
	DerivedFrom     map[string]string `json:"derived_from,omitempty"`
}

type ClassifyRules struct {
	RequireSections       []string          `json:"require_sections,omitempty"`
	FailIfEmpty           bool              `json:"fail_if_empty,omitempty"`
	FailIfSignal          map[string]string `json:"fail_if_signal,omitempty"`
	VerdictOnPass         string            `json:"verdict_on_pass,omitempty"`
	RequireFailureContext bool              `json:"require_failure_context,omitempty"`
	VerdictFromSentinel   string            `json:"verdict_from_sentinel,omitempty"`
	Grammars              []string          `json:"grammars,omitempty"`
}

type Gates struct {
	In  string `json:"in,omitempty"`
	Out string `json:"out,omitempty"`
}

type PhaseSpec struct {
	Name              string                      `json:"name"`
	Kind              string                      `json:"kind,omitempty"`
	Role              string                      `json:"archetype,omitempty"`
	Optional          bool                        `json:"optional,omitempty"`
	Agent             string                      `json:"agent,omitempty"`
	Model             string                      `json:"model,omitempty"`
	WritesSource      bool                        `json:"writes_source,omitempty"`
	RetractOnFail     bool                        `json:"retract_on_fail,omitempty"`
	Description       string                      `json:"description,omitempty"`
	WhenToUse         string                      `json:"when_to_use,omitempty"`
	Categories        []string                    `json:"categories,omitempty"`
	AllowedCLIs       []string                    `json:"allowed_clis,omitempty"`
	ModelTierEnvelope *profiles.ModelTierEnvelope `json:"model_tier_envelope,omitempty"`
	Catalog           string                      `json:"catalog,omitempty"`
	Enabled           string                      `json:"enabled,omitempty"`
	EnableVar         string                      `json:"enable_var,omitempty"`
	Inputs            IO                          `json:"inputs,omitempty"`
	Outputs           IO                          `json:"outputs,omitempty"`
	Effects           []string                    `json:"effects,omitempty"`
	PromptContext     []string                    `json:"prompt_context,omitempty"`
	Classify          *ClassifyRules              `json:"classify,omitempty"`
	Routing           *config.RoutingBlock        `json:"routing,omitempty"`
	Gates             Gates                       `json:"gates,omitempty"`
	After             string                      `json:"after,omitempty"`
	OnPass            string                      `json:"on_pass,omitempty"`
	OnFail            string                      `json:"on_fail,omitempty"`
	BranchingStrategy string                      `json:"branching_strategy,omitempty"`
	Gate              *ArtifactGate               `json:"gate,omitempty"`
	Recovery          *RecoveryMap                `json:"recovery,omitempty"`
	EarlyExit         *bool                       `json:"early_exit,omitempty"`
}

type RecoveryMap struct {
	Targets map[string]string `json:"targets,omitempty"`
}

type ArtifactGate struct {
	RequiresPresent bool     `json:"requires_present,omitempty"`
	VerdictIn       []string `json:"verdict_in,omitempty"`
}

const (
	BranchingVerdict = "verdict"
	BranchingHistory = "history"
	BranchingSignal  = "signal"
)

func (s PhaseSpec) KindOrDefault() string {
	if s.Kind == "" {
		return "llm"
	}
	return s.Kind
}

type Role string

const (
	RolePlan     Role = "plan"
	RoleBuild    Role = "build"
	RoleEvaluate Role = "evaluate"
	RoleControl  Role = "control"
)

var inferredRoles = map[string]Role{
	"intent": RolePlan, "scout": RolePlan, "triage": RolePlan, "tdd": RolePlan,
	"build-planner": RolePlan, "swarm-plan": RolePlan, "architecture-design": RolePlan, "plan-review": RolePlan,
	"build": RoleBuild,
	"audit": RoleEvaluate, "tester": RoleEvaluate, "evaluator": RoleEvaluate,
	"ship": RoleControl, "retro": RoleControl, "retrospective": RoleControl,
	"memo": RoleControl, "debugger": RoleControl, "start": RoleControl, "end": RoleControl,
}

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

func (s PhaseSpec) AgentName() string {
	if s.Agent != "" {
		return s.Agent
	}
	return "evolve-" + s.Name
}

func (s PhaseSpec) ModelOrDefault() string {
	if s.Model == "" {
		return "auto"
	}
	return s.Model
}

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

const (
	CatalogSelect   = ""
	CatalogOnDemand = "on-demand"
)

func (s PhaseSpec) IsOnDemand() bool { return s.Catalog == CatalogOnDemand }

func KnownCatalogWord(c string) bool { return c == CatalogSelect || c == CatalogOnDemand }

type Catalog struct {
	order     []string
	byName    map[string]PhaseSpec
	userNames map[string]bool
}

func (c Catalog) Get(name string) (PhaseSpec, bool) {
	s, ok := c.byName[name]
	return s, ok
}

func (c Catalog) All() []PhaseSpec {
	out := make([]PhaseSpec, 0, len(c.order))
	for _, n := range c.order {
		out = append(out, c.byName[n])
	}
	return out
}

func (c Catalog) UserPhases() []PhaseSpec {
	out := make([]PhaseSpec, 0)
	for _, n := range c.order {
		if c.userNames[n] {
			out = append(out, c.byName[n])
		}
	}
	return out
}

func (c Catalog) Names() []string {
	out := append([]string(nil), c.order...)
	sort.Strings(out)
	return out
}

type registryDoc struct {
	Phases []PhaseSpec `json:"phases"`
}

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
