// Package phasespec is the rich, declarative definition of a pipeline phase —
// the "Lego brick" descriptor. A PhaseSpec carries everything the engine needs
// to run a phase as DATA: identity, the typed I/O contract (files + signals it
// consumes and produces), the prompt/classify behavior, and routing triggers.
//
// It is the single full parser of phase-registry.json's phases[] array and of
// per-phase .evolve/phases/<name>/phase.json overlays (Stage 4). config.Load
// reads only the routing subset; phaseorder reads only the order — both are
// consolidated onto this Catalog in later stages.
//
// Layering: phasespec imports config (to reuse RoutingBlock/Condition) and
// stdlib only. It MUST NOT import core — core imports phasespec (PhaseRequest
// carries a PhaseSpec), so a phasespec→core edge would cycle. PhaseSpec is pure
// data: no behavior beyond defaulted accessors.
package phasespec

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// IO is the typed input/output contract of a phase: the artifact files it
// reads/writes and the namespaced signals it consumes/emits. The signals
// declaration is what makes the pipe reorderable — a brick states what it needs
// and what it produces, so the catalog can order it and validate unsatisfiable
// inputs.
type IO struct {
	Files   []string `json:"files,omitempty"`
	Signals []string `json:"signals,omitempty"`
}

// ClassifyRules is the declarative verdict spec — replaces per-phase Go Classify
// for the common case. require_sections + fail_if_empty cover the markdown-shape
// checks the built-in phases hand-code today; fail_if_signal gates on an emitted
// signal threshold (e.g. {"security.severity_max": ">=HIGH"}).
type ClassifyRules struct {
	RequireSections []string          `json:"require_sections,omitempty"`
	FailIfEmpty     bool              `json:"fail_if_empty,omitempty"`
	FailIfSignal    map[string]string `json:"fail_if_signal,omitempty"`
	VerdictOnPass   string            `json:"verdict_on_pass,omitempty"`
}

// Gates names the inter-phase gate functions (declarative; resolved by the
// guard layer). Carried for the registry contract; not evaluated here.
type Gates struct {
	In  string `json:"in,omitempty"`
	Out string `json:"out,omitempty"`
}

// PhaseSpec is the brick definition. All fields are optional in JSON so a user
// phase.json can be minimal; accessor methods supply conventional defaults.
type PhaseSpec struct {
	Name         string `json:"name"`
	Kind         string `json:"kind,omitempty"`      // "llm" (default) | "native" | "command" (reserved)
	Role         string `json:"archetype,omitempty"` // Plan|Build|Evaluate|Control archetype (see Role; inferred from Name when empty). NOTE: distinct from the registry's "role" key, which names the agent/profile (intent/scout/builder/auditor); this is the composition archetype, hence a separate "archetype" JSON key.
	Optional     bool   `json:"optional,omitempty"`
	Agent        string `json:"agent,omitempty"`
	Model        string `json:"model,omitempty"`
	WritesSource bool   `json:"writes_source,omitempty"`
	// Advisor-facing metadata (ADR-0038): rendered into the phase inventory and
	// the advisor's SELECT catalog so routing decisions are informed, not
	// name-guessing. All optional; absence degrades to today's name-only card.
	Description   string               `json:"description,omitempty"` // one line: what the phase produces
	WhenToUse     string               `json:"when_to_use,omitempty"` // the signal/goal that should trigger SELECTing it
	Categories    []string             `json:"categories,omitempty"`  // goal types, validated softly by UnknownCategories
	Enabled       string               `json:"enabled,omitempty"`
	EnableVar     string               `json:"enable_var,omitempty"`
	Inputs        IO                   `json:"inputs,omitempty"`
	Outputs       IO                   `json:"outputs,omitempty"`
	PromptContext []string             `json:"prompt_context,omitempty"`
	Classify      *ClassifyRules       `json:"classify,omitempty"`
	Routing       *config.RoutingBlock `json:"routing,omitempty"`
	Gates         Gates                `json:"gates,omitempty"`
	// After names the phase this one slots in right after, in the routing order
	// (e.g. "build" → runs between build and audit). Empty defaults to running
	// just before "audit" — the canonical post-build check slot.
	After string `json:"after,omitempty"`
	// Verdict branch hints (Stage 3 routing inversion). Reserved; unused in Stage 1.
	OnPass string `json:"on_pass,omitempty"`
	OnFail string `json:"on_fail,omitempty"`
}

// KindOrDefault returns Kind, defaulting to "llm".
func (s PhaseSpec) KindOrDefault() string {
	if s.Kind == "" {
		return "llm"
	}
	return s.Kind
}

// Role is the Plan/Build/Evaluate archetype a phase fulfills — the organizing
// abstraction for advisor composition (compose within roles) and the integrity
// floor (a plan reaching ship must run ≥1 Evaluate phase). Control covers
// pipeline mechanics that are none of the three (ship, retro, memo, debugger).
type Role string

const (
	RolePlan     Role = "plan"     // decide what/how: intent, scout, triage, tdd, build-planner, architecture-design
	RoleBuild    Role = "build"    // produce the change: build
	RoleEvaluate Role = "evaluate" // verify the change: audit, tester
	RoleControl  Role = "control"  // pipeline control, not Plan/Build/Evaluate: ship, retro, memo, debugger
)

// inferredRoles maps the built-in phase names to their archetype. A phase whose
// Role is unset falls back to this table (so existing registry entries need no
// edit); an unknown name defaults to Plan (the safest "needs scoping" bucket).
var inferredRoles = map[string]Role{
	"intent": RolePlan, "scout": RolePlan, "triage": RolePlan, "tdd": RolePlan,
	"build-planner": RolePlan, "swarm-plan": RolePlan, "architecture-design": RolePlan, "plan-review": RolePlan,
	"build": RoleBuild,
	"audit": RoleEvaluate, "tester": RoleEvaluate, "evaluator": RoleEvaluate,
	"ship": RoleControl, "retro": RoleControl, "retrospective": RoleControl,
	"memo": RoleControl, "debugger": RoleControl, "start": RoleControl, "end": RoleControl,
}

// RoleOrDefault returns the explicit Role (normalized + validated), or infers
// one from the phase Name when unset or unrecognized. Used by the floor
// (Evaluate detection) and the advisor catalog, so a mis-cased or typo'd
// "role" in registry/overlay JSON must NOT silently become an unmatchable
// Role — it falls through to name inference instead.
func (s PhaseSpec) RoleOrDefault() Role {
	if s.Role != "" {
		switch normalized := Role(strings.ToLower(strings.TrimSpace(s.Role))); normalized {
		case RolePlan, RoleBuild, RoleEvaluate, RoleControl:
			return normalized
		}
		// unknown explicit value → fall through to name inference
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

// Catalog is the ordered, lookup-able set of phase specs. Registry order is
// preserved (All) for pipeline sequencing; Names is a sorted snapshot.
type Catalog struct {
	order     []string
	byName    map[string]PhaseSpec
	userNames map[string]bool // names contributed by an operator overlay (see Merge)
}

// Get returns the spec for name. (spec, false) on miss.
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

// UserPhases returns specs for phases contributed by an operator overlay
// (as opposed to built-in registry entries). Order matches registry insertion.
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

// registryDoc is the full phases[] view of phase-registry.json.
type registryDoc struct {
	Phases []PhaseSpec `json:"phases"`
}

// Load reads the registry at path and returns its phase Catalog. An unreadable
// or malformed file is a hard error (the registry is a required contract, not a
// fail-open signal source). A spec with an empty name is skipped with no error.
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
		if _, ok := cat.byName[s.Name]; ok {
			continue // first wins; built-ins precede user overlays at merge time
		}
		cat.order = append(cat.order, s.Name)
		cat.byName[s.Name] = s
	}
	return cat, nil
}
